package keydb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Flash represents a one-time message to display to the user.
type Flash struct {
	Type    string `json:"type"` // "success", "error", "info"
	Message string `json:"message"`
}

// SetFlash stores a flash message in KeyDB with a TTL.
// Key pattern: flash:sess:<sessionID>
func SetFlash(ctx context.Context, sessionID string, f Flash, ttlSeconds int) error {
	if Client == nil {
		return errors.New("KeyDB client not initialized")
	}

	data, err := json.Marshal(f)
	if err != nil {
		return err
	}

	key := "flash:sess:" + sessionID
	return Client.Set(ctx, key, data, time.Duration(ttlSeconds)*time.Second).Err()
}

// GetAndClearFlash retrieves the flash message and deletes it atomically from KeyDB.
// Returns nil if no flash exists.
// Uses GETDEL if available (Redis 6.2+), otherwise falls back to Lua script.
func GetAndClearFlash(ctx context.Context, sessionID string) (*Flash, error) {
	if Client == nil {
		return nil, errors.New("KeyDB client not initialized")
	}

	key := "flash:sess:" + sessionID

	// Try GETDEL (atomic get + delete) - available in Redis 6.2+
	val, err := Client.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key doesn't exist, no flash
			return nil, nil
		}
		// Other error (connection, etc.)
		return nil, err
	}

	if len(val) == 0 {
		return nil, nil
	}

	var f Flash
	if err := json.Unmarshal(val, &f); err != nil {
		return nil, err
	}

	return &f, nil
}
