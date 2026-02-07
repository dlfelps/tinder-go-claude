// Package handlers contains integration tests for the Tinder-Claude REST API.
//
// These tests exercise the full HTTP request/response cycle using Go's
// httptest package. This is the Go equivalent of FastAPI's TestClient —
// it creates an in-memory HTTP server that processes real HTTP requests
// without opening a network port.
//
// Key Go testing concepts demonstrated:
//   - httptest.NewServer / httptest.NewRecorder for testing HTTP handlers
//   - Table-driven tests (a slice of test cases run in a loop)
//   - Subtests with t.Run() for organized test output
//   - JSON response parsing and assertion
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/services"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupTestRouter creates a fresh router with all endpoints registered and
// the store reset. This is called before each test to ensure isolation.
//
// It returns the HTTP handler (mux), which can be used with httptest to
// simulate HTTP requests without starting a real server.
func setupTestRouter(t *testing.T) http.Handler {
	t.Helper()

	// Reset the store to ensure a clean slate.
	s := store.GetStore()
	s.Reset()

	// Wire up dependencies — same as in main.go.
	feedService := services.NewFeedService(s)
	swipeService := services.NewSwipeService(s)

	userHandler := NewUserHandler(s)
	feedHandler := NewFeedHandler(feedService)
	swipeHandler := NewSwipeHandler(swipeService, s)

	// Create a new mux with all routes registered.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", HealthCheck)
	mux.HandleFunc("POST /users/", userHandler.CreateUser)
	mux.HandleFunc("GET /users/{id}", userHandler.GetUser)
	mux.HandleFunc("GET /feed", feedHandler.GetFeed)
	mux.HandleFunc("POST /swipe", swipeHandler.CreateSwipe)
	mux.HandleFunc("GET /matches", swipeHandler.GetMatches)

	return mux
}

// doRequest is a helper that sends an HTTP request to the test router and
// returns the response recorder. It handles JSON body encoding for POST requests.
func doRequest(t *testing.T, mux http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		// Marshal the body to JSON. In tests, we use bytes.Buffer as an
		// in-memory io.Reader that the HTTP request reads from.
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	} else {
		reqBody = &bytes.Buffer{}
	}

	// Create a new HTTP request.
	// httptest.NewRequest creates a request suitable for testing — it doesn't
	// actually make a network call.
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	// httptest.NewRecorder captures the response written by the handler.
	// It implements http.ResponseWriter so the handler writes to it normally.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	return rr
}

// parseResponse is a helper that decodes a JSON API response envelope.
func parseResponse(t *testing.T, rr *httptest.ResponseRecorder) models.APIResponse {
	t.Helper()

	var resp models.APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nbody: %s", err, rr.Body.String())
	}
	return resp
}

// createTestUser is a helper that creates a user via the API and returns
// the parsed user data (as a map) along with the user's UUID.
func createTestUser(t *testing.T, mux http.Handler, name, gender, zone string, age int) (uuid.UUID, map[string]interface{}) {
	t.Helper()

	body := models.CreateUserRequest{
		Name:   name,
		Age:    age,
		Gender: gender,
		ZoneID: zone,
	}

	rr := doRequest(t, mux, "POST", "/users/", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user failed: status %d, body: %s", rr.Code, rr.Body.String())
	}

	resp := parseResponse(t, rr)
	userData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected user data in response")
	}

	userID, err := uuid.Parse(userData["id"].(string))
	if err != nil {
		t.Fatalf("invalid user ID in response: %v", err)
	}

	return userID, userData
}

// ---------------------------------------------------------------------------
// Health check tests
// ---------------------------------------------------------------------------

func TestHealthCheck(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", "/", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := parseResponse(t, rr)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be an object")
	}
	if data["status"] != "healthy" {
		t.Errorf("status: got %v, want healthy", data["status"])
	}
}

// ---------------------------------------------------------------------------
// User endpoint tests
// ---------------------------------------------------------------------------

