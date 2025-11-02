package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/MagnunAVF/url-shortener/internal"
)

type ClickEvent struct {
	ShortCode string    `json:"short_code"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: .env file not found, relying on env vars: %v", err)
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.SetPrefix("analytics-worker ")

	writeDB, err := gorm.Open(postgres.Open(os.Getenv("DB_URL")), &gorm.Config{})
	if err != nil {
		log.Fatalf("Unable to connect to primary database: %v", err)
	}

	rabbitConn, err := amqp091.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		log.Fatalf("Unable to connect to RabbitMQ: %v", err)
	}
	defer rabbitConn.Close()

	rabbitCH, err := rabbitConn.Channel()
	if err != nil {
		log.Fatalf("Unable to open RabbitMQ channel: %v", err)
	}
	defer rabbitCH.Close()

	q, err := rabbitCH.QueueDeclare(
		os.Getenv("CLICK_QUEUE_NAME"),
		true, false, false, false, nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	// Set prefetch to 100. This worker will grab 100 messages at a time.
	if err := rabbitCH.Qos(100, 0, false); err != nil {
		log.Fatalf("Failed to set QoS: %v", err)
	}

	msgs, err := rabbitCH.Consume(
		q.Name, "", false, false, false, false, nil,
	)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	log.Println("Analytics Worker started. Waiting for click events...")

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
					log.Println("RabbitMQ channel closed.")
					return
				}
				var event ClickEvent
				if err := json.Unmarshal(d.Body, &event); err != nil {
					log.Printf("Error decoding message: %v. Rejecting.", err)
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
					log.Printf("Timer flush: processing %d queued events", len(events))
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
	log.Printf("Processing batch of %d events", len(events))

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
				log.Printf("Error upserting click count for short code %s: %v", shortCode, err)
				return err
			}
		}
		log.Printf("Successfully processed batch of %d events", len(events))
		return nil
	})

	// Nack in transaction error
	if err != nil {
		log.Printf("Failed to process batch transaction: %v. Nacking messages.", err)
		// Re-queue messages for another try
		nackAll(deliveries)
		return
	}

	// ack in transaction success
	ackAll(deliveries)
	log.Printf("Successfully processed and acked %d messages.", len(deliveries))
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
