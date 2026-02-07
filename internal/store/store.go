// Package store provides an in-memory data store for the Tinder-Claude
// application. It acts as a simple "database" that holds users, swipes, and
// matches in memory using Go's built-in data structures.
//
// Key Go concepts demonstrated here:
//   - sync.Mutex for thread-safe access to shared data
//   - Maps (hash tables) for O(1) lookups by ID
//   - Slices (dynamic arrays) for ordered collections
//   - The sync package for concurrency primitives
//
// In production, you would replace this with a real database (e.g., PostgreSQL),
// but an in-memory store is perfect for prototyping and learning.
package store

import (
	"sync"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/google/uuid"
)

// InMemoryStore holds all application data in memory. It is safe for
// concurrent use because all methods acquire a mutex lock before reading
// or writing data.
//
// In Go, we achieve thread safety with sync.Mutex rather than Python's GIL
// or asyncio locks. The mutex ensures that only one goroutine can access
// the store's data at a time.
type InMemoryStore struct {
	// mu protects all fields below from concurrent access.
	// Convention: always lock mu before reading or writing any field.
	mu sync.Mutex

	// users maps user IDs to User structs for O(1) lookups.
	users map[uuid.UUID]models.User

	// swipes stores all swipe records in chronological order.
	swipes []models.Swipe

	// matches stores all match records in chronological order.
	matches []models.Match
}

// ---------------------------------------------------------------------------
// Singleton pattern
// ---------------------------------------------------------------------------

// defaultStore is the package-level singleton instance. In Go, singletons
// are typically implemented as package-level variables, sometimes protected
// by sync.Once for lazy initialization. Here we use a simple variable since
// we want it available immediately.
var defaultStore = &InMemoryStore{
	users:   make(map[uuid.UUID]models.User),
	swipes:  make([]models.Swipe, 0),
	matches: make([]models.Match, 0),
}

// GetStore returns the singleton InMemoryStore instance. Every part of the
// application that needs data access calls this function to get the same
// shared store.
func GetStore() *InMemoryStore {
	return defaultStore
}

// ---------------------------------------------------------------------------
// User operations
// ---------------------------------------------------------------------------

// AddUser stores a new user in the store. The user's ID should already be
// set before calling this method.
func (s *InMemoryStore) AddUser(user models.User) {
	// Lock the mutex before writing. The deferred Unlock ensures the mutex
	// is released even if a panic occurs (defensive programming).
	s.mu.Lock()
	defer s.mu.Unlock()

	s.users[user.ID] = user
}

// GetUser retrieves a user by their UUID. It returns the user and a boolean
// indicating whether the user was found.
//
// This follows the Go convention of returning (value, ok) instead of raising
// exceptions. The caller checks the boolean to handle the "not found" case.
func (s *InMemoryStore) GetUser(id uuid.UUID) (models.User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	return user, exists
}

// GetAllUsers returns a slice containing all users in the store. The order
// is not guaranteed because Go maps do not maintain insertion order.
func (s *InMemoryStore) GetAllUsers() []models.User {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Pre-allocate the slice with the exact capacity we need. This avoids
	// unnecessary memory reallocations as we append items.
	result := make([]models.User, 0, len(s.users))
	for _, user := range s.users {
		result = append(result, user)
	}
	return result
}

// ---------------------------------------------------------------------------
// Swipe operations
// ---------------------------------------------------------------------------

// AddSwipe records a new swipe action in the store.
func (s *InMemoryStore) AddSwipe(swipe models.Swipe) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.swipes = append(s.swipes, swipe)
}

// GetSwipesByUser returns all swipe records where the given user was the swiper.
// This is used by the feed service to determine which users have already been
// swiped on (the "seen-state" filter).
func (s *InMemoryStore) GetSwipesByUser(userID uuid.UUID) []models.Swipe {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Filter the swipes slice for entries matching the given swiper ID.
	// In Go, there's no built-in "filter" function like Python's list
	// comprehension — we use a simple loop instead.
	var result []models.Swipe
	for _, swipe := range s.swipes {
		if swipe.SwiperID == userID {
			result = append(result, swipe)
		}
	}
	return result
}

// FindSwipe searches for a specific swipe from one user to another.
// It returns a pointer to the Swipe if found, or nil if no such swipe exists.
//
// Using a pointer return (*models.Swipe) is the Go idiom for "maybe a value."
// Python would use Optional[Swipe] or return None; Go uses nil pointers.
func (s *InMemoryStore) FindSwipe(swiperID, swipedID uuid.UUID) *models.Swipe {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Linear scan through all swipes. In production, you'd want an index
	// (e.g., a map keyed by (swiperID, swipedID)) for O(1) lookup.
	for _, swipe := range s.swipes {
		if swipe.SwiperID == swiperID && swipe.SwipedID == swipedID {
			// Return a pointer to a copy of the swipe. We copy it so the
			// caller can't accidentally modify the store's internal data.
			result := swipe
			return &result
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Match operations
// ---------------------------------------------------------------------------

// AddMatch records a new mutual match between two users.
func (s *InMemoryStore) AddMatch(match models.Match) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.matches = append(s.matches, match)
}

// GetMatchesForUser returns all matches involving the given user, regardless
// of whether they are user1 or user2 in the match record.
func (s *InMemoryStore) GetMatchesForUser(userID uuid.UUID) []models.Match {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []models.Match
	for _, match := range s.matches {
		// A user could be on either side of the match, so we check both.
		if match.User1ID == userID || match.User2ID == userID {
			result = append(result, match)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// Reset clears all data from the store. This is primarily used in tests to
// ensure each test starts with a clean slate (test isolation).
func (s *InMemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reinitialize all data structures. Using make() creates fresh, empty
	// maps and slices, allowing the garbage collector to reclaim the old data.
	s.users = make(map[uuid.UUID]models.User)
	s.swipes = make([]models.Swipe, 0)
	s.matches = make([]models.Match, 0)
}
