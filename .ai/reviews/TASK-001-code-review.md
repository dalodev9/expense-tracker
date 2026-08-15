# TASK-001 — Code Review

**Reviewer:** Independent Senior Go Code Reviewer  
**Review Date:** 2026-08-15  
**Task:** TASK-001 — Initialize Go API  
**Source of Truth:** `.ai/docs/architecture.md`, `.ai/docs/requirements.md`, `.ai/tasks/TASK-001.md`

---

## Files Inspected

| File | Size |
|------|------|
| `main.go` | 91 lines |
| `internal/model/expense.go` | 72 lines |
| `internal/model/expense_test.go` | 120 lines |
| `internal/store/expense.go` | 101 lines |
| `internal/handler/expense.go` | 188 lines |
| `migrations/001_create_expenses.sql` | 8 lines |
| `tests/api_test.go` | 611 lines |
| `go.mod` | 20 lines |

`go build ./...`, `go vet ./...`, and `go test ./...` all pass cleanly.

---

## 1. Architecture Compliance

**Result: COMPLIANT.**

### Package Responsibilities

- `main`: Wires dependencies, registers routes, applies migrations, starts the server. ✅
- `internal/handler`: Decodes requests, enforces Content-Type, calls validation, calls store, writes responses. ✅
- `internal/model`: Defines structs and all validation logic. ✅
- `internal/store`: All database CRUD, sentinel errors. ✅

### Dependency Flow

`main → handler → store` and `handler → model` and `store → model`. No cycles. No upward dependencies. ✅

### No Interfaces / No DI Framework

The handler takes a concrete `*store.ExpenseStore`. No interface is defined. Dependencies are wired manually in `main.go`. ✅

### Standard Library HTTP

`net/http` with Go 1.22+ method+path routing patterns (`"GET /expenses/{id}"`). No third-party router. ✅

### Store Design

Exactly matches the architecture spec: `ErrNotFound` sentinel, concrete `ExpenseStore` struct, `NewExpenseStore(db *sql.DB)`, correct method signatures. ✅

### Embedded Migrations

`//go:embed migrations/*.sql` in `main.go`. Files are sorted by name before execution. `CREATE TABLE IF NOT EXISTS` makes it idempotent. ✅

### UUID Generation

`github.com/google/uuid` used in `store.Create`. UUID is generated in the store when the ID is empty. ✅

### Amount as Integer Cents

`int64` in the model struct; `INTEGER NOT NULL` in the schema; `4999` represents `$49.99`. ✅

### Request/Response Models

`CreateExpenseRequest` and `UpdateExpenseRequest` have no `id` field. `DisallowUnknownFields()` is set before decode on both POST and PUT. ✅

### Route Registration in `main.go`

`SetupRouter` is ~10 lines, lives in `main.go`. No separate router package. ✅

---

## 2. Correctness

**Result: CORRECT.**

### CRUD Behavior

- `Create`: Generates UUID, inserts, returns the full `model.Expense` with the generated ID. ✅
- `List`: Returns `make([]model.Expense, 0)` — never nil — ensuring `[]` not `null` in JSON output. ✅
- `GetByID`: Maps `sql.ErrNoRows` → `ErrNotFound`. ✅
- `Update`: Checks `RowsAffected()`, returns `ErrNotFound` if zero. Sets `expense.ID = id` before returning, so the response contains the correct ID. ✅
- `Delete`: Checks `RowsAffected()`, returns `ErrNotFound` if zero. ✅

### HTTP Status Codes

| Scenario | Expected | Implemented |
|----------|----------|-------------|
| Create success | 201 | ✅ |
| List success | 200 | ✅ |
| Get success | 200 | ✅ |
| Update success | 200 | ✅ |
| Delete success | 204 | ✅ |
| Not found | 404 | ✅ |
| Validation error | 400 | ✅ |
| Wrong Content-Type | 415 | ✅ |
| Unknown JSON fields | 400 | ✅ |
| DB error | 500 | ✅ |

### PUT Full-Replacement Semantics

Validation rules are identical to POST. A partial body (e.g., `{"amount": 3000}`) triggers 400 because `description`, `category`, and `date` fail their non-empty/format checks. ✅

### Empty List

`make([]model.Expense, 0)` marshals to `[]` in JSON, not `null`. ✅ Verified by test.

### Ordering

`ORDER BY date DESC, id ASC` in the `List` query. ✅ Verified by test.

---

## 3. SQLite Correctness

**Result: CORRECT, with one LOW observation.**

### Connection Handling

`sql.Open` followed by `db.Exec` for migrations. The `*sql.DB` is passed to the store and shared across requests. ✅

### Parameterized Queries

All queries use `?` placeholders. No string concatenation or `fmt.Sprintf` for SQL construction. No SQL injection risk. ✅

### `rows.Close()` and `rows.Err()`

`List` correctly defers `rows.Close()` and checks `rows.Err()` after the loop. ✅

