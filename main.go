package main

import (
	"log/slog"
	"net/http"
	"os"

	"ticket-api/handlers"
	"ticket-api/store"
)

func main() {
	// store.New() creates the in-memory store and seeds it with three events.
	// Every function on *Store is safe to call from multiple goroutines
	// simultaneously because the store uses a sync.RWMutex internally.
	s := store.New()

	// NewTicketHandler wraps the store. By passing the store as a dependency
	// (Dependency Injection), we keep handlers testable — you can swap in a
	// fake store during tests without changing any handler code.
	th := handlers.NewTicketHandler(s)

	// http.NewServeMux is the standard Go HTTP router.
	// Since Go 1.22 it understands method prefixes ("GET ") and path
	// parameters ("{id}") natively — no third-party router needed.
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/tickets", th.List)
	mux.HandleFunc("POST /api/v1/tickets", th.Create)
	mux.HandleFunc("GET /api/v1/tickets/{id}", th.Get)
	mux.HandleFunc("PUT /api/v1/tickets/{id}", th.Update)
	mux.HandleFunc("DELETE /api/v1/tickets/{id}", th.Delete)
	mux.HandleFunc("POST /api/v1/tickets/{id}/reserve", th.Reserve)

	// handlers.Chain wraps the mux with three middleware layers.
	// The leftmost middleware listed runs first (outermost), so the execution
	// order for every request is:
	//   Recovery → Logger → CORS → your handler → CORS → Logger → Recovery
	// Recovery goes first so it catches panics even inside Logger.
	handler := handlers.Chain(mux,
		handlers.Recovery,
		handlers.Logger,
		handlers.CORS,
	)

	addr := ":8080"
	slog.Info("ticket-api listening", "addr", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
