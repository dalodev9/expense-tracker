# Architecture Review — Expense Tracker API

**Reviewer:** Independent Senior Software Architect  
**Review Date:** 2026-08-15  
**Documents Reviewed:**
- `.ai/docs/requirements.md`
- `.ai/docs/architecture.md`

---

## Executive Summary

The architecture is broadly appropriate for the scope: a simple personal expense tracker REST API in Go. The layering is sane, the dependency choices are mostly defensible, and the requirements are fully enumerated in a coverage checklist. However, several issues range from a correctness bug (`:memory:` test isolation) to design decisions that will cause real pain either now or at the first iteration. These must be addressed before implementation begins.

---

## Issues

---

### CRITICAL-1 — `:memory:` SQLite in Tests Shares State Across Connections

**Severity:** CRITICAL

**What is wrong:**  
The architecture states tests use `":memory:"` SQLite. In the `modernc.org/sqlite` driver (and the standard `database/sql` connection pool), a `:memory:` DSN creates a **per-connection in-memory database**. Because `sql.DB` maintains a connection pool, different connections within the same pool may open **separate, independent in-memory databases**. This means a row `INSERT`ed on one pooled connection may be invisible to a subsequent `SELECT` on a different connection. Tests will exhibit intermittent, non-deterministic failures or false passes depending on pool sizing.

**Why it matters:**  
This is a correctness bug that will manifest in the test suite — the very artifact the architecture explicitly relies on for confidence. A green test suite built on a broken foundation is worse than no test suite.

**Concrete recommendation:**  
Use `file::memory:?cache=shared&mode=memory` with a `_mutex=full` or similar shared-cache DSN **and** pin the pool to a single connection:

```go
db, _ := sql.Open("sqlite", "file::memory:?cache=shared&mode=memory")
db.SetMaxOpenConns(1)
```

Or simply use a temporary file-based database (`os.CreateTemp`) for tests, which avoids the shared-cache complexity entirely and is just as fast at this scale.

---

### CRITICAL-2 — `float64` for Monetary Values Introduces Silent Rounding Errors

**Severity:** CRITICAL

**What is wrong:**  
`float64` cannot represent all decimal fractions exactly. For example, `0.1 + 0.2 != 0.3` in IEEE 754. An expense entered as `49.99` may be stored as `49.98999999999999772626...`, then returned over the API as `49.99` only because `json.Marshal` rounds the display — but the stored value is wrong. Aggregations (e.g., a future monthly total) will accumulate this error silently.

The architecture acknowledges this and dismisses it as "over-engineering." This is an incorrect risk assessment. The overhead of using integer cents (`int64`) is **zero additional complexity** — it is a trivial representation choice, not an extra library or layer.

**Why it matters:**  
Financial values that drift due to floating-point error erode user trust and are a serious correctness bug in the domain of money. The fix is not over-engineering; it is using the right primitive.

**Concrete recommendation:**  
Store `amount` as `INTEGER` cents in SQLite (e.g., `4999` for `$49.99`). Expose it as a JSON integer (cents) or as a formatted string (`"49.99"`) in the API. This eliminates floating-point error with no additional dependencies and no meaningful complexity increase.

If a decimal string representation is preferred for the API, use `"amount": "49.99"` (string) and parse with `math/big` or a small `decimal` helper — still no third-party library required.

---

### HIGH-1 — PUT Semantics Are Ambiguous / Likely Incorrect

**Severity:** HIGH

**What is wrong:**  
The architecture specifies `PUT /expenses/{id}` for "Update an expense" but does not define whether PUT is a full replacement or a partial update. RFC 9110 defines PUT as a **full replacement** of the resource. The request body shown in §4 omits `id` (correct for PUT) but gives no guidance on what happens if only some fields are provided. If the implementation treats missing fields as "no change," it is implementing PATCH semantics with a PUT verb — a semantic mismatch that will confuse clients. If it treats missing fields as clearing them (true PUT), then callers must always send the full resource, which is not stated anywhere.

**Why it matters:**  
An ambiguous update contract leads to bugs in clients, especially once authentication and a real frontend are added. It also makes the test cases ambiguous: the test "PUT with changes → 200, data updated" does not specify whether omitted fields are preserved or zeroed.

**Concrete recommendation:**  
Make the choice explicit in the architecture:

- **Option A (true PUT):** Require all non-id fields. Missing fields are a `400` validation error. State this clearly.
- **Option B (PATCH):** Rename the verb to `PATCH` and document partial-update semantics. Use Go pointer fields (`*float64`, `*string`) to distinguish "not provided" from "zero value."

For a simple CRUD tracker, Option A (true PUT with all fields required) is simpler and correct. Just document it.

---

### HIGH-2 — "Not Found" DB Error → HTTP 404 Mapping Is Unspecified and Fragile

