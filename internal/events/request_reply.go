package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// RequestReply publishes a request event and waits for a single reply.
// It creates a unique reply topic and includes it in the published payload under key "ReplyTo".
// Subscribers should publish the response to that reply topic. The call blocks until a
// reply arrives, the context is cancelled, or the timeout elapses.
func RequestReply(ctx context.Context, requestEvent string, payload interface{}, timeout time.Duration) (interface{}, error) {
	// generate unique reply topic
	id, err := genID(12)
	if err != nil {
		return nil, err
	}
	replyTopic := fmt.Sprintf("_reply.%s", id)

	// subscribe to reply topic
	ch := make(chan interface{}, 1)
	unsub := Subscribe(replyTopic, func(_ctx context.Context, p interface{}) {
		select {
		case ch <- p:
		default:
		}
	})
	defer unsub()

	// build request payload: if payload is map[string]interface{} we inject ReplyTo, otherwise wrap
	var req interface{}
	if m, ok := payload.(map[string]interface{}); ok {
		// copy to avoid mutating caller data
		copyMap := make(map[string]interface{}, len(m)+1)
		for k, v := range m {
			copyMap[k] = v
		}
		copyMap["ReplyTo"] = replyTopic
		req = copyMap
	} else {
		req = map[string]interface{}{"ReplyTo": replyTopic, "Body": payload}
	}

	// publish request
	Publish(requestEvent, req)

	// wait for reply, context cancel, or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("request reply timeout")
	case r := <-ch:
		return r, nil
	}
}

func genID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
