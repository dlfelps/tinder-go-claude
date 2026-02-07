// This file contains the HTTP handler for the discovery feed endpoint:
//   - GET /feed?user_id=<uuid> — Get a filtered discovery feed for a user
package handlers

import (
	"net/http"

	"github.com/dlfelps/tinder-go-claude/internal/services"
	"github.com/google/uuid"
)

// FeedHandler handles feed-related HTTP requests.
type FeedHandler struct {
	feedService *services.FeedService
}

// NewFeedHandler creates a new FeedHandler with the given feed service.
func NewFeedHandler(fs *services.FeedService) *FeedHandler {
	return &FeedHandler{feedService: fs}
}

// GetFeed handles GET /feed?user_id=<uuid> — returns a personalized
// discovery feed for the given user.
//
// Query parameters in Go are accessed through r.URL.Query(), which returns
// a url.Values (essentially a map[string][]string). This is different from
// FastAPI where query parameters are declared as function arguments.
func (h *FeedHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract the user_id query parameter.
	// r.URL.Query().Get() returns an empty string if the parameter is missing.
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		writeError(w, http.StatusUnprocessableEntity, "user_id query parameter is required")
		return
	}

	// Step 2: Parse the user_id as a UUID.
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "user_id must be a valid UUID")
		return
	}

	// Step 3: Call the feed service to generate the filtered feed.
	// The service handles all the business logic (zone filtering, self-exclusion,
	// seen-state filtering). The handler just coordinates the HTTP layer.
	feed, err := h.feedService.GetFeed(userID)
	if err != nil {
		// If the service returns an error, it means the user wasn't found.
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Step 4: Return the feed with a count in the metadata.
	// The "count" meta field tells the client how many profiles are in the feed
	// without requiring them to check the array length.
	writeSuccess(w, http.StatusOK, feed, map[string]any{
		"count": len(feed),
	})
}
