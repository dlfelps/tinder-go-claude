// Package main is the entry point for the Tinder-Claude API server.
//
// In Go, every executable program must have a `main` package with a `main()`
// function. This is where the program starts running — similar to Python's
// `if __name__ == "__main__":` pattern, but enforced by the language.
//
// This file sets up the HTTP router, initializes dependencies, and starts
// the server. It uses Go 1.22+'s enhanced ServeMux which supports method-based
// routing (e.g., "GET /users/{id}") without needing a third-party router.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dlfelps/tinder-go-claude/internal/handlers"
	"github.com/dlfelps/tinder-go-claude/internal/services"
	"github.com/dlfelps/tinder-go-claude/internal/store"
)

func main() {
	// -----------------------------------------------------------------------
	// Dependency initialization
	// -----------------------------------------------------------------------
	// We follow the "dependency injection" pattern: create dependencies at the
	// top level and pass them down. This makes the code testable and the
	// dependency graph explicit.

	// Get the shared in-memory store (singleton).
	dataStore := store.GetStore()

	// Create services with their dependencies.
	feedService := services.NewFeedService(dataStore)
	swipeService := services.NewSwipeService(dataStore)

	// Create handlers with their dependencies.
	userHandler := handlers.NewUserHandler(dataStore)
	feedHandler := handlers.NewFeedHandler(feedService)
	swipeHandler := handlers.NewSwipeHandler(swipeService, dataStore)

	// -----------------------------------------------------------------------
	// Router setup
	// -----------------------------------------------------------------------
	// http.NewServeMux() creates a new request multiplexer (router). In Go 1.22+,
	// ServeMux supports patterns like "GET /users/{id}" that include both the
	// HTTP method and path parameters. This eliminates the need for third-party
	// routers like gorilla/mux or chi for most use cases.

	mux := http.NewServeMux()

	// Register routes. The pattern format is: "METHOD /path"
	// Path parameters use {name} syntax and are accessed via r.PathValue("name").

	// Health check — GET /
	mux.HandleFunc("GET /", handlers.HealthCheck)

	// User endpoints
	mux.HandleFunc("POST /users/", userHandler.CreateUser)    // Create user
	mux.HandleFunc("GET /users/{id}", userHandler.GetUser)     // Get user by ID

	// Feed endpoint
	mux.HandleFunc("GET /feed", feedHandler.GetFeed) // Get discovery feed

	// Swipe and match endpoints
	mux.HandleFunc("POST /swipe", swipeHandler.CreateSwipe)  // Record a swipe
	mux.HandleFunc("GET /matches", swipeHandler.GetMatches)  // List matches

	// -----------------------------------------------------------------------
	// Server startup
	// -----------------------------------------------------------------------
	// Determine the port to listen on. We use an environment variable so the
	// port can be configured without changing code (12-factor app principle).
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000" // Default port matches the original FastAPI/Uvicorn default.
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Tinder-Claude API server starting on http://localhost%s", addr)

	// http.ListenAndServe starts the HTTP server. It blocks (runs forever)
	// until the server encounters a fatal error. If it returns an error,
	// we log it and exit. This is equivalent to uvicorn.run() in FastAPI.
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
