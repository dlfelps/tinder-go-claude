// This file implements the SwipeService, which handles swipe processing and
// bidirectional match detection. When a user swipes LIKE on someone who has
// also LIKED them, a Match is created.
package services

import (
	"fmt"
	"time"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// SwipeService handles swipe recording and mutual match detection.
type SwipeService struct {
	store *store.InMemoryStore
}

// NewSwipeService creates a new SwipeService connected to the given store.
func NewSwipeService(s *store.InMemoryStore) *SwipeService {
	return &SwipeService{store: s}
}

// ProcessSwipeResult holds the outcome of processing a swipe action.
// By using a result struct instead of multiple return values, we keep
// the API clean and make it easy to add more fields in the future.
type ProcessSwipeResult struct {
	// Swipe is the recorded swipe action.
	Swipe models.Swipe

	// Matched indicates whether a mutual match was detected.
	Matched bool

	// Match contains the match details if Matched is true.
	// Using a pointer (*models.Match) lets us represent "no match" as nil.
	Match *models.Match
}

// ProcessSwipe validates and records a swipe action, then checks for a
// mutual match. It enforces several business rules:
//   - Both the swiper and swiped users must exist (404 error)
//   - A user cannot swipe on themselves (400 error)
//
// The function returns a structured result and an error. In Go, we often
// need to distinguish between different types of errors. Here we use a
// simple approach: the error message contains enough context for the
// handler to determine the appropriate HTTP status code.
func (ss *SwipeService) ProcessSwipe(swiperID, swipedID uuid.UUID, action models.SwipeAction) (*ProcessSwipeResult, error) {
	// Validate business rules.

	// Rule 1: Users cannot swipe on themselves.
	// We check this first because it doesn't require a database lookup.
	if swiperID == swipedID {
		return nil, &ValidationError{Message: "cannot swipe on yourself"}
	}

	// Rule 2: The swiper must exist.
	if _, exists := ss.store.GetUser(swiperID); !exists {
		return nil, &NotFoundError{Message: fmt.Sprintf("swiper user %s not found", swiperID)}
	}

	// Rule 3: The swiped user must exist.
	if _, exists := ss.store.GetUser(swipedID); !exists {
		return nil, &NotFoundError{Message: fmt.Sprintf("swiped user %s not found", swipedID)}
	}

	// Record the swipe.
	swipe := models.Swipe{
		SwiperID:  swiperID,
		SwipedID:  swipedID,
		Action:    action,
		Timestamp: time.Now().UTC(),
	}
	ss.store.AddSwipe(swipe)

	result := &ProcessSwipeResult{
		Swipe:   swipe,
		Matched: false,
	}

	// Check for mutual match: only LIKE actions can create matches.
	// We look for a "reverse" swipe — did the other user also LIKE us?
	if action == models.SwipeActionLike {
		reverseSwipe := ss.store.FindSwipe(swipedID, swiperID)

		// If a reverse swipe exists and it's also a LIKE, we have a match!
		if reverseSwipe != nil && reverseSwipe.Action == models.SwipeActionLike {
			match := models.Match{
				User1ID:   swiperID,
				User2ID:   swipedID,
				Timestamp: time.Now().UTC(),
			}
			ss.store.AddMatch(match)
			result.Matched = true
			result.Match = &match
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Custom error types
// ---------------------------------------------------------------------------

// Go uses error interfaces for error handling. By defining custom error types,
// we can carry structured information about what went wrong. The HTTP handler
// can then use type assertions (or errors.As) to decide the right status code.

// NotFoundError indicates that a required resource was not found.
// This maps to HTTP 404.
type NotFoundError struct {
	Message string
}

// Error implements the error interface. Any type with an Error() string method
// satisfies Go's built-in `error` interface — this is called "duck typing"
// or "structural typing" in Go.
func (e *NotFoundError) Error() string {
	return e.Message
}

// ValidationError indicates a business rule violation (e.g., self-swipe).
// This maps to HTTP 400 Bad Request.
type ValidationError struct {
	Message string
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	return e.Message
}
