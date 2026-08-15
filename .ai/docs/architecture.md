# Expense Tracker API — Architecture

## 1. Project Structure

```
expense-tracker/
├── main.go                  # Entry point: wires dependencies, starts server
├── go.mod
├── go.sum
├── README.md
│
├── internal/
│   ├── handler/
│   │   └── expense.go       # HTTP handlers for expense endpoints
│   ├── model/
│   │   └── expense.go       # Expense struct, request/response types, validation
│   └── store/
│       └── expense.go       # Data access layer (SQLite), sentinel errors
│
├── migrations/
│   └── 001_create_expenses.sql  # Embedded at build time via go:embed
│
└── tests/
    └── api_test.go           # End-to-end API tests
```

### Package Responsibilities

| Package | Responsibility |
|---------|---------------|
| `main` | Wire dependencies, register routes on `http.ServeMux`, embed and apply migrations, start HTTP server |
| `internal/handler` | Decode HTTP requests, enforce Content-Type, call model validation, call store, write HTTP responses |
| `internal/model` | Define data structures (Expense, request types) and all validation logic |
| `internal/store` | All database interaction (CRUD queries), sentinel errors |

Route registration (~10 lines) lives in `main.go`. A separate `router` package is unnecessary at this scale.

---

## 2. Go Architecture

### Dependency Flow

```
main → handler → store
       handler → model
       store   → model
```

- **No interfaces for now.** The handler takes a concrete `*store.ExpenseStore`. There is one implementation (SQLite). Because the handler accepts the store as a struct field (not a package-level global), a consumer-side interface can be extracted later by defining an interface in the handler package that `*store.ExpenseStore` already satisfies — no refactoring of the store is needed.
- **No dependency injection framework.** Dependencies are wired manually in `main.go`.
- **Standard library HTTP.** Use `net/http` with Go 1.22+ routing patterns (`"GET /expenses/{id}"`). No third-party router.

### Store

```go
package store

import "errors"

var ErrNotFound = errors.New("expense not found")

type ExpenseStore struct {
    db *sql.DB
}

func NewExpenseStore(db *sql.DB) *ExpenseStore
func (s *ExpenseStore) Create(expense model.Expense) (model.Expense, error)
func (s *ExpenseStore) List() ([]model.Expense, error)
func (s *ExpenseStore) GetByID(id string) (model.Expense, error)
func (s *ExpenseStore) Update(id string, expense model.Expense) (model.Expense, error)
func (s *ExpenseStore) Delete(id string) error
```

- `GetByID` returns `ErrNotFound` when `sql.ErrNoRows` is encountered.
- `Update` and `Delete` check the affected row count; if zero, return `ErrNotFound`.
- All other database errors are returned as-is (the handler maps them to 500).

### Handler

```go
type ExpenseHandler struct {
    store *store.ExpenseStore
}

func NewExpenseHandler(store *store.ExpenseStore) *ExpenseHandler
func (h *ExpenseHandler) Create(w http.ResponseWriter, r *http.Request)
func (h *ExpenseHandler) List(w http.ResponseWriter, r *http.Request)
func (h *ExpenseHandler) Get(w http.ResponseWriter, r *http.Request)
func (h *ExpenseHandler) Update(w http.ResponseWriter, r *http.Request)
func (h *ExpenseHandler) Delete(w http.ResponseWriter, r *http.Request)
```

Handler error-mapping logic:

```go
if errors.Is(err, store.ErrNotFound) {
    writeError(w, http.StatusNotFound, "expense not found")
    return
}
writeError(w, http.StatusInternalServerError, "internal error")
```

---

## 3. REST API Endpoints

| Method | Path | Description | Success Code |
|--------|------|-------------|-------------|
| `POST` | `/expenses` | Create an expense | `201 Created` |
| `GET` | `/expenses` | List all expenses | `200 OK` |
| `GET` | `/expenses/{id}` | Get a single expense | `200 OK` |
| `PUT` | `/expenses/{id}` | Full update of an expense | `200 OK` |
| `DELETE` | `/expenses/{id}` | Delete an expense | `204 No Content` |

All endpoints that accept a request body (`POST`, `PUT`) require `Content-Type: application/json`. Requests with a different Content-Type receive `415 Unsupported Media Type`.

All endpoints that return data produce `application/json`. `DELETE` returns no body.

### PUT Semantics

`PUT` performs a **full replacement** of the expense resource (per RFC 9110). All fields (`amount`, `description`, `category`, `date`) are required in the request body. Missing or empty fields are a `400 Bad Request` validation error — identical to the `POST` validation rules.

---

## 4. Request/Response Models

### Expense (stored/returned)

The `amount` field is represented as an **integer in cents** to avoid floating-point rounding errors.

```json
{
  "id": "uuid-string",
  "amount": 4999,
  "description": "Grocery shopping",
  "category": "food",
  "date": "2026-08-15"
}
```

