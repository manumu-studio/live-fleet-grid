// client.go - per-connection read and write pumps. Each WebSocket connection
// gets one Client with a buffered send channel. The write pump owns all writes
// to the socket (gorilla allows only one concurrent writer) and emits heartbeat
// pings; the read pump drains/discards inbound frames and enforces a read
// deadline via the pong handler so dead peers are reaped.
package main

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is how long a single write may block before we give up.
	writeWait = 10 * time.Second
	// pongWait is the read deadline; if no pong/data arrives in this window
	// the connection is considered dead.
	pongWait = 60 * time.Second
	// pingPeriod must be shorter than pongWait so a pong can refresh the
	// deadline before it expires.
	pingPeriod = (pongWait * 9) / 10
	// sendBuffer bounds per-client back-pressure. A reader that falls this
	// far behind starts dropping frames (see Hub.broadcast).
	sendBuffer = 32
)

// Client wraps a single WebSocket connection and its outbound queue.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// readPump enforces the read deadline and refreshes it on every pong. It
// discards any inbound payloads (this is a one-way broadcast feed) and exits
// on any read error, triggering cleanup.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump is the sole writer for the connection. It drains the send channel
// and, on a ticker, emits ping frames for liveness. A closed send channel (set
// by the hub during unregister) cleanly closes the socket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
