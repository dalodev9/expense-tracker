# TASK-001-FIXES — Code Review

**Reviewer:** Independent Senior Go Code Reviewer  
**Review Date:** 2026-08-15  
**Task:** TASK-001-FIXES — Address Code Review Findings  
**Source of Truth:** `.ai/docs/architecture.md`, `.ai/docs/requirements.md`, `.ai/tasks/TASK-001.md`, `.ai/reviews/TASK-001-code-review.md`

---

## Files Inspected

| File | Lines | Bytes | Notes |
|------|-------|-------|-------|
| `main.go` | 58 | 1162 | Modified |
| `migrations/migrations.go` | 40 | 867 | **New file** |
| `migrations/001_create_expenses.sql` | 8 | 201 | Unchanged |
| `internal/handler/expense.go` | 200 | 5171 | Modified (`SetupRouter` moved here) |
| `internal/model/expense.go` | 72 | 2236 | Unchanged |
| `internal/model/expense_test.go` | 125 | 3494 | Modified |
| `internal/store/expense.go` | 101 | 2520 | Unchanged |
| `tests/api_test.go` | 685 | 17871 | Modified |
| `go.mod` | 20 | 505 | Unchanged |

**Command results (run independently):**

```
go test -count=1 ./...   → PASS (exit 0)
go vet ./...             → PASS (exit 0, no output)
go build ./...           → PASS (exit 0, no output)
```

---

## Verification Against TASK-001-FIXES Scope

### Fix 1 — Reuse production route registration (`SetupRouter`)

**Required:** Test setup must call `SetupRouter(h)` instead of manually registering routes.

**Verified:**

`SetupRouter` has been **moved** from `main.go` into `internal/handler/expense.go` (lines 22–32). It is now part of the `handler` package:

```go
// internal/handler/expense.go, lines 22–32
func SetupRouter(h *ExpenseHandler) *http.ServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", h.Health)
    mux.HandleFunc("POST /expenses", h.Create)
    mux.HandleFunc("GET /expenses", h.List)
    mux.HandleFunc("GET /expenses/{id}", h.Get)
    mux.HandleFunc("PUT /expenses/{id}", h.Update)
    mux.HandleFunc("DELETE /expenses/{id}", h.Delete)
    return mux
}
```

`main.go` retains a thin forwarding wrapper (lines 22–24) that delegates to `handler.SetupRouter`.

`setupTestServer` in `tests/api_test.go` (line 59) now calls:

```go
mux := handler.SetupRouter(h)
```

There are **zero** manual `mux.HandleFunc` calls in the test file. The single authoritative route table is in `handler.SetupRouter`. ✅

**Production behavior:** The actual mux for production goes through `handler.SetupRouter`. No routes were added, removed, or changed. ✅

---

### Fix 2 — Reuse production migration logic (`RunMigrations`)

**Required:** Tests must call `RunMigrations(db)` (the production embedded-FS function) instead of reading the SQL file from the filesystem via `os.ReadFile`.

**Verified:**

A new package `migrations` was created (`migrations/migrations.go`). It owns:

- The `//go:embed *.sql` directive (line 12) pointing at the `migrations/` directory.
- The exported `RunMigrations(db *sql.DB) error` function (lines 16–39).

`main.go` now delegates to `migrations.RunMigrations` via its own wrapper (lines 17–19).

`tests/api_test.go` imports `expense-tracker/migrations` (line 16) and calls (line 53):

```go
if err := migrations.RunMigrations(db); err != nil {
```

There is **no** `os.ReadFile`, `filepath.Join`, or any filesystem-based migration read anywhere in the test file. The test exercises exactly the same embedded-FS migration path as production. ✅

**Embed directive correctness:** `//go:embed *.sql` at the package root (`migrations/`) correctly embeds `001_create_expenses.sql`. `fs.ReadDir(FS, ".")` enumerates the embedded entries. The `.sql` suffix filter and alphabetical sort are present. `CREATE TABLE IF NOT EXISTS` makes the migration idempotent. ✅

---

### Fix 3 — Malformed JSON test

**Required:** A test that sends syntactically invalid JSON and asserts `400 Bad Request` plus a non-empty JSON error body.

