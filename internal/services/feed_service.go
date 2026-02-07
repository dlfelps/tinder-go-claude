// Package services contains the business logic layer of the Tinder-Claude
// application. Services coordinate between the HTTP handlers and the data store,
// enforcing business rules and performing complex operations.
//
// This file implements the FeedService, which generates a personalized
// discovery feed for a user by applying a three-tier filtering pipeline:
//
//  1. Zone Filter — only show users in the same geographic zone
//  2. Self-Exclusion — don't show the user their own profile
//  3. Seen-State Filter — don't show users already swiped on
package services

import (
	"fmt"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// FeedService handles the generation of personalized discovery feeds.
//
// In Go, services are typically structs that hold references to their
// dependencies (like the data store). This makes them easy to test —
// you can swap in a mock store during testing.
type FeedService struct {
	store *store.InMemoryStore
}

// NewFeedService creates a new FeedService connected to the given store.
// This is a constructor function — Go's convention for creating initialized
// struct instances. Unlike Python's __init__, Go doesn't have constructors
// built into the language; we use plain functions by convention.
func NewFeedService(s *store.InMemoryStore) *FeedService {
	return &FeedService{store: s}
}

// GetFeed generates a discovery feed for the given user by applying the
// three-tier filtering pipeline. It returns a slice of User models that
// the requesting user has not yet seen and who are in the same zone.
//
// The function returns an error if the requesting user doesn't exist.
// In Go, we return errors as values rather than throwing exceptions.
// The caller is expected to check the error before using the result.
func (fs *FeedService) GetFeed(userID uuid.UUID) ([]models.User, error) {
	// Step 0: Verify the requesting user exists.
	// The comma-ok idiom (value, ok := ...) is how Go handles lookups
	// that might fail — no exceptions needed.
	requestingUser, exists := fs.store.GetUser(userID)
	if !exists {
		return nil, fmt.Errorf("user %s not found", userID)
	}

	// Step 1: Get all users from the store.
	allUsers := fs.store.GetAllUsers()

	// Step 2: Build a set of already-swiped user IDs for O(1) lookup.
	// Go doesn't have a built-in Set type, so we use a map with empty struct
	// values. The empty struct (struct{}) takes zero bytes of memory, making
	// it the most efficient "set element" in Go.
	swipes := fs.store.GetSwipesByUser(userID)
	seenSet := make(map[uuid.UUID]struct{}, len(swipes))
	for _, swipe := range swipes {
		seenSet[swipe.SwipedID] = struct{}{}
	}

	// Step 3: Apply the three-tier filter pipeline.
	// We iterate through all users once (O(N)) and apply each filter in order.
	var feed []models.User
	for _, candidate := range allUsers {
		// Tier 1: Zone Filter — only include users in the same zone.
		if candidate.ZoneID != requestingUser.ZoneID {
			continue // Skip users in different zones.
		}

		// Tier 2: Self-Exclusion — don't include the requesting user.
		if candidate.ID == userID {
			continue // Skip self.
		}

		// Tier 3: Seen-State Filter — don't include already-swiped users.
		// The underscore (_) discards the value; we only care if the key exists.
		if _, alreadySeen := seenSet[candidate.ID]; alreadySeen {
			continue // Skip users we've already swiped on.
		}

		// The candidate passed all three filters — add them to the feed.
		feed = append(feed, candidate)
	}

	// Return an empty slice instead of nil so JSON serialization produces
	// "[]" instead of "null". This is a common Go idiom for API responses.
	if feed == nil {
		feed = []models.User{}
	}

	return feed, nil
}
