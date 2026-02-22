package events

import (
	"context"
	"log"
	"sync"
)

type handlerFunc func(ctx context.Context, payload interface{})

type bus struct {
	mu       sync.RWMutex
	handlers map[string][]handlerFunc
}

var defaultBus = &bus{handlers: make(map[string][]handlerFunc)}

// Publish publishes an event asynchronously to all subscribers.
func Publish(event string, payload interface{}) {
	log.Printf("events.Publish: event=%s payload_type=%T", event, payload)
	defaultBus.mu.RLock()
	hs := append([]handlerFunc{}, defaultBus.handlers[event]...)
	defaultBus.mu.RUnlock()

	for _, h := range hs {
		ev := event
		go func(h handlerFunc, ev string) {
			log.Printf("events: invoking handler for event=%s", ev)
			h(context.Background(), payload)
		}(h, ev)
	}
}

// Subscribe registers a handler for an event. It returns an unsubscribe function.
func Subscribe(event string, h handlerFunc) func() {
	defaultBus.mu.Lock()
	defaultBus.handlers[event] = append(defaultBus.handlers[event], h)
	idx := len(defaultBus.handlers[event]) - 1
	defaultBus.mu.Unlock()

	return func() {
		defaultBus.mu.Lock()
		defer defaultBus.mu.Unlock()
		hs := defaultBus.handlers[event]
		if idx < 0 || idx >= len(hs) {
			return
		}
		defaultBus.handlers[event] = append(hs[:idx], hs[idx+1:]...)
	}
}
