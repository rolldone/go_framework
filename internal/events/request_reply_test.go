package events_test

import (
	"context"
	"testing"
	"time"

	"go_framework/internal/events"
)

// TestRequestReply demonstrates a safe request-reply pattern using RequestReply.
func TestRequestReply(t *testing.T) {
	// subscriber: listens for "user.query" and replies to ReplyTo topic
	unsub := events.Subscribe("user.query", func(ctx context.Context, payload interface{}) {
		m, ok := payload.(map[string]interface{})
		if !ok {
			return
		}
		replyTo, _ := m["ReplyTo"].(string)
		q := m["Body"]
		// simulate processing
		time.Sleep(50 * time.Millisecond)
		_ = replyTo
		// echo back
		events.Publish(replyTo, map[string]interface{}{"ok": true, "query": q})
	})
	defer unsub()

	ctx := context.Background()

	// make two concurrent requests
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			resp, err := events.RequestReply(ctx, "user.query", map[string]interface{}{"id": i}, 500*time.Millisecond)
			if err != nil {
				t.Errorf("request %d failed: %v", i, err)
			}
			if resp == nil {
				t.Errorf("request %d: nil response", i)
			}
			done <- struct{}{}
		}(i)
	}

	// wait for both
	select {
	case <-done:
		// at least one returned; wait for second
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for first response")
	}
	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for second response")
	}
}
