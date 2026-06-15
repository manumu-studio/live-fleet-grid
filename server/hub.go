// hub.go - the connection registry. Tracks every live client and fans out
// broadcasts to all of them. A single sync.Mutex guards the client set, so
// register/unregister/broadcast are safe to call from the HTTP handler
// goroutines and the simulation goroutine concurrently.
package main

import "sync"

// Hub is the single-node broadcast registry. clients is a set; the empty
// struct value carries no data, it just marks membership.
type Hub struct {
	mu      sync.Mutex
	clients map[*Client]struct{}
}

// newHub returns an empty, ready-to-use hub.
func newHub() *Hub {
	return &Hub{clients: make(map[*Client]struct{})}
}

// register adds a client to the set.
func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

// unregister removes a client and closes its send channel exactly once.
func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// broadcast queues a message to every client. If a client's send buffer is
// full (a slow/stalled reader) we drop the frame for that client rather than
// block the whole fan-out - back-pressure is handled per-connection.
func (h *Hub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Slow consumer: drop this frame for this client only.
		}
	}
}

// count returns the number of connected clients (used by /healthz-style checks).
func (h *Hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
