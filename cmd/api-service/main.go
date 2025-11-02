package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/MagnunAVF/url-shortener/internal"
	applog "github.com/MagnunAVF/url-shortener/internal/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	AppDomain    string
	ClickQueue   string
	IDServiceURL string
	Redis        *redis.Client
	DB           *gorm.DB
	RabbitMQ     *amqp091.Channel
}

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

	ctx := context.Background()
	cfg := loadConfig(ctx)

	slog.Info("Running GORM Auto-Migration...")
	err := cfg.DB.AutoMigrate(&internal.URL{}, &internal.URLAnalytics{})
	if err != nil {
		slog.Error("Failed to auto-migrate database", "err", err)
		os.Exit(1)
	}
	slog.Info("Migration complete.")

	app := fiber.New()
	app.Use(logger.New())
	app.Use(cors.New())

	app.Get("/:short_code", handleRedirect(cfg))
	app.Post("/shorten", handleShorten(cfg))
	app.Get("/stats/:short_code", handleGetStats(cfg))

	slog.Info("Starting API Service", "port", os.Getenv("API_SERVICE_PORT"))
	if err := app.Listen(os.Getenv("API_SERVICE_PORT")); err != nil {
		slog.Error("API Service failed", "err", err)
		os.Exit(1)
	}
}

func handleRedirect(cfg *Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		shortCode := c.Params("short_code")
		ctx := c.Context()

		cacheKey := "url:" + shortCode
		longURL, err := cfg.Redis.Get(ctx, cacheKey).Result()

		if err == redis.Nil {
			var url internal.URL
			err = cfg.DB.Select("long_url").Where("short_code = ?", shortCode).First(&url).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Short URL not found"})
			} else if err != nil {
				slog.Error("DB error", "err", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
			}

			longURL = url.LongURL

			if err := cfg.Redis.Set(ctx, cacheKey, longURL, 1*time.Hour).Err(); err != nil {
				slog.Error("Error setting cache", "err", err)
			}
		} else if err != nil {
			slog.Error("Error reading cache", "err", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Cache error"})
		}

		userAgent := c.Get("User-Agent")
		if userAgent == "" {
			userAgent = "Unknown"
		}
		go publishClickEvent(cfg, shortCode, userAgent)

		return c.Redirect(longURL, fiber.StatusFound)
	}
}

func handleShorten(cfg *Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			URL string `json:"url"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}
		if req.URL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL cannot be empty"})
		}

		var existingURL internal.URL
		err := cfg.DB.Select("short_code").Where("long_url = ?", req.URL).First(&existingURL).Error
		if err == nil {
			return c.JSON(fiber.Map{
				"short_url": fmt.Sprintf("%s/%s", cfg.AppDomain, existingURL.ShortCode),
			})
		}

		id, err := getNewID(cfg.IDServiceURL)
		if err != nil {
			slog.Error("Error getting new ID", "err", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not generate ID"})
		}

		shortCode := internal.EncodeID(id)

		newURL := internal.URL{
			ID:        int64(id), // TODO: improve this id type. at this time, tmp cast this value
			ShortCode: shortCode,
			LongURL:   req.URL,
		}

		err = cfg.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&newURL).Error; err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			slog.Error("Error creating short URL", "err", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save URL"})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"short_url": fmt.Sprintf("%s/%s", cfg.AppDomain, shortCode),
		})
	}
}

func handleGetStats(cfg *Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendString("Returning from handleGetStats")
	}
}

func loadConfig(ctx context.Context) *Config {
	DB, err := gorm.Open(postgres.Open(os.Getenv("DB_URL")), &gorm.Config{})
	if err != nil {
		slog.Error("Unable to connect to database", "err", err)
		os.Exit(1)
	}

	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		slog.Error("Unable to connect to Redis", "err", err)
		os.Exit(1)
	}

	rabbitConn, err := amqp091.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		slog.Error("Unable to connect to RabbitMQ", "err", err)
		os.Exit(1)
	}
	rabbitCH, err := rabbitConn.Channel()
	if err != nil {
		slog.Error("Unable to open RabbitMQ channel", "err", err)
		os.Exit(1)
	}

	queueName := os.Getenv("CLICK_QUEUE_NAME")
	_, err = rabbitCH.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		slog.Error("Failed to declare RabbitMQ queue", "queue", queueName, "err", err)
		os.Exit(1)
	}

	IDServiceURL := "http://" + os.Getenv("ID_SERVICE_DOMAIN") + os.Getenv("ID_SERVICE_PORT") + "/new-id"

	return &Config{
		AppDomain:    os.Getenv("APP_DOMAIN"),
		ClickQueue:   queueName,
		IDServiceURL: IDServiceURL,
		Redis:        rdb,
		DB:           DB,
		RabbitMQ:     rabbitCH,
	}
}

func getNewID(serviceURL string) (uint64, error) {
	resp, err := http.Get(serviceURL)
	if err != nil {
		return 0, fmt.Errorf("failed to call ID service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ID service returned non-200 status: %s", resp.Status)
	}
	var data struct {
		ID uint64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("failed to decode ID service response: %w", err)
	}
	return data.ID, nil
}

func publishClickEvent(cfg *Config, shortCode, userAgent string) {
	event := ClickEvent{
		ShortCode: shortCode,
		Timestamp: time.Now(),
		UserAgent: userAgent,
	}
	slog.Info("Publishing click event", "event", event)

	body, err := json.Marshal(event)
	if err != nil {
		slog.Error("Error marshalling click event", "err", err)
		return
	}
	err = cfg.RabbitMQ.PublishWithContext(
		context.Background(),
		"", cfg.ClickQueue, false, false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		slog.Error("Error publishing click event", "err", err)
	}
}