### Go Structs

```go
// Expense is the stored representation, used in API responses.
type Expense struct {
    ID          string `json:"id"`
    Amount      int64  `json:"amount"`
    Description string `json:"description"`
    Category    string `json:"category"`
    Date        string `json:"date"`
}

// CreateExpenseRequest is the expected body for POST /expenses.
type CreateExpenseRequest struct {
    Amount      int64  `json:"amount"`
    Description string `json:"description"`
    Category    string `json:"category"`
    Date        string `json:"date"`
}

// UpdateExpenseRequest is the expected body for PUT /expenses/{id}.
type UpdateExpenseRequest struct {
    Amount      int64  `json:"amount"`
    Description string `json:"description"`
    Category    string `json:"category"`
    Date        string `json:"date"`
}
```

Request structs have no `id` field. Combined with `DisallowUnknownFields()`, a client sending `id` in the body receives an immediate `400 Bad Request`.

### JSON Decoding

All request body decoding uses:

```go
decoder := json.NewDecoder(r.Body)
decoder.DisallowUnknownFields()
err := decoder.Decode(&req)
```

Unknown fields in the request body result in `400 Bad Request`.

### List Response

```json
[
  { "id": "...", "amount": 4999, "description": "...", "category": "...", "date": "..." }
]
```

Returns an empty array `[]` (not `null`) when no expenses exist.

Results are ordered by `date DESC, id ASC` (most recent expenses first, stable tie-breaking by ID).

---

## 5. Database Schema

**Engine:** SQLite (via `modernc.org/sqlite` — pure Go, no CGO).

```sql
CREATE TABLE IF NOT EXISTS expenses (
    id          TEXT PRIMARY KEY,
    amount      INTEGER NOT NULL,
    description TEXT NOT NULL,
    category    TEXT NOT NULL,
    date        TEXT NOT NULL
);
```

- `id` — UUID v4, generated by the application using `google/uuid`.
- `amount` — stored as `INTEGER` (cents). `4999` represents `$49.99`.
- `date` — stored as `TEXT` in `YYYY-MM-DD` format. Validated at the application layer.

### Migrations

Migration SQL is embedded into the binary using Go's `//go:embed` directive:

```go
import "embed"

//go:embed migrations/*.sql
var migrationFS embed.FS
```

At startup, `main.go` reads embedded migration files, sorts them by filename, and executes them. `CREATE TABLE IF NOT EXISTS` makes the single migration idempotent. This approach makes the binary self-contained with no filesystem dependency at runtime.

---

## 6. Error Handling

### Error Response Format

All errors return a consistent JSON body:

```json
{
  "error": "human-readable error message"
}
```

### HTTP Status Codes

| Scenario | Status Code |
|----------|-------------|
| Wrong Content-Type on POST/PUT | `415 Unsupported Media Type` |
| Malformed JSON body | `400 Bad Request` |
| Unknown fields in JSON body | `400 Bad Request` |
| Validation failure | `400 Bad Request` |
| Expense not found | `404 Not Found` |
| Wrong HTTP method (handled by `net/http`) | `405 Method Not Allowed` |
| Internal/database error | `500 Internal Server Error` |

### Implementation

A small helper function in the handler package:

```go
func writeError(w http.ResponseWriter, status int, message string)
func writeJSON(w http.ResponseWriter, status int, data any)
```

The handler distinguishes "not found" from "internal error" using `errors.Is(err, store.ErrNotFound)`. Internal error details are logged server-side via `log.Printf` but not exposed to the client.

---

## 7. Validation

Validation logic lives in the `model` package. The handler calls validation after decoding the request body; if validation fails, the handler returns `400 Bad Request`.

**Contract: all callers of the store must pass validated data.**

```go
func (r CreateExpenseRequest) Validate() error
func (r UpdateExpenseRequest) Validate() error
```

Both methods apply the same rules:

| Field | Rules |
|-------|-------|
| `amount` | Required. Must be a positive integer (> 0). |
| `description` | Required. Must be a non-empty string after trimming whitespace. |
| `category` | Required. Must be a non-empty string after trimming whitespace. |
| `date` | Required. Must parse successfully with `time.Parse("2006-01-02", date)`. This rejects invalid formats, non-existent dates (e.g., `2026-02-30`), and non-date strings. No additional range restrictions (future dates are allowed). |

Returns the first validation error encountered. No third-party validation library.

---

## 8. Testing Strategy

### Approach: End-to-End API Tests

Tests exercise the full stack (HTTP → handler → store → SQLite) by:

1. Creating a temporary SQLite database file via `os.CreateTemp` (avoids the `:memory:` connection pool isolation bug where each pooled connection gets its own in-memory database).
2. Using `net/http/httptest.NewServer` to create a test server.
3. Making real HTTP requests against the test server.
4. Asserting on HTTP status codes and JSON response bodies.
5. Cleaning up the temp file with `t.Cleanup`.