**Severity:** HIGH

**What is wrong:**  
The architecture mandates that a missing expense returns `404 Not Found`, but the store layer returns a generic `error`. The architecture provides no mechanism for the handler to distinguish "row not found" from "database I/O error." Go's `database/sql` returns `sql.ErrNoRows` for empty result sets, but this is an implementation detail never mentioned in the architecture. If the handler simply checks `err != nil` and returns `500`, it will return 500 on a missing ID. If it blindly maps all store errors to 404, a real DB failure will silently become a 404.

**Why it matters:**  
Error classification is a core part of the API contract. Getting it wrong silently violates the documented status-code table and makes debugging production failures much harder.

**Concrete recommendation:**  
Define a sentinel error in the store package:

```go
var ErrNotFound = errors.New("expense not found")
```

The store returns `ErrNotFound` when `sql.ErrNoRows` is encountered. The handler checks `errors.Is(err, store.ErrNotFound)` to return 404; all other errors become 500. This pattern is simple, idiomatic Go, and explicit.

---

### HIGH-3 — Unknown JSON Fields Are Silently Accepted

**Severity:** HIGH

**What is wrong:**  
The architecture uses `json.Decoder` (implied by standard Go JSON decoding) without specifying `DisallowUnknownFields()`. A client sending `{"amount": 10, "description": "x", "category": "y", "date": "2026-01-01", "user_id": "admin"}` will have `user_id` silently ignored. This is not a security issue today (no auth), but it establishes a pattern of silently accepting garbage input.

**Why it matters:**  
Silent acceptance of unknown fields:
1. Hides client bugs (wrong field name, wrong endpoint).
2. Establishes a precedent that makes adding auth fields later dangerous — an old client sending `user_id` in the body would silently succeed rather than being rejected.
3. Makes the API surface unclear to consumers.

**Concrete recommendation:**  
Add `decoder.DisallowUnknownFields()` before `decoder.Decode(&req)` in all handlers that parse a request body. Return `400 Bad Request` on unknown fields. This is a single line per handler and costs nothing.

---

### MEDIUM-1 — Migration Applied via Raw SQL File Read at Startup Is Fragile

**Severity:** MEDIUM

**What is wrong:**  
The architecture states the migration SQL is applied at startup by reading and executing `migrations/001_create_expenses.sql`. This means:
1. The binary has a runtime dependency on the filesystem path `migrations/001_create_expenses.sql`. If the binary is moved or deployed without the `migrations/` directory, startup fails or silently skips migration.
2. There is no concept of migration state — applying `CREATE TABLE IF NOT EXISTS` every startup is idempotent but not scalable. A second migration (`002_add_notes.sql`) has no documented application mechanism.
3. No error handling for migration failure is documented.

**Why it matters:**  
For a single-migration schema this is tolerable, but the architecture presents no path for schema evolution, which is a near-certainty. The filesystem dependency also makes the binary non-self-contained.

**Concrete recommendation:**  
Embed the SQL file(s) using Go's `//go:embed` directive. This makes the binary self-contained:

```go
//go:embed migrations/*.sql
var migrationFS embed.FS
```

For migration ordering, a minimal approach is sufficient: embed files, sort by filename, track applied migrations in a `schema_migrations` table. At this scale, a micro-library like `golang-migrate` is acceptable — or implement the minimal version manually. At minimum, document the migration lifecycle.

---

### MEDIUM-2 — Validation Split: Handler vs. Model Is Architecturally Inconsistent

**Severity:** MEDIUM

**What is wrong:**  
§2 states "Validation is performed in the handler layer." §7 states validation is a method on the model: `func (e Expense) Validate() error`. The package responsibility table says `internal/model` is for "data structures and validation logic." These three statements are in direct tension. If `Validate()` is on the model, who calls it — the handler or the model itself? If it is the handler's responsibility to call `Validate()`, what stops someone from bypassing it? If validation is truly in the model, then the model must return errors the handler can translate to 400.

**Why it matters:**  
Ambiguous ownership of validation means it will be inconsistently applied. When a second entry point is added (e.g., a batch import endpoint), the developer must guess where to call validation.

**Concrete recommendation:**  
Settle on the model approach, which is better:
- `Validate()` lives on the model struct and enforces all business rules.
- The handler always calls `expense.Validate()` after decoding the request. If `Validate()` returns an error, the handler returns 400.
- Document this contract explicitly: **all callers of the store must have a validated model.**

The "handler layer" phrasing in §2 should be removed or corrected to say "the handler triggers model-level validation."

---

### MEDIUM-3 — `date` Field Validation Is Underspecified

**Severity:** MEDIUM

