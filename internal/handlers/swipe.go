// This file contains HTTP handlers for swipe and match endpoints:
//   - POST /swipe         — Submit a swipe action (LIKE or PASS)
//   - GET  /matches?user_id=<uuid> — List all matches for a user
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/services"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// SwipeHandler handles swipe and match HTTP requests.
type SwipeHandler struct {
	swipeService *services.SwipeService
	store        *store.InMemoryStore
}

// NewSwipeHandler creates a new SwipeHandler with the given swipe service
// and store. The store is needed for the GetMatches handler to verify user
// existence.
func NewSwipeHandler(ss *services.SwipeService, s *store.InMemoryStore) *SwipeHandler {
	return &SwipeHandler{
		swipeService: ss,
		store:        s,
	}
}

// CreateSwipe handles POST /swipe — records a swipe action and checks for
// mutual matches.
//
// This is the most complex handler because it needs to:
//  1. Parse and validate the request body
//  2. Delegate to the swipe service for business logic
//  3. Handle different error types (not found vs. validation errors)
//  4. Return different response shapes based on whether a match occurred
func (h *SwipeHandler) CreateSwipe(w http.ResponseWriter, r *http.Request) {
	// Step 1: Decode the JSON request body.
	var req models.CreateSwipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON in request body")
		return
	}

	// Step 2: Validate the request.
	// The Validate method returns parsed UUIDs and action along with errors,
	// so we don't have to parse them again if validation succeeds.
	swiperID, swipedID, action, errs := req.Validate()
	if len(errs) > 0 {
		writeError(w, http.StatusUnprocessableEntity, errs...)
		return
	}

	// Step 3: Process the swipe through the service layer.
	result, err := h.swipeService.ProcessSwipe(swiperID, swipedID, action)
	if err != nil {
		// Use Go's errors.As() to check the type of error and determine
		// the appropriate HTTP status code. This is Go's type-safe alternative
		// to Python's isinstance() or except clauses.
		var notFoundErr *services.NotFoundError
		var validationErr *services.ValidationError

		switch {
		case errors.As(err, &notFoundErr):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.As(err, &validationErr):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	// Step 4: Build the response payload.
	// The response includes the swipe details and match information.
	responseData := map[string]any{
		"swipe":   result.Swipe,
		"matched": result.Matched,
	}

	// If a match was detected, include the match details in the response.
	if result.Match != nil {
		responseData["match"] = result.Match
	}

	writeSuccess(w, http.StatusCreated, responseData, nil)
}

// GetMatches handles GET /matches?user_id=<uuid> — returns all matches
// for the given user.
func (h *SwipeHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract and validate the user_id query parameter.
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		writeError(w, http.StatusUnprocessableEntity, "user_id query parameter is required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "user_id must be a valid UUID")
		return
	}

	// Step 2: Verify the user exists before querying matches.
	if _, exists := h.store.GetUser(userID); !exists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Step 3: Retrieve all matches for the user.
	matches := h.store.GetMatchesForUser(userID)

	// Ensure we return an empty array rather than null in JSON.
	if matches == nil {
		matches = []models.Match{}
	}

	writeSuccess(w, http.StatusOK, matches, map[string]any{
		"count": len(matches),
	})
}