### Resource Cleanup

`db.Close()` is called via `defer` in `main`. Store methods do not leak statement handles. ✅

### `sql.ErrNoRows` Handling

Correctly handled in `GetByID` only. `Update` and `Delete` use `RowsAffected()` instead, which is the appropriate pattern. ✅

### [LOW — F-01] No `db.SetMaxOpenConns(1)` for SQLite

SQLite (especially with the default DELETE journal mode) does not support concurrent writers. The `sql.DB` pool defaults to unlimited connections, which can cause `SQLITE_BUSY` errors under concurrent load. For a personal expense tracker this is a latent rather than immediate risk.

**Recommendation:** Add `db.SetMaxOpenConns(1)` in `main.go` after `sql.Open`.

---

## 4. HTTP/API Correctness

**Result: CORRECT with one LOW observation.**

### Routing

All five expense routes plus `/health` are registered with correct method+path patterns. Go 1.22+ `net/http` automatically returns `405 Method Not Allowed` for unregistered methods on registered paths. ✅

### Path Parameters

`r.PathValue("id")` used correctly. The `id == ""` guard is harmless; `net/http` routing with `{id}` cannot produce an empty path value in practice. ✅

### `checkContentType`

Correctly handles media-type parameters (e.g., `application/json; charset=utf-8`) by stripping the `;` suffix before comparing. ✅

### `writeJSON` / `writeError`

`Content-Type: application/json` is set before `WriteHeader`. Error details are not exposed in 500 responses — logged server-side only. ✅

### DELETE Response

`w.WriteHeader(http.StatusNoContent)` with no body. ✅

### [LOW — F-02] Decoder Error Detail in 400 Response Body

```go
writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
```

The standard library decoder's error message (e.g., `json: unknown field "extra_field"`) is included verbatim. This carries no sensitive internal state and helps API clients debug malformed requests. Noted as LOW because the architecture's "no internal details to client" guidance is aimed at 500 errors; 400 diagnostic messages are standard practice.

---

## 5. Validation

**Result: FULLY COMPLIANT.**

| Rule | Implementation | Result |
|------|---------------|--------|
| `amount > 0` | `r.Amount <= 0` → error | ✅ |
| `amount == 0` | Caught by `<= 0` | ✅ |
| `amount < 0` | Caught by `<= 0` | ✅ |
| `amount` missing from JSON | Defaults to `0`, caught by `<= 0` | ✅ |
| `description` non-empty after trim | `strings.TrimSpace(r.Description) == ""` | ✅ |
| `description` whitespace-only | `strings.TrimSpace` catches `"   "` | ✅ |
| `description` empty string | Caught | ✅ |
| `category` non-empty after trim | `strings.TrimSpace(r.Category) == ""` | ✅ |
| `category` whitespace-only | Caught | ✅ |
| `category` empty string | Caught | ✅ |
| `date` valid format | `time.Parse("2006-01-02", date)` | ✅ |
| `date` impossible (2026-02-30) | `time.Parse` returns "day out of range" | ✅ |
| `date` invalid string | `time.Parse` returns error | ✅ |
| `date` empty/missing | `time.Parse("")` returns error | ✅ |
| `date` missing from JSON | Defaults to `""`, `time.Parse` fails | ✅ |

Independently verified: `time.Parse("2006-01-02", "2026-02-30")` → `parsing time "2026-02-30": day out of range`. Impossible dates are correctly rejected. ✅

---

## 6. Security and Robustness

**Result: NO ISSUES WITHIN PROJECT SCOPE.**

### SQL Injection

All queries use parameterized `?` placeholders. No dynamic SQL construction. ✅

### Unbounded Request Bodies

`http.MaxBytesReader` is not used. A client could send a large body. For a personal expense tracker with no public exposure this is within acceptable scope. Not flagged as a finding.

### Error Disclosure

500 responses return only `{"error": "internal error"}`. DB errors are logged server-side via `log.Printf`. ✅

### Panic Possibilities

No `panic` calls. No unguarded type assertions. Handler is nil-safe given the constructor always initializes the store field. ✅

### Path Traversal / Injection

IDs flow from `r.PathValue("id")` directly into parameterized SQL. No filesystem operations using user-supplied data. ✅

---

## 7. Testing Quality

**Result: GOOD. All architecture-mandated cases are present and passing.**

### Tests Are Meaningful

Tests exercise the real stack: temp-file SQLite → store → handler → `httptest.NewServer` → real HTTP client. Not mocked unit tests. ✅

### Temp-File Approach

`os.CreateTemp` + `t.Cleanup(os.Remove(dbPath))`. The architecture's concern about `:memory:` connection pool isolation is correctly addressed. ✅

### Coverage vs. Architecture Checklist

