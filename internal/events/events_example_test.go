package events_test

import (
	"context"
	"testing"
	"time"

	"go_framework/internal/events"
)

// TestPublishSubscribe demonstrates basic usage of the events package:
// subscribe to an event, publish it, and assert the handler is invoked.
func TestPublishSubscribe(t *testing.T) {
	ch := make(chan interface{}, 1)

	unsub := events.Subscribe("user.created", func(ctx context.Context, payload interface{}) {
		ch <- payload
	})
	defer unsub()

	// publish asynchronously
	events.Publish("user.created", "hello-world")

	select {
	case v := <-ch:
		s, ok := v.(string)
		if !ok || s != "hello-world" {
			t.Fatalf("unexpected payload: %#v", v)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event handler")
	}
}
