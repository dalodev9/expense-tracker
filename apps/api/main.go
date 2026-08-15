package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"expense-tracker/internal/handler"
	"expense-tracker/internal/store"
	"expense-tracker/migrations"

	_ "modernc.org/sqlite"
)

// RunMigrations applies all SQL migrations from the embedded filesystem.
func RunMigrations(db *sql.DB) error {
	return migrations.RunMigrations(db)
}

// SetupRouter creates and configures the HTTP request multiplexer.
func SetupRouter(h *handler.ExpenseHandler) *http.ServeMux {
	return handler.SetupRouter(h)
}

func main() {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "expenses.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

	if err := RunMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	st := store.NewExpenseStore(db)
	h := handler.NewExpenseHandler(st)
	mux := SetupRouter(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
