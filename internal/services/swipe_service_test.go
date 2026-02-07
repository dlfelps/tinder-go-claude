// This file contains unit tests for the SwipeService, covering:
//   - Successful swipe recording
//   - Mutual match detection (bidirectional LIKE)
//   - Business rule enforcement (self-swipe prevention, user existence)
package services

import (
	"testing"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// setupSwipeTest resets the store and creates a SwipeService for testing.
func setupSwipeTest(t *testing.T) (*SwipeService, *store.InMemoryStore) {
	t.Helper()
	s := store.GetStore()
	s.Reset()
	return NewSwipeService(s), s
}

// ---------------------------------------------------------------------------
// Swipe processing tests
// ---------------------------------------------------------------------------

func TestProcessSwipe_RecordsSwipe(t *testing.T) {
	ss, s := setupSwipeTest(t)

	alice := makeTestUser(s, "Alice", "zone-a")
	bob := makeTestUser(s, "Bob", "zone-a")

	result, err := ss.ProcessSwipe(alice.ID, bob.ID, models.SwipeActionLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The swipe should be recorded.
	if result.Swipe.SwiperID != alice.ID {
		t.Error("swiper ID mismatch")
	}
	if result.Swipe.SwipedID != bob.ID {
		t.Error("swiped ID mismatch")
	}
	if result.Swipe.Action != models.SwipeActionLike {
		t.Error("action mismatch")
	}

	// No match yet (one-sided LIKE).
	if result.Matched {
		t.Error("expected no match on one-sided LIKE")
	}
}

func TestProcessSwipe_MutualLikeCreatesMatch(t *testing.T) {
	ss, s := setupSwipeTest(t)

	alice := makeTestUser(s, "Alice", "zone-a")
	bob := makeTestUser(s, "Bob", "zone-a")

	// Alice likes Bob — no match yet.
	result1, err := ss.ProcessSwipe(alice.ID, bob.ID, models.SwipeActionLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.Matched {
		t.Error("expected no match after first LIKE")
	}

	// Bob likes Alice — this should create a match!
	result2, err := ss.ProcessSwipe(bob.ID, alice.ID, models.SwipeActionLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result2.Matched {
		t.Fatal("expected a match on mutual LIKE")
	}
	if result2.Match == nil {
		t.Fatal("expected match details to be present")
	}

	// Verify the match is stored.
	matches := s.GetMatchesForUser(alice.ID)
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestProcessSwipe_LikeAndPassNoMatch(t *testing.T) {
	ss, s := setupSwipeTest(t)

	alice := makeTestUser(s, "Alice", "zone-a")
	bob := makeTestUser(s, "Bob", "zone-a")

	// Alice likes Bob.
	_, err := ss.ProcessSwipe(alice.ID, bob.ID, models.SwipeActionLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bob passes on Alice — no match should be created.
	result, err := ss.ProcessSwipe(bob.ID, alice.ID, models.SwipeActionPass)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Matched {
		t.Error("expected no match when one user passes")
	}
}

func TestProcessSwipe_PassAndLikeNoMatch(t *testing.T) {
	ss, s := setupSwipeTest(t)

	alice := makeTestUser(s, "Alice", "zone-a")
	bob := makeTestUser(s, "Bob", "zone-a")

	// Alice passes on Bob.
	_, err := ss.ProcessSwipe(alice.ID, bob.ID, models.SwipeActionPass)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bob likes Alice — no match because Alice passed.
	result, err := ss.ProcessSwipe(bob.ID, alice.ID, models.SwipeActionLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Matched {
		t.Error("expected no match when first user passed")
	}
}

// ---------------------------------------------------------------------------
// Business rule enforcement tests
// ---------------------------------------------------------------------------

func TestProcessSwipe_SelfSwipePrevented(t *testing.T) {
	ss, s := setupSwipeTest(t)

	alice := makeTestUser(s, "Alice", "zone-a")

	// A user should not be able to swipe on themselves.
	_, err := ss.ProcessSwipe(alice.ID, alice.ID, models.SwipeActionLike)
	if err == nil {
		t.Fatal("expected error for self-swipe")
	}

	// The error should be a ValidationError (HTTP 400).
	if _, ok := err.(*ValidationError); !ok {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestProcessSwipe_SwiperNotFound(t *testing.T) {
	ss, s := setupSwipeTest(t)

	bob := makeTestUser(s, "Bob", "zone-a")

	// Non-existent swiper should return NotFoundError.
	_, err := ss.ProcessSwipe(uuid.New(), bob.ID, models.SwipeActionLike)
	if err == nil {
		t.Fatal("expected error for non-existent swiper")
	}

	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestProcessSwipe_SwipedNotFound(t *testing.T) {
	ss, s := setupSwipeTest(t)

	alice := makeTestUser(s, "Alice", "zone-a")

	// Non-existent swiped user should return NotFoundError.
	_, err := ss.ProcessSwipe(alice.ID, uuid.New(), models.SwipeActionLike)
	if err == nil {
		t.Fatal("expected error for non-existent swiped user")
	}

	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}