**What is wrong:**  
The architecture says `date` must be a "valid date in `YYYY-MM-DD` format," but does not specify:
- Whether future dates are allowed (e.g., can you create an expense for next year?).
- Whether the date must be parseable using Go's `time.Parse("2006-01-02", date)` — which is implied but not stated.
- What happens if the date string passes format validation but is semantically invalid (e.g., `"2026-02-30"`). Go's `time.Parse` will reject this, but only if `time.Parse` is actually used.

**Why it matters:**  
Under-specified validation becomes inconsistently implemented. `"9999-99-99"` passes a regex but fails `time.Parse`. The architecture must specify which validator is authoritative.

**Concrete recommendation:**  
Specify: "The `date` field is validated using `time.Parse("2006-01-02", date)` with no additional range restrictions." This is one sentence and removes all ambiguity.

---

### MEDIUM-4 — No Interface on Store Prevents Unit Testing of the Handler

**Severity:** MEDIUM

**What is wrong:**  
The architecture explicitly rejects store interfaces. The stated rationale is "only one storage backend exists." However, the consequence is that **handler logic cannot be unit-tested in isolation**. The only testing path is through the full E2E stack. If the handler ever gains non-trivial logic (e.g., business rule enforcement, response shaping, conditional logic), that logic can only be tested by standing up a full SQLite database.

**Why it matters:**  
The architecture relies entirely on E2E tests. E2E tests are slow feedback and cannot easily inject error conditions (e.g., "what if the store returns an error on Create? Does the handler return 500?"). Without a store interface, testing the handler's error-handling paths requires either database failure injection (complex) or trusting the code unverified.

**Concrete recommendation:**  
Define a minimal store interface — not for multi-backend reasons, but for testability:

```go
type ExpenseStore interface {
    Create(model.Expense) (model.Expense, error)
    List() ([]model.Expense, error)
    GetByID(string) (model.Expense, error)
    Update(string, model.Expense) (model.Expense, error)
    Delete(string) error
}
```

The concrete `*store.ExpenseStore` satisfies this interface automatically. No DI framework needed. The handler takes the interface. This is not over-engineering — it is idiomatic Go and the standard way to write testable code. The "introduce interfaces when a second implementation is needed" rule is a common but misapplied heuristic; the primary driver should be testability, not backend count.

---

### MEDIUM-5 — List Endpoint Has No Pagination or Filtering Acknowledgement

**Severity:** MEDIUM

**What is wrong:**  
`GET /expenses` returns all expenses with no pagination, filtering, or sorting. The requirements do not specify any of these, so this is not a requirements gap — but the architecture also does not acknowledge this limitation or provide a migration path.

**Why it matters:**  
A personal expense tracker accumulates records over months or years. Without pagination, the response body grows unbounded. At 10,000 expenses this becomes a real latency and memory concern. More importantly, adding pagination later is a **breaking API change** if clients have been built against the unpaginated format.

**Concrete recommendation:**  
Add a short note to the architecture: "Pagination is not implemented in v1. When needed, add `?limit=` and `?offset=` query parameters; this is additive and non-breaking." This costs zero implementation effort and saves a future architectural discussion.

---

### LOW-1 — `id` in Request Body Is "Ignored" — No Documented Behavior

**Severity:** LOW

**What is wrong:**  
The architecture states "`id` is ignored in request bodies." This means if a client sends `{"id": "evil-uuid", "amount": 10, ...}` in a POST, the server silently discards `id` and generates a new one. This is correct behavior, but "ignored" is vague — it could mean "silently dropped" or "causes a 400."

**Why it matters:**  
Without specifying this explicitly in the API contract, clients have no way to know whether passing `id` is an error. If `DisallowUnknownFields()` is later enabled (per HIGH-3), `id` in the request body would suddenly become a 400 — a breaking change.

**Concrete recommendation:**  
Use separate request structs: `CreateExpenseRequest` and `UpdateExpenseRequest` (without an `id` field) for decoding request bodies, distinct from `Expense` (which has `id`). This makes the separation structurally enforced, not documented policy. With `DisallowUnknownFields()` enabled, clients sending `id` get an immediate, helpful 400.

---

### LOW-2 — No Structured Logging

**Severity:** LOW

**What is wrong:**  
The architecture mentions "Internal error details are logged server-side" but specifies no logging approach — not even `log.Printf`. There is no mention of request IDs, which makes tracing a specific failing request in logs impossible.

**Why it matters:**  
Without structured logging or request correlation, debugging a reported `500` in production means searching through a wall of unordered log lines. Even a simple `log/slog` setup is standard in Go 1.21+ and costs almost nothing.

**Concrete recommendation:**  
Use Go 1.21's `log/slog` for structured logging. Log at minimum: method, path, status code, and duration per request (a simple middleware). On errors, log the error with the request context. No third-party library required.

---

### LOW-3 — No Content-Type Enforcement on Incoming Requests

**Severity:** LOW

