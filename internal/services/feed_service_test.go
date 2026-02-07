// Package services contains tests for the FeedService.
//
// These unit tests verify the three-tier filtering pipeline:
//  1. Zone filter — only same-zone users appear
//  2. Self-exclusion — the requesting user is removed
//  3. Seen-state filter — already-swiped users are removed
package services

import (
	"testing"
	"time"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// setupFeedTest is a helper that resets the store and creates a FeedService.
// Returning both allows tests to add data to the store and call service methods.
func setupFeedTest(t *testing.T) (*FeedService, *store.InMemoryStore) {
	t.Helper()
	s := store.GetStore()
	s.Reset()
	return NewFeedService(s), s
}

// makeTestUser creates and stores a user with the given name and zone.
// It returns the created User for use in assertions.
func makeTestUser(s *store.InMemoryStore, name, zone string) models.User {
	user := models.User{
		ID:     uuid.New(),
		Name:   name,
		Age:    25,
		Gender: "other",
		ZoneID: zone,
	}
	s.AddUser(user)
	return user
}

// ---------------------------------------------------------------------------
// Feed filtering tests
// ---------------------------------------------------------------------------

func TestGetFeed_UserNotFound(t *testing.T) {
	fs, _ := setupFeedTest(t)

	// Requesting a feed for a non-existent user should return an error.
	_, err := fs.GetFeed(uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestGetFeed_ZoneFiltering(t *testing.T) {
	fs, s := setupFeedTest(t)

	// Create users in two different zones.
	alice := makeTestUser(s, "Alice", "zone-a")
	makeTestUser(s, "Bob", "zone-a")     // Same zone as Alice.
	makeTestUser(s, "Charlie", "zone-b") // Different zone.

	feed, err := fs.GetFeed(alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Alice should only see Bob (same zone). Charlie is in a different zone.
	if len(feed) != 1 {
		t.Fatalf("expected 1 user in feed, got %d", len(feed))
	}
	if feed[0].Name != "Bob" {
		t.Errorf("expected Bob in feed, got %s", feed[0].Name)
	}
}

func TestGetFeed_SelfExclusion(t *testing.T) {
	fs, s := setupFeedTest(t)

	// Create a single user — their feed should be empty (only themselves in zone).
	alice := makeTestUser(s, "Alice", "zone-a")

	feed, err := fs.GetFeed(alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Alice should not see herself in the feed.
	if len(feed) != 0 {
		t.Errorf("expected empty feed (self-exclusion), got %d users", len(feed))
	}
}

func TestGetFeed_SeenStateFiltering(t *testing.T) {
	fs, s := setupFeedTest(t)

	// Create three users in the same zone.
	alice := makeTestUser(s, "Alice", "zone-a")
	bob := makeTestUser(s, "Bob", "zone-a")
	charlie := makeTestUser(s, "Charlie", "zone-a")

	// Alice swipes on Bob — Bob should no longer appear in Alice's feed.
	s.AddSwipe(models.Swipe{
		SwiperID:  alice.ID,
		SwipedID:  bob.ID,
		Action:    models.SwipeActionLike,
		Timestamp: time.Now().UTC(),
	})

	feed, err := fs.GetFeed(alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only Charlie should remain (Bob was swiped on).
	if len(feed) != 1 {
		t.Fatalf("expected 1 user in feed, got %d", len(feed))
	}
	if feed[0].ID != charlie.ID {
		t.Errorf("expected Charlie in feed, got %s", feed[0].Name)
	}
}

func TestGetFeed_PassAlsoFiltersSeen(t *testing.T) {
	fs, s := setupFeedTest(t)

	// PASS swipes should also remove users from the feed (not just LIKEs).
	alice := makeTestUser(s, "Alice", "zone-a")
	bob := makeTestUser(s, "Bob", "zone-a")

	s.AddSwipe(models.Swipe{
		SwiperID:  alice.ID,
		SwipedID:  bob.ID,
		Action:    models.SwipeActionPass,
		Timestamp: time.Now().UTC(),
	})

	feed, err := fs.GetFeed(alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(feed) != 0 {
		t.Errorf("expected empty feed after PASS, got %d users", len(feed))
	}
}

func TestGetFeed_EmptyFeedReturnsEmptySlice(t *testing.T) {
	fs, s := setupFeedTest(t)

	// When the feed is empty, we should get an empty slice (not nil).
	// This is important for JSON serialization: [] vs null.
	alice := makeTestUser(s, "Alice", "zone-a")

	feed, err := fs.GetFeed(alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if feed == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(feed) != 0 {
		t.Errorf("expected 0 users, got %d", len(feed))
	}
}

func TestGetFeed_MultipleZones(t *testing.T) {
	fs, s := setupFeedTest(t)

	// Test with users spread across multiple zones.
	alice := makeTestUser(s, "Alice", "zone-a")
	makeTestUser(s, "Bob", "zone-a")
	makeTestUser(s, "Charlie", "zone-b")
	makeTestUser(s, "Diana", "zone-c")
	makeTestUser(s, "Eve", "zone-a")

	feed, err := fs.GetFeed(alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Alice should see Bob and Eve (same zone-a), but not Charlie or Diana.
	if len(feed) != 2 {
		t.Errorf("expected 2 users in feed, got %d", len(feed))
	}

	// Verify all feed users are in zone-a.
	for _, user := range feed {
		if user.ZoneID != "zone-a" {
			t.Errorf("user %s is in zone %s, expected zone-a", user.Name, user.ZoneID)
		}
	}
}
