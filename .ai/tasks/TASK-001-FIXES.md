# TASK-001-FIXES — Address Code Review Findings

## Objective

Address the non-blocking findings from the independent TASK-001 code review.

## References

- `.ai/docs/requirements.md`
- `.ai/docs/architecture.md`
- `.ai/tasks/TASK-001.md`
- `.ai/reviews/TASK-001-code-review.md`

The approved architecture remains the source of truth.

## Scope

Implement only the following test-quality improvements.

### 1. Reuse production route registration

Update the test server setup so it uses the existing `SetupRouter(h)` function instead of manually registering routes again.

This prevents production and test routing from diverging.

### 2. Reuse production migration logic

Update the test setup so it calls the production `RunMigrations(db)` function instead of reading:

`migrations/001_create_expenses.sql`

directly from the filesystem.

Tests should exercise the same embedded migration mechanism used by the application.

### 3. Add malformed JSON test

Add an API test proving that syntactically invalid JSON returns:

`400 Bad Request`

and the standard JSON error response.

### 4. Add response Content-Type test

Add an API test verifying JSON responses include:

`Content-Type: application/json`

### 5. Add whitespace-only category validation test

Add a validation test proving that:

```json
{
  "category": "   "
}