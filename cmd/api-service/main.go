package main

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	AppDomain string
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: .env file not found, relying on env vars: %v", err)
	}

	ctx := context.Background()
	config := loadConfig(ctx)

	app := fiber.New()
	app.Use(logger.New())
	app.Use(cors.New())

	app.Get("/:short_code", handleRedirect(config))
	app.Post("/shorten", handleShorten(config))
	app.Get("/stats/:short_code", handleGetStats(config))

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
		return c.SendString("Returning from handleShorten")
	}
}

func handleGetStats(cfg *Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendString("Returning from handleGetStats")
	}
}

func loadConfig(ctx context.Context) *Config {
	return &Config{
		AppDomain: os.Getenv("APP_DOMAIN"),
	}
}