func TestCreateUser_Success(t *testing.T) {
	mux := setupTestRouter(t)

	body := models.CreateUserRequest{
		Name:   "Alice",
		Age:    28,
		Gender: "female",
		ZoneID: "zone-a",
	}

	rr := doRequest(t, mux, "POST", "/users/", body)

	// Verify HTTP 201 Created.
	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusCreated)
	}

	// Verify the response contains the user data with a generated UUID.
	resp := parseResponse(t, rr)
	userData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected user data in response")
	}

	if userData["name"] != "Alice" {
		t.Errorf("name: got %v, want Alice", userData["name"])
	}
	if userData["zone_id"] != "zone-a" {
		t.Errorf("zone_id: got %v, want zone-a", userData["zone_id"])
	}

	// Verify the ID is a valid UUID.
	if _, err := uuid.Parse(userData["id"].(string)); err != nil {
		t.Errorf("expected valid UUID, got: %v", userData["id"])
	}
}

func TestCreateUser_ValidationErrors(t *testing.T) {
	mux := setupTestRouter(t)

	// Table-driven test: each case tests a different validation failure.
	// This is a very common Go testing pattern — define a slice of test
	// cases and loop over them. Each case runs as a subtest.
	tests := []struct {
		name string
		body models.CreateUserRequest
	}{
		{
			name: "missing name",
			body: models.CreateUserRequest{Age: 25, Gender: "male", ZoneID: "zone-a"},
		},
		{
			name: "invalid age",
			body: models.CreateUserRequest{Name: "Bob", Age: 0, Gender: "male", ZoneID: "zone-a"},
		},
		{
			name: "missing gender",
			body: models.CreateUserRequest{Name: "Bob", Age: 25, ZoneID: "zone-a"},
		},
		{
			name: "missing zone_id",
			body: models.CreateUserRequest{Name: "Bob", Age: 25, Gender: "male"},
		},
	}

	for _, tc := range tests {
		// t.Run creates a subtest with the given name. If this subtest fails,
		// the output clearly identifies which test case caused the failure.
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(t, mux, "POST", "/users/", tc.body)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	mux := setupTestRouter(t)

	// Send a raw string that isn't valid JSON.
	req := httptest.NewRequest("POST", "/users/", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestGetUser_Success(t *testing.T) {
	mux := setupTestRouter(t)

	// First, create a user.
	userID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)

	// Now retrieve them by ID.
	rr := doRequest(t, mux, "GET", fmt.Sprintf("/users/%s", userID), nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := parseResponse(t, rr)
	userData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected user data")
	}
	if userData["name"] != "Alice" {
		t.Errorf("name: got %v, want Alice", userData["name"])
	}
}

func TestGetUser_NotFound(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", fmt.Sprintf("/users/%s", uuid.New()), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetUser_InvalidUUID(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", "/users/not-a-uuid", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Feed endpoint tests
// ---------------------------------------------------------------------------

func TestGetFeed_Success(t *testing.T) {
	mux := setupTestRouter(t)

	// Create users: Alice and Bob in zone-a, Charlie in zone-b.
	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	createTestUser(t, mux, "Bob", "male", "zone-a", 30)
	createTestUser(t, mux, "Charlie", "male", "zone-b", 25)

	// Get Alice's feed.
	rr := doRequest(t, mux, "GET", fmt.Sprintf("/feed?user_id=%s", aliceID), nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := parseResponse(t, rr)

	// Data should be an array with 1 user (Bob, same zone).
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 user in feed, got %d", len(data))
	}

	// Meta should include the count.
	if count, ok := resp.Meta["count"].(float64); !ok || int(count) != 1 {
		t.Errorf("expected meta.count=1, got %v", resp.Meta["count"])
	}
}

func TestGetFeed_UserNotFound(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", fmt.Sprintf("/feed?user_id=%s", uuid.New()), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetFeed_MissingUserID(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", "/feed", nil)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestGetFeed_InvalidUUID(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", "/feed?user_id=not-a-uuid", nil)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestGetFeed_ExcludesSwipedUsers(t *testing.T) {
	mux := setupTestRouter(t)

	// Create three users in the same zone.
	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)
	createTestUser(t, mux, "Charlie", "male", "zone-a", 25)

	// Alice swipes on Bob.
	swipeBody := models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: bobID.String(),
		Action:   "LIKE",
	}
	doRequest(t, mux, "POST", "/swipe", swipeBody)

	// Alice's feed should only have Charlie (Bob was swiped on).
	rr := doRequest(t, mux, "GET", fmt.Sprintf("/feed?user_id=%s", aliceID), nil)
	resp := parseResponse(t, rr)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 user in feed, got %d", len(data))
	}

	// The remaining user should be Charlie.
	user := data[0].(map[string]interface{})
	if user["name"] != "Charlie" {
		t.Errorf("expected Charlie in feed, got %v", user["name"])
	}
}

