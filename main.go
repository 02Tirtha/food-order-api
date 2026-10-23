package main

// Package main defines the entry point of our food order management backend.
// It initializes the storage layer, business service, HTTP handlers, routes, and starts the server.

import (
	// log provides console logging for startup messages and critical server errors.
	"log"
	// net/http provides HTTP server configurations, routing, and network connection handling.
	"net/http"
	// time provides duration primitives used to set resilient server timeouts.
	"time"
)

func main() {
	// 1. Initialize dependencies in bottom-up order:
	// Store (Data) -> Service (Business Rules) -> Handler (HTTP Transport).
	// This pattern is known as Dependency Injection; it makes our code modular, testable, and loosely coupled.
	store := NewStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 2. Setup HTTP request multiplexer and register API routes.
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 3. Wrap the multiplexer with our logging middleware to log every incoming request's
	// method, path, HTTP status, and duration.
	loggedMux := LoggingMiddleware(mux)

	// 4. Configure http.Server with explicit timeouts to prevent hung or slow client connections.
	server := &http.Server{
		Addr:         ":8080",
		Handler:      loggedMux,
		ReadTimeout:  5 * time.Second,  // Limits the time allowed to read the incoming request.
		WriteTimeout: 10 * time.Second, // Limits the time allowed to respond to the client.
		IdleTimeout:  60 * time.Second, // Keeps keep-alive connections alive for 60 seconds of inactivity.
	}

	log.Println("Server listening on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server stopped unexpectedly: %v", err)
	}
}
