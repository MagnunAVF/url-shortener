package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/MagnunAVF/url-shortener/internal"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	AppDomain string
	DB        *gorm.DB
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: .env file not found, relying on env vars: %v", err)
	}

	ctx := context.Background()
	cfg := loadConfig(ctx)

	log.Println("Running GORM Auto-Migration...")
	err := cfg.DB.AutoMigrate(&internal.URL{})
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	log.Println("Migration complete.")

	app := fiber.New()
	app.Use(logger.New())
	app.Use(cors.New())

	app.Get("/:short_code", handleRedirect(cfg))
	app.Post("/shorten", handleShorten(cfg))
	app.Get("/stats/:short_code", handleGetStats(cfg))

	log.Printf("Starting API Service on %s", os.Getenv("API_SERVICE_PORT"))
	log.Fatal(app.Listen(os.Getenv("API_SERVICE_PORT")))
}

func handleRedirect(cfg *Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendString("Returning from handleRedirect")
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

		id := getNewID()
		shortCode := internal.EncodeID(uint64(id))

		newURL := internal.URL{
			ID:        id,
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
			log.Printf("Error creating short URL: %v", err)
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
		log.Fatalf("Unable to connect to database: %v", err)
	}

	return &Config{
		AppDomain: os.Getenv("APP_DOMAIN"),
		DB:        DB,
	}
}

// Generate a positive 63-bit integer
func getNewID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		// Mask to 63 bits to avoid negative values and overflow
		v := int64(binary.BigEndian.Uint64(b[:]) & ((1 << 63) - 1))
		if v != 0 {
			return v
		}
	}
	// Fallback to time-based value; ensure non-zero and 63-bit positive
	ns := time.Now().UnixNano()
	if ns < 0 {
		ns = -ns
	}

	ns &= (1 << 63) - 1
	if ns == 0 {
		ns = 1
	}

	return ns
}
