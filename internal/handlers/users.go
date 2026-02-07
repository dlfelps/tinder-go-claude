// This file contains HTTP handlers for user-related endpoints:
//   - POST /users/   — Create a new user profile
//   - GET  /users/{id} — Retrieve a user by their UUID
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dlfelps/tinder-go-claude/internal/models"
	"github.com/dlfelps/tinder-go-claude/internal/store"
	"github.com/google/uuid"
)

// UserHandler groups all user-related HTTP handlers together.
// In Go, we organize related handlers into a struct so they can share
// dependencies (like the store). This is the Go equivalent of a Python class
// with dependency injection.
type UserHandler struct {
	store *store.InMemoryStore
}

// NewUserHandler creates a new UserHandler with the given store.
func NewUserHandler(s *store.InMemoryStore) *UserHandler {
	return &UserHandler{store: s}
}

// CreateUser handles POST /users/ — creates a new user profile.
//
// In FastAPI, you'd write:
//
//	@router.post("/users/", status_code=201)
//	async def create_user(request: CreateUserRequest): ...
//
// In Go, we manually parse the request body, validate it, and write the
// response. There's more boilerplate, but you have full control over every
// step of the process.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Step 1: Decode the JSON request body into our request struct.
	// json.NewDecoder reads from r.Body (an io.Reader) and parses JSON.
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If the request body isn't valid JSON, return a 422 error.
		// This mirrors FastAPI's automatic validation error response.
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON in request body")
		return
	}

	// Step 2: Validate the request fields.
	// In FastAPI + Pydantic, validation happens automatically. In Go, we
	// call our explicit validation method.
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusUnprocessableEntity, errs...)
		return
	}

	// Step 3: Create the domain model with a generated UUID.
	// uuid.New() generates a random UUID v4, similar to Python's uuid.uuid4().
	user := models.User{
		ID:     uuid.New(),
		Name:   req.Name,
		Age:    req.Age,
		Gender: req.Gender,
		ZoneID: req.ZoneID,
	}

	// Step 4: Persist the user in the store.
	h.store.AddUser(user)

	// Step 5: Return the created user with HTTP 201 Created.
	writeSuccess(w, http.StatusCreated, user, nil)
}

// GetUser handles GET /users/{id} — retrieves a user by their UUID.
//
// Go 1.22+ introduced path parameters in the standard library's ServeMux.
// We extract the {id} parameter using r.PathValue("id"), which is similar
// to FastAPI's path parameter injection.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract and parse the user ID from the URL path.
	// r.PathValue() is a Go 1.22+ feature that extracts named path segments.
	idStr := r.PathValue("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		// If the ID isn't a valid UUID, return 404.
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Step 2: Look up the user in the store.
	user, exists := h.store.GetUser(userID)
	if !exists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Step 3: Return the user data with HTTP 200 OK.
	writeSuccess(w, http.StatusOK, user, nil)
}
