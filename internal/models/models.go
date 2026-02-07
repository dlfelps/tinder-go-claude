// Package models defines the core data types used throughout the Tinder-Claude
// application. In Go, we use structs (similar to classes in other languages) to
// represent our domain entities: Users, Swipes, and Matches.
//
// Go doesn't have enums like Python or Java. Instead, we use a combination of
// a custom type and constants (called "iota" pattern) to achieve similar behavior.
//
// Struct tags (the `json:"..."` annotations) control how Go marshals and
// unmarshals JSON. This is how we map between Go field names (PascalCase by
// convention) and JSON field names (snake_case by convention).
package models

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// SwipeAction enum
// ---------------------------------------------------------------------------

// SwipeAction represents the type of swipe a user can perform.
// In Go, we define "enums" by creating a new type based on a primitive type
// (here, string) and then defining constants of that type.
type SwipeAction string

const (
	// SwipeActionLike indicates the user is interested in the other user.
	SwipeActionLike SwipeAction = "LIKE"

	// SwipeActionPass indicates the user is not interested.
	SwipeActionPass SwipeAction = "PASS"
)

// IsValid checks whether a SwipeAction contains a recognized value.
// Since Go doesn't enforce enum membership the way Python's Enum class does,
// we need to validate manually.
func (s SwipeAction) IsValid() bool {
	switch s {
	case SwipeActionLike, SwipeActionPass:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Core domain models
// ---------------------------------------------------------------------------

// User represents a dating profile in the system. Each user belongs to a
// geographic "zone" that determines which other users appear in their feed.
//
// The `json` struct tags tell Go's encoding/json package how to serialize
// and deserialize this struct. For example, `json:"id"` means the Go field
// "ID" will appear as "id" in JSON output.
type User struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Age    int       `json:"age"`
	Gender string    `json:"gender"`
	ZoneID string    `json:"zone_id"`
}

// Swipe records a single swipe action — one user expressing interest (LIKE)
// or disinterest (PASS) in another user.
type Swipe struct {
	SwiperID  uuid.UUID   `json:"swiper_id"`
	SwipedID  uuid.UUID   `json:"swiped_id"`
	Action    SwipeAction `json:"action"`
	Timestamp time.Time   `json:"timestamp"`
}

// Match represents a mutual connection between two users. A match is created
// when both users have LIKED each other (bidirectional match detection).
type Match struct {
	User1ID   uuid.UUID `json:"user1_id"`
	User2ID   uuid.UUID `json:"user2_id"`
	Timestamp time.Time `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// API request and response types
// ---------------------------------------------------------------------------

// CreateUserRequest is the JSON body expected when creating a new user.
// We keep request types separate from domain models so we can validate input
// independently of how data is stored. Notice there's no ID field — the server
// generates that.
type CreateUserRequest struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Gender string `json:"gender"`
	ZoneID string `json:"zone_id"`
}

// Validate checks that all required fields in a CreateUserRequest are present
// and valid. In Python/FastAPI, Pydantic handles this automatically. In Go,
// we typically write explicit validation functions.
func (r CreateUserRequest) Validate() []string {
	// We collect all validation errors into a slice so the caller gets
	// a complete picture of what's wrong, rather than failing on the first error.
	var errs []string

	if r.Name == "" {
		errs = append(errs, "name is required")
	}
	if r.Age <= 0 {
		errs = append(errs, "age must be a positive integer")
	}
	if r.Gender == "" {
		errs = append(errs, "gender is required")
	}
	if r.ZoneID == "" {
		errs = append(errs, "zone_id is required")
	}

	return errs
}

// CreateSwipeRequest is the JSON body expected when recording a swipe.
type CreateSwipeRequest struct {
	SwiperID string `json:"swiper_id"`
	SwipedID string `json:"swiped_id"`
	Action   string `json:"action"`
}

// Validate checks that the swipe request has valid UUIDs and a recognized action.
func (r CreateSwipeRequest) Validate() (swiperID, swipedID uuid.UUID, action SwipeAction, errs []string) {
	var err error

	// Parse and validate the swiper UUID.
	swiperID, err = uuid.Parse(r.SwiperID)
	if err != nil {
		errs = append(errs, "swiper_id must be a valid UUID")
	}

	// Parse and validate the swiped UUID.
	swipedID, err = uuid.Parse(r.SwipedID)
	if err != nil {
		errs = append(errs, "swiped_id must be a valid UUID")
	}

	// Validate the action is a known SwipeAction.
	action = SwipeAction(r.Action)
	if !action.IsValid() {
		errs = append(errs, "action must be LIKE or PASS")
	}

	return swiperID, swipedID, action, errs
}

// ---------------------------------------------------------------------------
// API response envelope
// ---------------------------------------------------------------------------

// APIResponse is the standardized response envelope used by all endpoints.
// Every API response wraps its payload in this structure so clients always
// know where to find data, metadata, and errors. This is sometimes called
// the "response envelope" pattern.
//
// The `interface{}` type (or `any` in Go 1.18+) means "Data" can hold any
// value — a single user, a list of users, a boolean, etc.
type APIResponse struct {
	Data   interface{}    `json:"data"`
	Meta   map[string]any `json:"meta"`
	Errors []APIError     `json:"errors"`
}

// APIError represents a single error message in the response envelope.
type APIError struct {
	Message string `json:"message"`
}

// NewSuccessResponse is a helper that builds a successful API response with
// the given data and optional metadata.
func NewSuccessResponse(data interface{}, meta map[string]any) APIResponse {
	// If no metadata was provided, initialize an empty map so the JSON output
	// always contains "meta": {} rather than "meta": null.
	if meta == nil {
		meta = map[string]any{}
	}
	return APIResponse{
		Data:   data,
		Meta:   meta,
		Errors: []APIError{}, // Empty slice serializes to [] instead of null.
	}
}

// NewErrorResponse is a helper that builds an error API response with one
// or more error messages.
func NewErrorResponse(messages ...string) APIResponse {
	errors := make([]APIError, 0, len(messages))
	for _, msg := range messages {
		errors = append(errors, APIError{Message: msg})
	}
	return APIResponse{
		Data:   nil,
		Meta:   map[string]any{},
		Errors: errors,
	}
}
