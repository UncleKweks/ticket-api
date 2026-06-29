# ticket-api

A RESTful HTTP API for managing event tickets. Written in pure Go — no
third-party frameworks, no ORMs, no code generators.

```
ticket-api/
├── main.go              Wires the router, store, handlers, and middleware
├── models/
│   └── ticket.go        Ticket struct, Status enum, request/response types
├── handlers/
│   ├── ticket.go        HTTP handlers: List, Get, Create, Update, Delete, Reserve
│   └── middleware.go    Logger, Recovery, CORS, Chain helper
└── store/
    └── store.go         Thread-safe in-memory store (sync.RWMutex + atomic ID)
```

---

## How to Run

```bash
cd ticket-api
go run .
# 2026/06/26 10:00:00 INFO ticket-api listening addr=:8080
```

**Test it instantly:**

```bash
# List all tickets
curl http://localhost:8080/api/v1/tickets

# Get one ticket
curl http://localhost:8080/api/v1/tickets/1

# Reserve 2 seats
curl -X POST http://localhost:8080/api/v1/tickets/1/reserve \
     -H "Content-Type: application/json" \
     -d '{"seats": 2}'

# Create a ticket
curl -X POST http://localhost:8080/api/v1/tickets \
     -H "Content-Type: application/json" \
     -d '{"event":"DevFest Lagos","description":"Google DevFest","price":0,"total_seats":300}'
```

---

## What `main.go` Does

The original `main.go` was a stub — it defined its own `Ticket` struct and
had a single `/tickets` handler that ignored the entire `handlers/`,
`store/`, and `models/` packages that were already built.

The rewritten `main.go` does exactly three things:

1. **Creates the store** — `store.New()` initialises the in-memory map and
   seeds it with three events.
2. **Creates the handler** — `handlers.NewTicketHandler(s)` wraps the store
   and gives us the six HTTP methods as Go methods.
3. **Builds the router and middleware chain** — every request passes through
   Recovery → Logger → CORS before reaching a handler.

```go
handler := handlers.Chain(mux,
    handlers.Recovery,   // outermost: catches panics
    handlers.Logger,     // middle: logs every request
    handlers.CORS,       // innermost: adds CORS headers
)
```

---

## Architecture: Why Four Packages?

### `models` — pure data shapes

`models/ticket.go` contains only structs. No methods, no logic, no imports
except `time`. This makes the types importable anywhere without creating
circular imports.

```go
type Ticket struct {
    ID             int64     `json:"id"`
    Event          string    `json:"event"`
    AvailableSeats int       `json:"available_seats"`
    Status         Status    `json:"status"`
    // ...
}
```

### `store` — all state lives here

The store is the single source of truth. It owns the in-memory map, the mutex,
and the ID counter. Nothing outside this package touches the map directly.

This means you can swap the store for a PostgreSQL implementation later by
writing a new struct that has the same methods (`Create`, `GetAll`, `GetByID`,
`Update`, `Delete`, `Reserve`). The handlers would not need to change at all.

### `handlers` — HTTP translation layer

Handlers do exactly three things:
1. Parse and validate the HTTP request.
2. Call the store.
3. Write an HTTP response.

They contain zero business logic. If the store returns `ErrNoSeats`, the
handler translates that to HTTP 409. That's it.

### `middleware` — cross-cutting concerns

Middleware wraps handlers. It runs before *and* after every request, regardless
of which handler is matched. Common middleware tasks: logging, panic recovery,
authentication, rate limiting, CORS.

```
Request → Recovery → Logger → CORS → handler
Response ← Recovery ← Logger ← CORS ← handler
```

`Chain(mux, Recovery, Logger, CORS)` applies them in that order by wrapping
in reverse:

```go
// Pseudocode of what Chain() produces:
handler = Recovery(Logger(CORS(mux)))
```

---

## Concurrency in the Store

The store is called by many goroutines simultaneously — one per HTTP request.
Without protection, concurrent reads and writes to the same map cause a data
race (undefined behaviour, crashes, corrupted data).

