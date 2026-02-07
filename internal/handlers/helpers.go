// Package handlers contains the HTTP handler functions for the Tinder-Claude
// REST API. Handlers are the "glue" between incoming HTTP requests and the
// business logic in the services layer.
//
// This file provides shared helper functions used across all handlers.
// In Go's net/http package, a handler is any function with the signature:
//
//	func(w http.ResponseWriter, r *http.Request)
//
// The ResponseWriter is where we write our response, and the Request contains
// all the information about the incoming HTTP request.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dlfelps/tinder-go-claude/internal/models"
)

// writeJSON is a helper that serializes a value to JSON and writes it to the
// HTTP response with the correct Content-Type header and status code.
//
// This is a common pattern in Go HTTP servers — you'll write a helper like
// this in almost every project to avoid repeating the same boilerplate.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	// Set the Content-Type header BEFORE calling WriteHeader.
	// In Go's net/http, headers must be set before the status code is written,
	// because WriteHeader sends the headers to the client immediately.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// json.NewEncoder writes directly to the ResponseWriter (which implements
	// io.Writer). This is more efficient than json.Marshal + w.Write because
	// it avoids an intermediate byte slice allocation.
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If JSON encoding fails (rare, but possible with unusual types),
		// log the error. At this point the status code is already sent,
		// so we can't change it.
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// writeSuccess writes a successful API response with the standard envelope.
func writeSuccess(w http.ResponseWriter, status int, data interface{}, meta map[string]any) {
	writeJSON(w, status, models.NewSuccessResponse(data, meta))
}

// writeError writes an error API response with the standard envelope.
func writeError(w http.ResponseWriter, status int, messages ...string) {
	writeJSON(w, status, models.NewErrorResponse(messages...))
}