**What is wrong:**  
The architecture says all endpoints consume `application/json` but does not specify that the handler validates the `Content-Type` header. A client sending `application/x-www-form-urlencoded` data will have their request processed by `json.Decoder`, which will return a decode error that maps to 400 — but with a confusing message like "invalid character '&' looking for beginning of value" rather than "Content-Type must be application/json."

**Why it matters:**  
Poor error messages waste developer time during integration.

**Concrete recommendation:**  
Add a simple check: if `Content-Type` is not `application/json` (on POST/PUT), return `415 Unsupported Media Type`. One if-statement per handler, or a middleware.

---

### LOW-4 — List Sort Order Is Undefined

**Severity:** LOW

**What is wrong:**  
The database schema has no `created_at` column, and the `SELECT` query for `List()` is not shown. SQLite does not guarantee row order without an explicit `ORDER BY`. This means:
- The list response order is non-deterministic.
- Tests that compare an ordered list against expected results will be fragile.

**Why it matters:**  
Non-deterministic ordering causes intermittent test failures and confuses API consumers.

**Concrete recommendation:**  
Add `ORDER BY date DESC, id ASC` (or similar) to the `List()` query and document the sort order in the architecture. If insertion order is desired, add a `created_at` column (internal only, not exposed in the API) and sort by it.

---

### SUGGESTION-1 — `router` Package May Be Over-Structured for Current Scale

**Severity:** SUGGESTION

**What is wrong:**  
A dedicated `internal/router` package containing a single file `router.go` whose sole job is to register 5 routes on a `*http.ServeMux` adds a package boundary with essentially no value at this scale. The route registration is ~10 lines of code.

**Concrete recommendation:**  
Consider inlining route registration in `main.go`. If the router package grows (middleware, grouping), extract it then. This is purely a readability concern and has no correctness implications.

---

### SUGGESTION-2 — `google/uuid` as External Dependency (Justified)

**Severity:** SUGGESTION

**What is wrong:**  
`google/uuid` is introduced solely to generate UUID v4 IDs. Go's standard library does not have a native UUID generator, so this dependency is justified. A UUID can be generated from `crypto/rand` in ~5 lines without any dependency, but `google/uuid` is stable and universally used.

**Concrete recommendation:**  
Keep `google/uuid`. Flagged here only for completeness.

---

### SUGGESTION-3 — Test Matrix Misses Validation Boundary Cases

**Severity:** SUGGESTION

**What is wrong:**  
The test matrix in §8 includes "Create expense — invalid: POST missing fields → 400" but does not enumerate specific invalid cases:
- `amount = 0` (boundary: must be > 0)
- `amount = -1` (negative)
- `date = "not-a-date"` (invalid format)
- `date = "2026-02-30"` (invalid date, passes naive format check)
- Empty `description` or `category` string

**Concrete recommendation:**  
Expand the test matrix to enumerate at least the boundary cases above. A developer implementing validation will have no test guidance for boundaries otherwise.

---

## Summary Table

| ID | Severity | Topic |
|----|----------|-------|
| CRITICAL-1 | CRITICAL | `:memory:` SQLite test isolation — connection pool bug |
| CRITICAL-2 | CRITICAL | `float64` for monetary amounts — silent rounding error |
| HIGH-1 | HIGH | PUT semantics undefined — full replace vs. partial update |
| HIGH-2 | HIGH | DB "not found" → 404 mapping mechanism unspecified |
| HIGH-3 | HIGH | Unknown JSON fields silently accepted |
| MEDIUM-1 | MEDIUM | Migration applied from filesystem — not embedded, not versioned |
| MEDIUM-2 | MEDIUM | Validation ownership split between handler and model descriptions |
| MEDIUM-3 | MEDIUM | `date` validation underspecified |
| MEDIUM-4 | MEDIUM | No store interface prevents handler unit-testing |
| MEDIUM-5 | MEDIUM | List endpoint unbounded — no pagination acknowledgement |
| LOW-1 | LOW | `id` in request body behavior undocumented; no separate request struct |
| LOW-2 | LOW | No structured logging or request correlation |
| LOW-3 | LOW | No Content-Type enforcement |
| LOW-4 | LOW | List sort order undefined |
| SUGGESTION-1 | SUGGESTION | `router` package may be over-structured |
| SUGGESTION-2 | SUGGESTION | `google/uuid` dependency (justified) |
| SUGGESTION-3 | SUGGESTION | Test matrix misses validation boundary cases |

---

## STATUS: CHANGES_REQUIRED

Two CRITICAL issues (`:memory:` connection pool bug, `float64` monetary representation) and three HIGH issues (PUT semantics, not-found error mapping, unknown JSON fields) must be resolved before implementation begins. The MEDIUM and LOW items should be addressed during implementation; they do not block design approval.
