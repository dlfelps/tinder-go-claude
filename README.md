# Tinder-Claude (Go)

A backend REST API prototype implementing core dating-app mechanics, ported from FastAPI/Python to idiomatic Go. This project serves as an educational reference for studying matching systems, geo-spatial filtering logic, and Go web development patterns.

## Features

- **Profile creation** with UUID-based identity
- **Location-based discovery feeds** with three-tier filtering (zone, self-exclusion, seen-state)
- **Swiping interactions** (LIKE / PASS)
- **Mutual match detection** on bidirectional LIKEs
- **Standardized API response envelope** (`data`, `meta`, `errors`)

## Project Structure

```
tinder-go-claude/
├── cmd/server/
│   └── main.go                        # Entry point, router setup, dependency wiring
├── internal/
│   ├── models/
│   │   └── models.go                  # Domain types, request/response structs, enums
│   ├── store/
│   │   ├── store.go                   # In-memory data store (singleton)
│   │   └── store_test.go              # Store unit tests
│   ├── services/
│   │   ├── feed_service.go            # Feed generation with 3-tier filter pipeline
│   │   ├── feed_service_test.go       # Feed service unit tests
│   │   ├── swipe_service.go           # Swipe processing & match detection
│   │   └── swipe_service_test.go      # Swipe service unit tests
│   └── handlers/
│       ├── helpers.go                 # Shared JSON response helpers
│       ├── health.go                  # GET / health check
│       ├── users.go                   # POST /users/, GET /users/{id}
│       ├── feed.go                    # GET /feed
│       ├── swipe.go                   # POST /swipe, GET /matches
│       └── handlers_test.go           # Integration tests (35+ scenarios)
├── go.mod
├── go.sum
└── design_document.docx               # Original design specification
```

## Getting Started

### Prerequisites

- Go 1.22 or later (uses the enhanced `ServeMux` with method-based routing)

### Run the Server

```bash
go run ./cmd/server/
```

The server starts on `http://localhost:8000` by default. Set the `PORT` environment variable to use a different port:

```bash
PORT=3000 go run ./cmd/server/
```

### Run Tests

```bash
# Run all tests
go test ./...

# Verbose output
go test -v ./...

# Run a specific package's tests
go test -v ./internal/store/
go test -v ./internal/services/
go test -v ./internal/handlers/
```

## API Endpoints

All responses use a standard envelope:

```json
{
  "data": "<payload>",
  "meta": {"count": 0},
  "errors": []
}
```

| Method | Endpoint            | Description                  | Status Codes     |
|--------|---------------------|------------------------------|------------------|
| GET    | `/`                 | Health check                 | 200              |
| POST   | `/users/`           | Create a new user profile    | 201, 422         |
| GET    | `/users/{id}`       | Retrieve user by UUID        | 200, 404         |
| GET    | `/feed?user_id=`    | Get filtered discovery feed  | 200, 404, 422    |
| POST   | `/swipe`            | Submit a swipe action        | 201, 400, 404, 422 |
| GET    | `/matches?user_id=` | List matches for a user      | 200, 404, 422    |

### Example Usage

```bash
# Create users
curl -X POST http://localhost:8000/users/ \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "age": 28, "gender": "female", "zone_id": "downtown"}'

curl -X POST http://localhost:8000/users/ \
  -H "Content-Type: application/json" \
  -d '{"name": "Bob", "age": 30, "gender": "male", "zone_id": "downtown"}'

# Get Alice's feed (replace <alice-uuid> with the ID from the create response)
curl http://localhost:8000/feed?user_id=<alice-uuid>

# Alice likes Bob
curl -X POST http://localhost:8000/swipe \
  -H "Content-Type: application/json" \
  -d '{"swiper_id": "<alice-uuid>", "swiped_id": "<bob-uuid>", "action": "LIKE"}'

# Bob likes Alice (creates a match!)
curl -X POST http://localhost:8000/swipe \
  -H "Content-Type: application/json" \
  -d '{"swiper_id": "<bob-uuid>", "swiped_id": "<alice-uuid>", "action": "LIKE"}'

# Check matches
curl http://localhost:8000/matches?user_id=<alice-uuid>
```

## Go Concepts Demonstrated

This codebase is designed to be educational for Go learners. Key concepts include:

- **Structs and methods** for object-oriented design without classes
- **Interfaces** and duck typing (`error` interface, custom error types)
- **Struct tags** for JSON serialization control
- **Goroutine safety** with `sync.Mutex`
- **The comma-ok idiom** for map lookups and optional values
- **Table-driven tests** with `t.Run()` subtests
- **`httptest`** for integration testing without a running server
- **Dependency injection** via constructor functions
- **Custom error types** with `errors.As()` for type-safe error handling
- **Go 1.22+ ServeMux** with method-based routing and path parameters
