# Expense Tracker API

A REST API for managing personal expenses, written in Go with SQLite persistence.

## Features

- **Create expense**: `POST /expenses`
- **List expenses**: `GET /expenses`
- **Get expense**: `GET /expenses/{id}`
- **Update expense**: `PUT /expenses/{id}`
- **Delete expense**: `DELETE /expenses/{id}`
- **Health check**: `GET /health`

## Running the Application

```bash
go run main.go
```

Environment variables:
- `PORT` — Port number to listen on (default: `8080`)
- `DATABASE_PATH` — Path to SQLite database file (default: `expenses.db`)

## Running Tests

```bash
go test ./...
```
