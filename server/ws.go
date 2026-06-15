// ws.go - the HTTP/WebSocket surface: the /ws upgrade handler (with an
// env-driven origin allow-list), the /healthz probe, and the upgrader
// configuration. On connect the client is registered, handed its initial
// snapshot, and then the read/write pumps take over.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

// defaultDevOrigins are the localhost dev origins permitted when ALLOWED_ORIGINS
// is unset. In production, ALLOWED_ORIGINS must include the client domain.
var defaultDevOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

// parseAllowedOrigins turns a comma-separated allow-list into a set. Entries are
// trimmed and empties are ignored. An empty/blank value yields the dev defaults,
// so local development works with ALLOWED_ORIGINS unset.
func parseAllowedOrigins(raw string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		set[origin] = struct{}{}
	}
	if len(set) == 0 {
		for _, origin := range defaultDevOrigins {
			set[origin] = struct{}{}
		}
	}
	return set
}

// newOriginChecker returns a CheckOrigin func bound to the given allow-list. It
// permits non-browser clients (no Origin header) and any origin whose normalized
// scheme://host[:port] is in the set; it rejects everything else.
func newOriginChecker(allowed map[string]struct{}) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // curl / native ws clients send no Origin
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		normalized := u.Scheme + "://" + u.Host
		_, ok := allowed[normalized]
		return ok
	}
}

// server bundles the dependencies the HTTP handlers need.
type server struct {
	hub      *Hub
	sim      *Simulator
	upgrader websocket.Upgrader
}

// newServer wires the upgrader with our origin policy and modest buffers. The
// origin allow-list is read from ALLOWED_ORIGINS (comma-separated); when unset
// it falls back to localhost dev origins.
func newServer(hub *Hub, sim *Simulator) *server {
	allowed := parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"))
	return &server{
		hub: hub,
		sim: sim,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     newOriginChecker(allowed),
		},
	}
}

// handleWS upgrades the connection, registers the client, sends the initial
// snapshot, and starts the pumps. Registration happens before the snapshot is
// taken so the client cannot miss an update that lands in between - keeping the
// client-visible sequence contiguous.
func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}

	client := &Client{hub: s.hub, conn: conn, send: make(chan []byte, sendBuffer)}
	s.hub.register(client)

	snapshot := s.sim.snapshot()
	if payload, err := json.Marshal(snapshot); err == nil {
		client.send <- payload
	}

	go client.writePump()
	go client.readPump()
}

// handleHealthz is a liveness probe; it also reports the live client count.
func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok clients=" + itoa(s.hub.count())))
}

// routes registers the HTTP handlers on a fresh mux.
func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/healthz", s.handleHealthz)
	return mux
}

// normalizePort accepts "8080" or ":8080" and returns a valid listen address.
func normalizePort(p string) string {
	if p == "" {
		p = "8080"
	}
	if strings.HasPrefix(p, ":") {
		return p
	}
	return ":" + p
}