Each test function gets a fresh database and server instance for full isolation.

### Test Cases

| Test | Description |
|------|-------------|
| **CRUD happy path** | |
| Create expense | POST valid expense → 201, response has generated `id`, correct data |
| List expenses | GET returns all created expenses, ordered by date DESC |
| List expenses — empty | GET returns `[]` when none exist |
| Get expense | GET by id → 200, correct data |
| Update expense | PUT with all fields changed → 200, data fully replaced |
| Delete expense | DELETE → 204, subsequent GET → 404 |
| **Not found** | |
| Get expense — not found | GET non-existent id → 404 |
| Update expense — not found | PUT non-existent id → 404 |
| Delete expense — not found | DELETE non-existent id → 404 |
| **Validation — amount** | |
| Create with amount = 0 | POST → 400 (boundary: must be > 0) |
| Create with amount = -1 | POST → 400 (negative) |
| Create with missing amount | POST with amount omitted → 400 |
| **Validation — date** | |
| Create with invalid date format | POST `date: "not-a-date"` → 400 |
| Create with non-existent date | POST `date: "2026-02-30"` → 400 |
| Create with missing date | POST with date omitted → 400 |
| **Validation — strings** | |
| Create with empty description | POST `description: ""` → 400 |
| Create with empty category | POST `category: ""` → 400 |
| **Request handling** | |
| Unknown JSON fields | POST with extra field → 400 |
| Wrong Content-Type | POST with `text/plain` → 415 |
| Update — full replacement | PUT with only some fields → 400 (validates all fields required) |

### Running Tests

```bash
go test ./...
```

No external test dependencies. Uses only the standard `testing` package.

---

## 9. Architectural Decisions

| Decision | Rationale |
|----------|-----------|
| **Standard library `net/http`** | Go 1.22+ has method+path routing built in. No need for `chi`, `gin`, or `mux`. |
| **SQLite** | Simplest possible persistence. No external database process. Single file. Perfect for a personal expense tracker. |
| **`modernc.org/sqlite`** | Pure Go SQLite driver. No CGO, no C compiler dependency. Simplifies builds and CI. |
| **No interfaces** | Only one storage backend exists. The handler takes a concrete `*store.ExpenseStore`. Because it is a struct field (not a global), a consumer-side interface can be introduced later in the handler package without modifying the store. |
| **No service/usecase layer** | Business logic is minimal (validation + CRUD). An extra layer would just proxy calls. Can be introduced when business rules grow. |
| **UUID for IDs** | Avoids exposing auto-increment internals. Safe for future API consumers. |
| **Integer cents for amount** | `int64` cents avoids IEEE 754 floating-point rounding errors. `4999` = `$49.99`. No precision loss, no extra dependencies. |
| **Separate request structs** | `CreateExpenseRequest` and `UpdateExpenseRequest` exclude `id`, making it structurally impossible for clients to set IDs. Combined with `DisallowUnknownFields()`, sending `id` in the body is a 400. |
| **Embedded migrations** | `go:embed` makes the binary self-contained. No runtime filesystem dependency. |
| **Temp-file SQLite for tests** | Avoids `:memory:` connection pool isolation bug. Each test gets a fresh temp file, cleaned up via `t.Cleanup`. |
| **True PUT (full replacement)** | All fields required on update. Simpler than PATCH partial-update semantics. Matches RFC 9110. |
| **End-to-end tests over unit tests** | With minimal business logic, E2E tests provide the highest confidence-to-effort ratio. They test the real code path users will hit. |
| **No authentication** | Per requirements. Will be added later. |
| **No frontend** | Per requirements. API-only for now. |

---

## 10. Future Considerations

The following are explicitly out of scope for v1 but are noted here to inform future design:

- **Pagination.** `GET /expenses` currently returns all records. When needed, add `?limit=` and `?offset=` query parameters. This is additive and non-breaking for existing clients.
- **Structured logging.** Go 1.21+ provides `log/slog`. When observability needs grow, add structured logging with request correlation IDs. For now, `log.Printf` is sufficient.

---

## Requirements Coverage Checklist

| Requirement | Covered By |
|-------------|-----------|
| Create an expense | `POST /expenses` |
| List expenses | `GET /expenses` (ordered by date DESC, id ASC) |
| Get a single expense | `GET /expenses/{id}` |
| Update an expense | `PUT /expenses/{id}` (full replacement) |
| Delete an expense | `DELETE /expenses/{id}` |
| Fields: id, amount, description, category, date | Model structs + DB schema (amount as integer cents) |
| Written in Go | All code is Go, stdlib `net/http` |
| Automated tests | E2E tests via `httptest` with temp-file SQLite |
| No authentication | Not implemented |
| No frontend/mobile | Not implemented |
