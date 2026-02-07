// Package store contains tests for the InMemoryStore.
//
// Go testing basics:
//   - Test files end with _test.go (the Go toolchain automatically excludes
//     them from production builds).
//   - Test functions start with "Test" and take a *testing.T parameter.
//   - Run tests with: go test ./internal/store/
//   - The -v flag shows verbose output: go test -v ./internal/store/
//
// We use subtests (t.Run) to organize related test cases. Subtests appear
// as nested output in verbose mode, making it easy to identify which specific
// scenario failed.
package store

import (
	"testing"
	"time"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/google/uuid"
)

// resetStore is a test helper that clears the singleton store before each test.
// This ensures test isolation — no test depends on state from another test.
// In Python/pytest, this would be an "autouse" fixture.
func resetStore(t *testing.T) *InMemoryStore {
	t.Helper() // Marks this as a helper so stack traces point to the caller.
	s := GetStore()
	s.Reset()
	return s
}

// makeUser is a test helper that creates a User with the given name and zone.
// Helper functions like this reduce boilerplate in tests and make test code
// more readable.
func makeUser(name, zone string) models.User {
	return models.User{
		ID:     uuid.New(),
		Name:   name,
		Age:    25,
		Gender: "other",
		ZoneID: zone,
	}
}

// ---------------------------------------------------------------------------
// Singleton tests
// ---------------------------------------------------------------------------

func TestGetStore_ReturnsSameInstance(t *testing.T) {
	// The singleton pattern means every call to GetStore() should return
	// the exact same pointer. In Go, we compare pointers with ==.
	store1 := GetStore()
	store2 := GetStore()

	if store1 != store2 {
		t.Error("GetStore() should return the same instance (singleton pattern)")
	}
}

// ---------------------------------------------------------------------------
// User operation tests
// ---------------------------------------------------------------------------

func TestAddAndGetUser(t *testing.T) {
	s := resetStore(t)
	user := makeUser("Alice", "zone-a")

	s.AddUser(user)

	// Retrieve the user and verify all fields match.
	got, exists := s.GetUser(user.ID)
	if !exists {
		t.Fatal("expected user to exist after adding")
	}
	if got.Name != user.Name {
		t.Errorf("name: got %q, want %q", got.Name, user.Name)
	}
	if got.ZoneID != user.ZoneID {
		t.Errorf("zone_id: got %q, want %q", got.ZoneID, user.ZoneID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	s := resetStore(t)

	// Looking up a UUID that doesn't exist should return (zero-value, false).
	_, exists := s.GetUser(uuid.New())
	if exists {
		t.Error("expected user not to exist")
	}
}

func TestGetAllUsers(t *testing.T) {
	s := resetStore(t)

	// Start with no users.
	if users := s.GetAllUsers(); len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}

	// Add some users and verify the count.
	s.AddUser(makeUser("Alice", "zone-a"))
	s.AddUser(makeUser("Bob", "zone-a"))
	s.AddUser(makeUser("Charlie", "zone-b"))

	users := s.GetAllUsers()
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}
}

// ---------------------------------------------------------------------------
// Swipe operation tests
// ---------------------------------------------------------------------------

func TestAddSwipeAndGetByUser(t *testing.T) {
	s := resetStore(t)

	alice := makeUser("Alice", "zone-a")
	bob := makeUser("Bob", "zone-a")
	s.AddUser(alice)
	s.AddUser(bob)

	// Record a swipe from Alice to Bob.
	swipe := models.Swipe{
		SwiperID:  alice.ID,
		SwipedID:  bob.ID,
		Action:    models.SwipeActionLike,
		Timestamp: time.Now().UTC(),
	}
	s.AddSwipe(swipe)

	// Alice's swipes should contain the swipe.
	aliceSwipes := s.GetSwipesByUser(alice.ID)
	if len(aliceSwipes) != 1 {
		t.Fatalf("expected 1 swipe for Alice, got %d", len(aliceSwipes))
	}
	if aliceSwipes[0].SwipedID != bob.ID {
		t.Error("swipe should be directed at Bob")
	}

	// Bob hasn't swiped, so his swipes should be empty.
	bobSwipes := s.GetSwipesByUser(bob.ID)
	if len(bobSwipes) != 0 {
		t.Errorf("expected 0 swipes for Bob, got %d", len(bobSwipes))
	}
}