### sync.RWMutex

```go
type Store struct {
    mu      sync.RWMutex
    tickets map[int64]*models.Ticket
    nextID  atomic.Int64
}
```

`sync.RWMutex` has two lock types:

| Lock | Method | Who can hold it |
|------|--------|-----------------|
| Read lock | `mu.RLock()` / `mu.RUnlock()` | Many goroutines simultaneously |
| Write lock | `mu.Lock()` / `mu.Unlock()` | One goroutine; blocks all others |

```go
// GetAll — read operation, uses shared lock
func (s *Store) GetAll(status models.Status) []models.Ticket {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // ... multiple goroutines can run this block at the same time
}

// Reserve — write operation, uses exclusive lock
func (s *Store) Reserve(id int64, seats int) (*models.Ticket, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // ... only one goroutine at a time; all others wait
}
```

### atomic.Int64 — lock-free ID counter

Incrementing a counter with a full mutex lock is wasteful. `atomic.Int64.Add`
uses a single CPU instruction (compare-and-swap) that is inherently safe across
goroutines:

```go
id := s.nextID.Add(1)   // returns the new value; no mutex needed
```

### clone() — defensive copies

The store never hands a caller a pointer into its internal map. It always
returns a copy:

```go
func clone(t *models.Ticket) *models.Ticket {
    c := *t   // dereference and copy the entire struct
    return &c
}
```

If the caller modified the returned pointer, they would modify the store's
internal data — a data race even under the mutex because the mutation happens
after `Unlock()`. Copies prevent this entirely.

---

## Middleware: How `statusWriter` Works

The `Logger` middleware needs to log the HTTP status code *after* the handler
runs. But `http.ResponseWriter` has no `StatusCode()` getter — once you call
`WriteHeader(200)`, you cannot ask what code was sent.

The solution: wrap the `ResponseWriter` in a struct that intercepts the call:

```go
type statusWriter struct {
    http.ResponseWriter       // embeds the real writer; all other methods pass through
    code int                  // we capture the status here
}

func (sw *statusWriter) WriteHeader(code int) {
    sw.code = code                        // save it
    sw.ResponseWriter.WriteHeader(code)   // pass it to the real writer
}
```

**Struct embedding** (`http.ResponseWriter` as an unnamed field) is Go's
composition mechanism. `statusWriter` automatically has all the methods of
`http.ResponseWriter`. We only override `WriteHeader` — all other calls (`Write`,
`Header`) go straight through to the underlying writer.

```go
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
        next.ServeHTTP(rw, r)    // handler writes into rw, not w
        slog.Info("request",
            "method",   r.Method,
            "path",     r.URL.Path,
            "status",   rw.code,    // ← captured above
            "duration", time.Since(start),
        )
    })
}
```

---

## API Reference

Base URL: `http://localhost:8080`

### Ticket shape

```json
{
  "id": 1,
  "event": "ETH Lagos",
  "description": "Ethereum developer conference in Lagos",
  "price": 25.00,
  "total_seats": 200,
  "available_seats": 198,
  "status": "open",
  "created_at": "2026-06-26T10:00:00Z",
  "updated_at": "2026-06-26T10:00:05Z"
}
```

**Status values:** `open` · `sold_out` · `closed`

### Endpoints

#### `GET /api/v1/tickets`

List all tickets. Optional query param: `?status=open`

#### `POST /api/v1/tickets`

```json
{ "event": "DevFest Lagos", "description": "...", "price": 0, "total_seats": 300 }
```

Response `201 Created` with the full ticket object.

#### `GET /api/v1/tickets/{id}`

Response `200` or `404`.

#### `PUT /api/v1/tickets/{id}`

Partial update. Only include fields you want to change:

```json
{ "price": 30.00 }
```

#### `DELETE /api/v1/tickets/{id}`

Response `204 No Content`.

#### `POST /api/v1/tickets/{id}/reserve`

```json
{ "seats": 2 }
```

Response `200` with updated ticket.
Response `409` if sold out or closed.
