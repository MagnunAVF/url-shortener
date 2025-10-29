package main

// This service is a simplified "Snowflake" ID generator.
// It creates unique 64-bit IDs that are roughly time-sortable.
// https://en.wikipedia.org/wiki/Snowflake_ID
// This service solves the "auto-increment" bottleneck in DB.

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

const (
	customEpoch int64 = 1704067200000 // Jan 1, 2024
	nodeIDBits  uint  = 10
	seqBits     uint  = 12
	maxNodeID   int64 = -1 ^ (-1 << nodeIDBits)
	maxSeq      int64 = -1 ^ (-1 << seqBits)
)

type IDGenerator struct {
	mu        sync.Mutex
	lastStamp int64
	nodeID    int64
	seq       int64
}

func NewIDGenerator(nodeID int64) (*IDGenerator, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fiber.ErrBadRequest
	}

	return &IDGenerator{nodeID: nodeID}, nil
}

func (g *IDGenerator) NextID() (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ts := time.Now().UnixMilli()
	if ts < g.lastStamp {
		// Clock went backwards, wait
		ts = g.wait(ts)
	}
	if ts == g.lastStamp {
		g.seq = (g.seq + 1) & maxSeq
		if g.seq == 0 {
			ts = g.wait(ts)
		}
	} else {
		g.seq = 0
	}
	g.lastStamp = ts
	id := (uint64(ts-customEpoch) << (nodeIDBits + seqBits)) |
		(uint64(g.nodeID) << seqBits) |
		uint64(g.seq)

	return id, nil
}

func (g *IDGenerator) wait(currentTS int64) int64 {
	for currentTS <= g.lastStamp {
		time.Sleep(1 * time.Millisecond)
		currentTS = time.Now().UnixMilli()
	}

	return currentTS
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: .env file not found, relying on env vars: %v", err)
	}

	// hardcoded Node ID = 1 at this time
	gen, err := NewIDGenerator(1)
	if err != nil {
		log.Fatalf("Failed to create ID generator: %v", err)
	}

	app := fiber.New()
	app.Get("/new-id", func(c *fiber.Ctx) error {
		id, err := gen.NextID()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate ID"})
		}
		return c.JSON(fiber.Map{"id": id})
	})

	log.Printf("Starting ID Service on %s", os.Getenv("ID_SERVICE_PORT"))
	log.Fatal(app.Listen(os.Getenv("ID_SERVICE_PORT")))
}