**Verified:** `TestRequestHandling/Malformed JSON syntax` (lines 518–539):

```go
payload := []byte(`{"amount": 1000, "description":`)
resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
// ...
if resp.StatusCode != http.StatusBadRequest { ... }
var errResp handler.ErrorResponse
if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil { ... }
if errResp.Error == "" { ... }
```

The payload is a genuine truncated/incomplete JSON object. The Go `json.Decoder` returns `*json.SyntaxError` or `io.ErrUnexpectedEOF` on this input; the handler maps it to `400`. The test:

1. Uses `Content-Type: application/json` (via `http.Post`) so the Content-Type guard is cleared. ✅
2. Asserts `StatusBadRequest`. ✅
3. Decodes the response as `handler.ErrorResponse` and asserts the `Error` field is non-empty. ✅

The test correctly exercises and proves the malformed-JSON code path. ✅

---

### Fix 4 — Response Content-Type test

**Required:** A test verifying that JSON responses carry `Content-Type: application/json`.

**Verified:** `TestRequestHandling/Response Content-Type is application/json` (lines 595–645).

The test checks the header on four distinct response types:

| Sub-check | Endpoint | Expected Status |
|-----------|----------|----------------|
| 1 | `GET /health` | 200 |
| 2 | `GET /expenses` | 200 |
| 3 | `POST /expenses` (valid) | 201 |
| 4 | `POST /expenses` (invalid body `{}`) | 400 |

For each, it calls `resp.Header.Get("Content-Type")` and asserts the value is exactly `"application/json"`.

**Assertion exactness:** `writeJSON` sets `Content-Type: application/json` via `w.Header().Set(...)` before `WriteHeader`. Go's `net/http` does not append a charset suffix when the header is set explicitly before the first write. The exact-equality assertion is therefore correct and will detect any future unintended change. ✅

---

### Fix 5 — Whitespace-only category validation test (E2E)

**Required:** An E2E test proving that `{"category": "   "}` returns `400 Bad Request`.

**Verified:** `TestValidation` table (lines 458–466) includes:

```go
{
    name: "whitespace-only category",
    payload: `{
        "amount": 1000,
        "description": "Valid description",
        "category": "   ",
        "date": "2026-08-15"
    }`,
},
```

This case is iterated by the same loop that asserts `StatusBadRequest` and a non-empty error body. The category value `"   "` (three spaces) hits `strings.TrimSpace(r.Category) == ""` → `"category is required"`. ✅

`internal/model/expense_test.go` also now covers whitespace-only category for `UpdateExpenseRequest` (lines 102–105). Both types are covered at both layers. ✅

---

## Specific Verification Points

### Point 6 — `db.SetMaxOpenConns(1)` placement and architectural soundness

`main.go` line 38:

```go
db.SetMaxOpenConns(1)
```

Called immediately after `sql.Open` and before `RunMigrations`. Same call is present in `setupTestServer` (`tests/api_test.go` line 50).

**Correctness:** Setting the pool to a single connection serializes all database access, eliminating SQLite's `SQLITE_BUSY` / `database is locked` risk under concurrent HTTP requests. For a single-process personal expense tracker this is the correct remedy.

**Architectural impact:** All operations in this codebase are single-statement (no multi-statement transactions), so no goroutine can hold a connection from the pool and then attempt to acquire a second. No deadlock risk is introduced. ✅

### Point 7 — Tests actually test what they claim

- **Malformed JSON:** `{"amount": 1000, "description":` is genuinely malformed (unexpected EOF). The decoder errors, the handler writes 400. ✅
- **Content-Type:** `http.Get`/`http.Post` make real HTTP requests through `httptest.NewServer`. Headers come from the actual handler's `writeJSON`, not a mock. ✅
- **Whitespace-only category:** `"   "` triggers `strings.TrimSpace` → `""` → validation error. ✅
- **Shared router:** `handler.SetupRouter(h)` is the single call site for routes in both tests and production. ✅
- **Shared migrations:** `migrations.RunMigrations(db)` uses the same embedded FS in both contexts. ✅

### Point 8 — Malformed JSON handling

Handler `Create` and `Update` both use `json.NewDecoder` + `DisallowUnknownFields()`. Invalid JSON triggers `decoder.Decode` error → `writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())`. Correct. ✅

### Point 9 — JSON Content-Type assertions are correct

`writeJSON` sets `Content-Type: application/json` before `WriteHeader`. Go stdlib does not auto-append `; charset=utf-8` when the header is set explicitly. The exact-match assertion `ct != "application/json"` is correct. ✅

### Point 10 — Whitespace-only category validation

`strings.TrimSpace(r.Category) == ""` in both `CreateExpenseRequest.Validate()` and `UpdateExpenseRequest.Validate()`. `strings.TrimSpace("   ")` returns `""`. Tested at unit and E2E level. ✅

### Point 11 — Command results

| Command | Exit Code | Output |
|---------|-----------|--------|
| `go test -count=1 ./...` | 0 | `ok expense-tracker/internal/model 0.002s`, `ok expense-tracker/tests 0.014s` |
| `go vet ./...` | 0 | (no output) |
| `go build ./...` | 0 | (no output) |

All pass cleanly. ✅

---

## Production Behavior Analysis

The structural changes introduced are:

1. **`SetupRouter` relocated** from `main` package to `handler` package. Route table is identical. No routes added, removed, or changed.
2. **`RunMigrations` relocated** from `main` package to `migrations` package. Migration logic (sort, filter `.sql`, exec) is identical. The `go:embed` glob (`*.sql`) covers the same file.
3. **`db.SetMaxOpenConns(1)` added** in `main.go`. Additive safety constraint; cannot change existing correct behavior.

No handler logic, validation logic, store logic, or SQL schema was changed. ✅

---

## Findings

### [LOW — NF-01] Redundant forwarding wrappers in `main.go`

`main.go` defines two functions that immediately delegate and are never called from outside `main`:

```go
func RunMigrations(db *sql.DB) error {
    return migrations.RunMigrations(db)
}

func SetupRouter(h *handler.ExpenseHandler) *http.ServeMux {
    return handler.SetupRouter(h)
}
```

The `main` package cannot be imported by anything else in Go. These wrappers are therefore dead code from a reuse perspective. They add an indirection layer and create a maintenance hazard: a future developer editing `main.RunMigrations` or `main.SetupRouter` may believe they are modifying the real implementation when the real implementation lives in the `migrations` and `handler` packages respectively.

`main()` should call `migrations.RunMigrations(db)` and `handler.SetupRouter(h)` directly and the wrapper functions should be removed.

**Severity: LOW** — no correctness or behavioral impact; purely a code quality concern.

---

## Summary of Findings

| ID | Severity | Area | Description |
|----|----------|------|-------------|
| NF-01 | LOW | Code Quality | `main.go` retains forwarding wrappers for `RunMigrations` and `SetupRouter` that are redundant now that both functions live in importable packages |

**No CRITICAL, HIGH, or MEDIUM findings.**

All five TASK-001-FIXES items are fully implemented and verified. Production behavior is unchanged. All three toolchain commands pass cleanly.

---

## TASK-001-FIXES Checklist

| # | Requirement | Implemented | Verified |
|---|-------------|-------------|---------|
| 1 | Test setup calls `SetupRouter(h)` | ✅ (`handler.SetupRouter`) | ✅ |
| 2 | Test setup calls `RunMigrations(db)` | ✅ (`migrations.RunMigrations`) | ✅ |
| 3 | Malformed JSON → 400 test exists | ✅ (`TestRequestHandling/Malformed JSON syntax`) | ✅ |
| 4 | Response Content-Type test exists | ✅ (`TestRequestHandling/Response Content-Type is application/json`) | ✅ |
| 5 | Whitespace-only category test exists | ✅ (`TestValidation/Create: whitespace-only category`) | ✅ |
| — | `db.SetMaxOpenConns(1)` applied | ✅ (main.go:38, api_test.go:50) | ✅ |
| — | Production behavior unchanged | ✅ | ✅ |
| — | `go test -count=1 ./...` passes | ✅ | ✅ |
| — | `go vet ./...` passes | ✅ | ✅ |
| — | `go build ./...` passes | ✅ | ✅ |

---

```
STATUS: APPROVED
```
