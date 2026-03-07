package keydb

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

// Init initializes the KeyDB client with the provided configuration.
// Call this once during app startup (e.g., in bootstrap).
func Init(host string, port string, password string, db int) error {
	addr := host + ":" + port
	Client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		log.Printf("[KeyDB] Ping failed: %v", err)
		return err
	}

	log.Printf("[KeyDB] Connected to %s", addr)
	return nil
}