func TestFindSwipe(t *testing.T) {
	s := resetStore(t)

	alice := makeUser("Alice", "zone-a")
	bob := makeUser("Bob", "zone-a")

	swipe := models.Swipe{
		SwiperID:  alice.ID,
		SwipedID:  bob.ID,
		Action:    models.SwipeActionLike,
		Timestamp: time.Now().UTC(),
	}
	s.AddSwipe(swipe)

	// Should find the swipe from Alice to Bob.
	found := s.FindSwipe(alice.ID, bob.ID)
	if found == nil {
		t.Fatal("expected to find swipe from Alice to Bob")
	}
	if found.Action != models.SwipeActionLike {
		t.Errorf("action: got %q, want %q", found.Action, models.SwipeActionLike)
	}

	// Should NOT find a swipe from Bob to Alice (reverse direction).
	notFound := s.FindSwipe(bob.ID, alice.ID)
	if notFound != nil {
		t.Error("expected no swipe from Bob to Alice")
	}
}

// ---------------------------------------------------------------------------
// Match operation tests
// ---------------------------------------------------------------------------

func TestAddMatchAndGetForUser(t *testing.T) {
	s := resetStore(t)

	alice := makeUser("Alice", "zone-a")
	bob := makeUser("Bob", "zone-a")
	charlie := makeUser("Charlie", "zone-a")

	// Create a match between Alice and Bob.
	match := models.Match{
		User1ID:   alice.ID,
		User2ID:   bob.ID,
		Timestamp: time.Now().UTC(),
	}
	s.AddMatch(match)

	// Alice should see the match.
	aliceMatches := s.GetMatchesForUser(alice.ID)
	if len(aliceMatches) != 1 {
		t.Fatalf("expected 1 match for Alice, got %d", len(aliceMatches))
	}

	// Bob should also see the same match.
	bobMatches := s.GetMatchesForUser(bob.ID)
	if len(bobMatches) != 1 {
		t.Fatalf("expected 1 match for Bob, got %d", len(bobMatches))
	}

	// Charlie has no matches.
	charlieMatches := s.GetMatchesForUser(charlie.ID)
	if len(charlieMatches) != 0 {
		t.Errorf("expected 0 matches for Charlie, got %d", len(charlieMatches))
	}
}

// ---------------------------------------------------------------------------
// Reset tests
// ---------------------------------------------------------------------------

func TestReset_ClearsAllData(t *testing.T) {
	s := resetStore(t)

	// Add some data.
	user := makeUser("Alice", "zone-a")
	s.AddUser(user)
	s.AddSwipe(models.Swipe{
		SwiperID:  user.ID,
		SwipedID:  uuid.New(),
		Action:    models.SwipeActionPass,
		Timestamp: time.Now().UTC(),
	})
	s.AddMatch(models.Match{
		User1ID:   user.ID,
		User2ID:   uuid.New(),
		Timestamp: time.Now().UTC(),
	})

	// Reset should clear everything.
	s.Reset()

	if users := s.GetAllUsers(); len(users) != 0 {
		t.Errorf("expected 0 users after reset, got %d", len(users))
	}
	if swipes := s.GetSwipesByUser(user.ID); len(swipes) != 0 {
		t.Errorf("expected 0 swipes after reset, got %d", len(swipes))
	}
	if matches := s.GetMatchesForUser(user.ID); len(matches) != 0 {
		t.Errorf("expected 0 matches after reset, got %d", len(matches))
	}
}
