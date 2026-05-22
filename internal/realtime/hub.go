// Package realtime is a tiny in-process pub/sub for Server-Sent Events.
//
// One Hub per process. Handlers/workers Publish(topic, kind, payload); the SSE
// endpoint Subscribes a connection to a set of topics and streams matching
// events. Delivery is best-effort: if a subscriber's buffer is full (a slow or
// stuck client) the event is dropped for that subscriber rather than blocking
// the publisher.
package realtime

import "sync"

// Event is one server-sent message.
type Event struct {
	Topic   string `json:"topic"`
	Kind    string `json:"kind"`
	Payload any    `json:"payload,omitempty"`
}

// Subscription is a live connection's view onto the hub.
type Subscription struct {
	C      chan Event
	topics map[string]bool
}

// Hub fans out events to subscribers by topic.
type Hub struct {
	mu   sync.RWMutex
	subs map[*Subscription]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[*Subscription]struct{})}
}

// Subscribe registers interest in the given topics and returns a Subscription
// whose C channel yields matching events. Call Unsubscribe when done.
func (h *Hub) Subscribe(topics []string) *Subscription {
	set := make(map[string]bool, len(topics))
	for _, t := range topics {
		set[t] = true
	}
	sub := &Subscription{C: make(chan Event, 32), topics: set}
	h.mu.Lock()
	h.subs[sub] = struct{}{}
	h.mu.Unlock()
	return sub
}

func (h *Hub) Unsubscribe(sub *Subscription) {
	h.mu.Lock()
	if _, ok := h.subs[sub]; ok {
		delete(h.subs, sub)
		close(sub.C)
	}
	h.mu.Unlock()
}

// Publish delivers an event to every subscriber listening on topic.
func (h *Hub) Publish(topic, kind string, payload any) {
	ev := Event{Topic: topic, Kind: kind, Payload: payload}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subs {
		if !sub.topics[topic] {
			continue
		}
		select {
		case sub.C <- ev:
		default:
			// Buffer full — drop rather than block the publisher.
		}
	}
}

// Subscribers reports the current subscriber count (for diagnostics).
func (h *Hub) Subscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