// ---------------------------------------------------------------------------
// Swipe endpoint tests
// ---------------------------------------------------------------------------

func TestCreateSwipe_Success(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)

	body := models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: bobID.String(),
		Action:   "LIKE",
	}

	rr := doRequest(t, mux, "POST", "/swipe", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusCreated)
	}

	resp := parseResponse(t, rr)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be an object")
	}
	if data["matched"] != false {
		t.Error("expected matched=false for one-sided LIKE")
	}
}

func TestCreateSwipe_MutualMatch(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)

	// Alice likes Bob.
	doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: bobID.String(),
		Action:   "LIKE",
	})

	// Bob likes Alice — should trigger a match.
	rr := doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: bobID.String(),
		SwipedID: aliceID.String(),
		Action:   "LIKE",
	})

	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusCreated)
	}

	resp := parseResponse(t, rr)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be an object")
	}
	if data["matched"] != true {
		t.Error("expected matched=true for mutual LIKE")
	}
	if data["match"] == nil {
		t.Error("expected match details in response")
	}
}

func TestCreateSwipe_SelfSwipe(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)

	body := models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: aliceID.String(),
		Action:   "LIKE",
	}

	rr := doRequest(t, mux, "POST", "/swipe", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateSwipe_NonexistentSwiper(t *testing.T) {
	mux := setupTestRouter(t)

	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)

	body := models.CreateSwipeRequest{
		SwiperID: uuid.New().String(),
		SwipedID: bobID.String(),
		Action:   "LIKE",
	}

	rr := doRequest(t, mux, "POST", "/swipe", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCreateSwipe_NonexistentSwiped(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)

	body := models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: uuid.New().String(),
		Action:   "LIKE",
	}

	rr := doRequest(t, mux, "POST", "/swipe", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCreateSwipe_InvalidAction(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)

	body := models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: bobID.String(),
		Action:   "INVALID",
	}

	rr := doRequest(t, mux, "POST", "/swipe", body)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestCreateSwipe_InvalidJSON(t *testing.T) {
	mux := setupTestRouter(t)

	req := httptest.NewRequest("POST", "/swipe", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestCreateSwipe_ValidationErrors(t *testing.T) {
	mux := setupTestRouter(t)

	// Invalid UUIDs should return 422.
	body := models.CreateSwipeRequest{
		SwiperID: "bad-uuid",
		SwipedID: "also-bad",
		Action:   "LIKE",
	}

	rr := doRequest(t, mux, "POST", "/swipe", body)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

// ---------------------------------------------------------------------------
// Matches endpoint tests
// ---------------------------------------------------------------------------

func TestGetMatches_Success(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)

	// Create a mutual match.
	doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: bobID.String(),
		Action:   "LIKE",
	})
	doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: bobID.String(),
		SwipedID: aliceID.String(),
		Action:   "LIKE",
	})

	// Check Alice's matches.
	rr := doRequest(t, mux, "GET", fmt.Sprintf("/matches?user_id=%s", aliceID), nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := parseResponse(t, rr)
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 match, got %d", len(data))
	}

	// Verify meta count.
	if count, ok := resp.Meta["count"].(float64); !ok || int(count) != 1 {
		t.Errorf("expected meta.count=1, got %v", resp.Meta["count"])
	}
}

func TestGetMatches_NoMatches(t *testing.T) {
	mux := setupTestRouter(t)

	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)

	rr := doRequest(t, mux, "GET", fmt.Sprintf("/matches?user_id=%s", aliceID), nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := parseResponse(t, rr)
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) != 0 {
		t.Errorf("expected 0 matches, got %d", len(data))
	}
}

