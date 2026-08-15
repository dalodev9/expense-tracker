# TASK-001 — Initialize Go API

## Objective

Initialize the Expense Tracker API according to the approved architecture.

## References

- `.ai/docs/requirements.md`
- `.ai/docs/architecture.md`

These documents are the source of truth.

## Scope

Implement only the initial application foundation:

1. Initialize the Go module.
2. Create the approved project structure.
3. Add the SQLite dependency.
4. Add the UUID dependency.
5. Implement database initialization.
6. Implement embedded migration loading.
7. Implement the `expenses` table migration.
8. Create the HTTP server.
9. Register the expense routes.
10. Add a basic health endpoint.
11. Add the initial test infrastructure.

## Health Endpoint

Add:

GET /health

Response:

HTTP 200

```json
{
  "status": "ok"
}