| Architecture Test Case | Present |
|------------------------|---------|
| Create expense → 201, generated id, correct data | ✅ |
| List expenses, ordered date DESC | ✅ |
| List expenses — empty → `[]` | ✅ |
| Get expense → 200, correct data | ✅ |
| Update expense → 200, full replacement | ✅ (verifies GET after PUT) |
| Delete expense → 204, subsequent GET → 404 | ✅ |
| Get not found → 404 | ✅ (checks error body) |
| Update not found → 404 | ✅ |
| Delete not found → 404 | ✅ |
| Amount = 0 → 400 | ✅ |
| Amount = -1 → 400 | ✅ |
| Missing amount → 400 | ✅ |
| Invalid date format → 400 | ✅ |
| Non-existent date (2026-02-30) → 400 | ✅ |
| Missing date → 400 | ✅ |
| Empty description → 400 | ✅ |
| Empty category → 400 | ✅ |
| Unknown JSON fields → 400 | ✅ |
| Wrong Content-Type POST → 415 | ✅ |
| Wrong Content-Type PUT → 415 | ✅ |
| PUT partial fields → 400 | ✅ |
| Sending `id` in body → 400 | ✅ |
| Health endpoint → 200 `{"status":"ok"}` | ✅ |

**All architecture-mandated test cases are present and pass.**

### [LOW — F-03] Missing Test: Malformed JSON Body

No test sends syntactically invalid JSON (e.g., `{broken`). The handler handles it correctly — the decoder returns an error — but it is unverified by a test.

### [LOW — F-04] Missing Test: Response `Content-Type` Header

No test asserts that responses carry `Content-Type: application/json`. The handler does set it in `writeJSON`, but this is untested.

### [LOW — F-05] Whitespace-Only Category Not Tested in E2E Suite

The E2E `TestValidation` uses `""` for empty category, not `"   "`. The model unit test covers whitespace-only description but not whitespace-only category. The production code handles both correctly via `strings.TrimSpace`; this is a minor coverage gap.

### [SUGGESTION — F-06] Test Infrastructure Duplicates Route Registration

`setupTestServer` re-registers all 6 routes manually instead of calling `SetupRouter(h)` (which is already exported from `main.go`). If a route is added to `main.go` but missed in `setupTestServer`, it will be silently untested.

**Recommendation:** Replace the manual `mux.HandleFunc` calls in `setupTestServer` with `SetupRouter(h)`.

### [SUGGESTION — F-07] Test Migration Read from Filesystem Instead of Embedded FS

`setupTestServer` reads the migration via `os.ReadFile(filepath.Join("..", "migrations", "001_create_expenses.sql"))` rather than calling the production `RunMigrations(db)` function. If the embedded file and the on-disk file diverge, tests would not detect it.

**Recommendation:** Export `RunMigrations` (it already is exported) and call it in `setupTestServer`.

---

## 8. Code Quality

**Result: HIGH QUALITY. Idiomatic Go throughout.**

### Naming

All exported identifiers are clear and conventional: `ExpenseStore`, `NewExpenseStore`, `ErrNotFound`, `ExpenseHandler`. Unexported helpers (`writeJSON`, `writeError`, `checkContentType`) are appropriately scoped. ✅

### Error Handling

Errors are wrapped with context in `RunMigrations`. Handler logs and maps errors without leaking internals. Store returns errors as-is for non-sentinel cases. ✅

### No Unnecessary Abstractions

No interfaces, no service layer, no middleware chains, no DI framework. The code is exactly as complex as the problem requires. ✅

### Acceptable Duplication

`CreateExpenseRequest.Validate()` and `UpdateExpenseRequest.Validate()` are structurally identical. The architecture explicitly notes this is intentional — the two types remain separate to prevent clients from setting IDs. ✅

### Static Analysis

`go vet ./...` passes cleanly. ✅

### Readability

All functions are short and single-purpose. Exported types and handlers have doc comments. ✅

---

## Summary of Findings

| ID | Severity | Area | Description |
|----|----------|------|-------------|
| F-01 | LOW | SQLite | No `db.SetMaxOpenConns(1)`; latent `SQLITE_BUSY` risk under concurrent writes |
| F-02 | LOW | HTTP | 400 responses include raw decoder error strings (acceptable; no sensitive state) |
| F-03 | LOW | Testing | No test for syntactically malformed JSON body |
| F-04 | LOW | Testing | No test for `Content-Type: application/json` on responses |
| F-05 | LOW | Testing | Whitespace-only category not covered in E2E suite (covered in model unit tests) |
| F-06 | SUGGESTION | Testing | `setupTestServer` duplicates route registration instead of calling `SetupRouter` |
| F-07 | SUGGESTION | Testing | Test migration read from filesystem instead of using `RunMigrations` |

**No CRITICAL or HIGH findings.**

---

## Verdict

The implementation is correct, complete, and faithfully follows the approved architecture. All architecture-mandated test cases are present and passing. The code is idiomatic Go with no security issues within the project's scope. The seven findings are minor quality and robustness observations that do not block continued development.

```
STATUS: APPROVED
```
