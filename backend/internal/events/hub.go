// Package events provides a fan-out hub for Server-Sent Events.
package events

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/pulsewatch/pulsewatch/backend/internal/models"
)

const subscriberBuffer = 16

// Hub broadcasts SSE events to connected clients without blocking publishers.
type Hub struct {
	mu   sync.RWMutex
	subs map[chan []byte]struct{}
	log  *slog.Logger
}

// NewHub creates an empty event hub.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		subs: make(map[chan []byte]struct{}),
		log:  logger,
	}
}

// Subscribe registers a new client channel. Caller must Unsubscribe when done.
func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, subscriberBuffer)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a client and closes its channel.
func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// Publish marshals an SSE event and non-blockingly fans it out.
// Slow subscribers are skipped so monitoring workers never stall.
func (h *Hub) Publish(event models.SSEEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		h.log.Error("failed to marshal sse event", "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- payload:
		default:
			// Drop — slow consumer must not block the monitoring path.
			h.log.Debug("dropping sse event for slow subscriber")
		}
	}
}

// SubscriberCount returns the number of connected SSE clients (for diagnostics).
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