func TestGetMatches_UserNotFound(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", fmt.Sprintf("/matches?user_id=%s", uuid.New()), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetMatches_MissingUserID(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", "/matches", nil)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestGetMatches_InvalidUUID(t *testing.T) {
	mux := setupTestRouter(t)

	rr := doRequest(t, mux, "GET", "/matches?user_id=not-a-uuid", nil)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

// ---------------------------------------------------------------------------
// Full flow integration test
// ---------------------------------------------------------------------------

func TestFullFlow_CreateSwipeMatch(t *testing.T) {
	mux := setupTestRouter(t)

	// 1. Create users in two zones.
	aliceID, _ := createTestUser(t, mux, "Alice", "female", "zone-a", 28)
	bobID, _ := createTestUser(t, mux, "Bob", "male", "zone-a", 30)
	charlieID, _ := createTestUser(t, mux, "Charlie", "male", "zone-a", 25)
	createTestUser(t, mux, "Diana", "female", "zone-b", 22)

	// 2. Check Alice's feed — should see Bob and Charlie (same zone).
	rr := doRequest(t, mux, "GET", fmt.Sprintf("/feed?user_id=%s", aliceID), nil)
	resp := parseResponse(t, rr)
	feedData := resp.Data.([]interface{})
	if len(feedData) != 2 {
		t.Fatalf("expected 2 users in Alice's feed, got %d", len(feedData))
	}

	// 3. Alice likes Bob.
	rr = doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: bobID.String(),
		Action:   "LIKE",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("swipe failed: %d", rr.Code)
	}

	// 4. Alice's feed should now only show Charlie.
	rr = doRequest(t, mux, "GET", fmt.Sprintf("/feed?user_id=%s", aliceID), nil)
	resp = parseResponse(t, rr)
	feedData = resp.Data.([]interface{})
	if len(feedData) != 1 {
		t.Fatalf("expected 1 user in Alice's feed after swipe, got %d", len(feedData))
	}

	// 5. Alice passes on Charlie.
	doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: aliceID.String(),
		SwipedID: charlieID.String(),
		Action:   "PASS",
	})

	// 6. Alice's feed should now be empty.
	rr = doRequest(t, mux, "GET", fmt.Sprintf("/feed?user_id=%s", aliceID), nil)
	resp = parseResponse(t, rr)
	feedData = resp.Data.([]interface{})
	if len(feedData) != 0 {
		t.Fatalf("expected empty feed after swiping all, got %d", len(feedData))
	}

	// 7. No matches yet (one-sided likes).
	rr = doRequest(t, mux, "GET", fmt.Sprintf("/matches?user_id=%s", aliceID), nil)
	resp = parseResponse(t, rr)
	matchData := resp.Data.([]interface{})
	if len(matchData) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matchData))
	}

	// 8. Bob likes Alice — creates a mutual match!
	rr = doRequest(t, mux, "POST", "/swipe", models.CreateSwipeRequest{
		SwiperID: bobID.String(),
		SwipedID: aliceID.String(),
		Action:   "LIKE",
	})
	resp = parseResponse(t, rr)
	swipeData := resp.Data.(map[string]interface{})
	if swipeData["matched"] != true {
		t.Error("expected mutual match")
	}

	// 9. Both Alice and Bob should now have 1 match.
	rr = doRequest(t, mux, "GET", fmt.Sprintf("/matches?user_id=%s", aliceID), nil)
	resp = parseResponse(t, rr)
	matchData = resp.Data.([]interface{})
	if len(matchData) != 1 {
		t.Errorf("expected 1 match for Alice, got %d", len(matchData))
	}

	rr = doRequest(t, mux, "GET", fmt.Sprintf("/matches?user_id=%s", bobID), nil)
	resp = parseResponse(t, rr)
	matchData = resp.Data.([]interface{})
	if len(matchData) != 1 {
		t.Errorf("expected 1 match for Bob, got %d", len(matchData))
	}
}

// ---------------------------------------------------------------------------
// Response envelope tests
// ---------------------------------------------------------------------------

func TestResponseEnvelope_AlwaysHasRequiredFields(t *testing.T) {
	mux := setupTestRouter(t)

	// Every response should have "data", "meta", and "errors" fields.
	// Test this across different endpoints and status codes.

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"health check", "GET", "/"},
		{"user not found", "GET", fmt.Sprintf("/users/%s", uuid.New())},
		{"feed missing param", "GET", "/feed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(t, mux, tc.method, tc.path, nil)

			// Parse the raw JSON to check structure.
			var raw map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}

			// Check that all three envelope fields exist.
			for _, field := range []string{"data", "meta", "errors"} {
				if _, exists := raw[field]; !exists {
					t.Errorf("response missing required field %q", field)
				}
			}
		})
	}
}
