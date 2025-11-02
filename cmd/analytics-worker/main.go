package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/MagnunAVF/url-shortener/internal"
	applog "github.com/MagnunAVF/url-shortener/internal/logger"
)

type ClickEvent struct {
	ShortCode string    `json:"short_code"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Warn(".env file not found, relying on env vars", "err", err)
	}

	applog.InitFromEnv()

	writeDB, err := gorm.Open(postgres.Open(os.Getenv("DB_URL")), &gorm.Config{Logger: applog.NewGormLogger(os.Getenv("GORM_LOG_LEVEL"))})
	if err != nil {
		slog.Error("Unable to connect to primary database", "err", err)
		os.Exit(1)
	}

	rabbitConn, err := amqp091.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		slog.Error("Unable to connect to RabbitMQ", "err", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	rabbitCH, err := rabbitConn.Channel()
	if err != nil {
		slog.Error("Unable to open RabbitMQ channel", "err", err)
		os.Exit(1)
	}
	defer rabbitCH.Close()

	q, err := rabbitCH.QueueDeclare(
		os.Getenv("CLICK_QUEUE_NAME"),
		true, false, false, false, nil,
	)
	if err != nil {
		slog.Error("Failed to declare queue", "err", err)
		os.Exit(1)
	}

	// Set prefetch to 100. This worker will grab 100 messages at a time.
	if err := rabbitCH.Qos(100, 0, false); err != nil {
		slog.Error("Failed to set QoS", "err", err)
		os.Exit(1)
	}

	msgs, err := rabbitCH.Consume(
		q.Name, "", false, false, false, false, nil,
	)
	if err != nil {
		slog.Error("Failed to register consumer", "err", err)
		os.Exit(1)
	}

	slog.Info("Analytics Worker started. Waiting for click events...")

	var forever chan struct{}
	var events []ClickEvent
	var deliveries []amqp091.Delivery

	// Batch process every 2 seconds
	ticker := time.NewTicker(2 * time.Second)

	go func() {
		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					slog.Warn("RabbitMQ channel closed")
					return
				}
				var event ClickEvent
				if err := json.Unmarshal(d.Body, &event); err != nil {
					slog.Error("Error decoding message. Rejecting.", "err", err)
					// 'false' means don't re-queue
					d.Reject(false)
					continue
				}
				events = append(events, event)
				deliveries = append(deliveries, d)

				// Process if batch is full
				if len(events) >= 100 {
					processBatch(writeDB, events, deliveries)
					events, deliveries = nil, nil
					ticker.Reset(2 * time.Second)
				}

			// Process on a timer
			case <-ticker.C:
				if len(events) > 0 {
					slog.Info("Timer flush: processing queued events", "count", len(events))
					processBatch(writeDB, events, deliveries)
					events, deliveries = nil, nil
				}
			}
		}
	}()

	// Block forever
	<-forever
}

func processBatch(db *gorm.DB, events []ClickEvent, deliveries []amqp091.Delivery) {
	if len(events) == 0 {
		return
	}
	slog.Info("Processing batch of events", "count", len(events))

	counts := make(map[string]int64)
	for _, event := range events {
		counts[event.ShortCode]++
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		for shortCode, count := range counts {
			// Upsert: insert initial count, or increment existing count atomically
			rec := internal.URLAnalytics{ShortCode: shortCode, ClickCount: count}
			if err := tx.Clauses(
				clause.OnConflict{
					Columns: []clause.Column{{Name: "short_code"}},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"click_count": gorm.Expr("url_analytics.click_count + EXCLUDED.click_count"),
					}),
				},
			).Create(&rec).Error; err != nil {
				slog.Error("Error upserting click count", "short_code", shortCode, "err", err)
				return err
			}
		}
		slog.Info("Successfully processed batch", "count", len(events))
		return nil
	})

	// Nack in transaction error
	if err != nil {
		slog.Error("Failed to process batch transaction. Nacking messages.", "err", err)
		// Re-queue messages for another try
		nackAll(deliveries)
		return
	}

	// ack in transaction success
	ackAll(deliveries)
	slog.Info("Successfully processed and acked messages", "count", len(deliveries))
}

func ackAll(deliveries []amqp091.Delivery) {
	for _, d := range deliveries {
		d.Ack(false)
	}
}

func nackAll(deliveries []amqp091.Delivery) {
	for _, d := range deliveries {
		d.Nack(false, true)
	}
}
