// main.go - process entry point. Builds the hub + simulator, starts the
// simulation goroutine, wires the HTTP routes, and serves with graceful
// shutdown on SIGINT/SIGTERM. Port is configurable via PORT (default 8080).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	hub := newHub()
	sim := newSimulator()

	done := make(chan struct{})
	go sim.run(hub, done)

	srv := newServer(hub, sim)
	addr := normalizePort(os.Getenv("PORT"))

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run the HTTP server in its own goroutine so main can wait for a signal.
	go func() {
		log.Printf("live-fleet-grid server listening on %s (ws: /ws, health: /healthz)", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Block until an interrupt arrives.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	// Stop the simulation, then drain in-flight HTTP with a short grace window.
	close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("stopped")
}
