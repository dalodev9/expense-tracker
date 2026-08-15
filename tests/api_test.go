package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"expense-tracker/internal/handler"
	"expense-tracker/internal/model"
	"expense-tracker/internal/store"
	"expense-tracker/migrations"

	_ "modernc.org/sqlite"
)

type testServer struct {
	server *httptest.Server
	db     *sql.DB
	url    string
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// 1. Temporary SQLite database file via os.CreateTemp
	tmpFile, err := os.CreateTemp("", "expense_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp db file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()

	t.Cleanup(func() {
		os.Remove(dbPath)
	})

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	db.SetMaxOpenConns(1)

	// Run migrations using production embedded migrations logic
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	st := store.NewExpenseStore(db)
	h := handler.NewExpenseHandler(st)
	mux := handler.SetupRouter(h)

	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		ts.Close()
	})

	return &testServer{
		server: ts,
		db:     db,
		url:    ts.URL,
	}
}

// 1. Health endpoint
func TestHealth(t *testing.T) {
	ts := setupTestServer(t)

	resp, err := http.Get(ts.url + "/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %q", body["status"])
	}
}

// 2. CRUD happy path
func TestCRUDHappyPath(t *testing.T) {
	ts := setupTestServer(t)

	// List expenses when empty -> returns []
	t.Run("List expenses — empty", func(t *testing.T) {
		resp, err := http.Get(ts.url + "/expenses")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var expenses []model.Expense
		if err := json.NewDecoder(resp.Body).Decode(&expenses); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if expenses == nil || len(expenses) != 0 {
			t.Fatalf("expected empty array [], got %v", expenses)
		}
	})

	// Create expense
	var created1, created2 model.Expense
	t.Run("Create expense 1", func(t *testing.T) {
		payload := []byte(`{
			"amount": 4999,
			"description": "Grocery shopping",
			"category": "food",
			"date": "2026-08-10"
		}`)

		resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 201, got %d, body: %s", resp.StatusCode, string(body))
		}

		if err := json.NewDecoder(resp.Body).Decode(&created1); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if created1.ID == "" {
			t.Fatal("expected non-empty id")
		}
		if created1.Amount != 4999 || created1.Description != "Grocery shopping" || created1.Category != "food" || created1.Date != "2026-08-10" {
			t.Fatalf("unexpected created expense data: %+v", created1)
		}
	})

	t.Run("Create expense 2", func(t *testing.T) {
		payload := []byte(`{
			"amount": 1500,
			"description": "Coffee",
			"category": "drinks",
			"date": "2026-08-15"
		}`)

		resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 201, got %d, body: %s", resp.StatusCode, string(body))
		}

		if err := json.NewDecoder(resp.Body).Decode(&created2); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if created2.ID == "" {
			t.Fatal("expected non-empty id")
		}
	})

	// List expenses (ordered by date DESC, id ASC)
	t.Run("List expenses", func(t *testing.T) {
		resp, err := http.Get(ts.url + "/expenses")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var expenses []model.Expense
		if err := json.NewDecoder(resp.Body).Decode(&expenses); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(expenses) != 2 {
			t.Fatalf("expected 2 expenses, got %d", len(expenses))
		}

		// Most recent date first (2026-08-15 before 2026-08-10)
		if expenses[0].ID != created2.ID || expenses[1].ID != created1.ID {
			t.Fatalf("expected ordering by date DESC, got [%s, %s]", expenses[0].Date, expenses[1].Date)
		}
	})

	// Get expense by ID
	t.Run("Get expense", func(t *testing.T) {
		resp, err := http.Get(ts.url + "/expenses/" + created1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var got model.Expense
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if got != created1 {
			t.Fatalf("expected %+v, got %+v", created1, got)
		}
	})

	// Update expense (full replacement)
	t.Run("Update expense", func(t *testing.T) {
		updatePayload := []byte(`{
			"amount": 5500,
			"description": "Weekly grocery",
			"category": "groceries",
			"date": "2026-08-11"
		}`)

		req, err := http.NewRequest(http.MethodPut, ts.url+"/expenses/"+created1.ID, bytes.NewReader(updatePayload))
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d, body: %s", resp.StatusCode, string(body))
		}

		var updated model.Expense
		if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if updated.ID != created1.ID || updated.Amount != 5500 || updated.Description != "Weekly grocery" || updated.Category != "groceries" || updated.Date != "2026-08-11" {
			t.Fatalf("unexpected updated expense: %+v", updated)
		}

		// Verify GET returns updated data
		getResp, err := http.Get(ts.url + "/expenses/" + created1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer getResp.Body.Close()

		var verified model.Expense
		json.NewDecoder(getResp.Body).Decode(&verified)
		if verified != updated {
			t.Fatalf("expected verified %+v, got %+v", updated, verified)
		}
	})

	// Delete expense
	t.Run("Delete expense", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.url+"/expenses/"+created1.ID, nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", resp.StatusCode)
		}

		// Subsequent GET -> 404
		getResp, err := http.Get(ts.url + "/expenses/" + created1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404 after delete, got %d", getResp.StatusCode)
		}
	})
}

// 3. Not found tests
func TestNotFoundCases(t *testing.T) {
	ts := setupTestServer(t)
	nonExistentID := "00000000-0000-0000-0000-000000000000"

	t.Run("Get expense — not found", func(t *testing.T) {
		resp, err := http.Get(ts.url + "/expenses/" + nonExistentID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}

		var errResp handler.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if errResp.Error != "expense not found" {
			t.Fatalf("expected 'expense not found', got %q", errResp.Error)
		}
	})

	t.Run("Update expense — not found", func(t *testing.T) {
		payload := []byte(`{
			"amount": 1000,
			"description": "Test",
			"category": "test",
			"date": "2026-08-15"
		}`)

		req, err := http.NewRequest(http.MethodPut, ts.url+"/expenses/"+nonExistentID, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("Delete expense — not found", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.url+"/expenses/"+nonExistentID, nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

// 4. Validation tests
func TestValidation(t *testing.T) {
	ts := setupTestServer(t)

	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "amount = 0",
			payload: `{
				"amount": 0,
				"description": "Valid description",
				"category": "valid-category",
				"date": "2026-08-15"
			}`,
		},
		{
			name: "amount = -1",
			payload: `{
				"amount": -1,
				"description": "Valid description",
				"category": "valid-category",
				"date": "2026-08-15"
			}`,
		},
		{
			name: "missing amount",
			payload: `{
				"description": "Valid description",
				"category": "valid-category",
				"date": "2026-08-15"
			}`,
		},
		{
			name: "invalid date format",
			payload: `{
				"amount": 1000,
				"description": "Valid description",
				"category": "valid-category",
				"date": "not-a-date"
			}`,
		},
		{
			name: "non-existent date",
			payload: `{
				"amount": 1000,
				"description": "Valid description",
				"category": "valid-category",
				"date": "2026-02-30"
			}`,
		},
		{
			name: "missing date",
			payload: `{
				"amount": 1000,
				"description": "Valid description",
				"category": "valid-category"
			}`,
		},
		{
			name: "empty description",
			payload: `{
				"amount": 1000,
				"description": "   ",
				"category": "valid-category",
				"date": "2026-08-15"
			}`,
		},
		{
			name: "empty category",
			payload: `{
				"amount": 1000,
				"description": "Valid description",
				"category": "",
				"date": "2026-08-15"
			}`,
		},
		{
			name: "whitespace-only category",
			payload: `{
				"amount": 1000,
				"description": "Valid description",
				"category": "   ",
				"date": "2026-08-15"
			}`,
		},
	}

	for _, tc := range tests {
		t.Run("Create: "+tc.name, func(t *testing.T) {
			resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader([]byte(tc.payload)))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected status 400, got %d, body: %s", resp.StatusCode, string(body))
			}

			var errResp handler.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if errResp.Error == "" {
				t.Fatal("expected non-empty error message")
			}
		})
	}
}

// 5. Request handling tests
func TestRequestHandling(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("Unknown JSON fields", func(t *testing.T) {
		payload := []byte(`{
			"amount": 1000,
			"description": "Valid description",
			"category": "food",
			"date": "2026-08-15",
			"extra_field": "disallowed"
		}`)

		resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 400, got %d, body: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("Malformed JSON syntax", func(t *testing.T) {
		payload := []byte(`{"amount": 1000, "description":`)

		resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 400 for malformed JSON, got %d, body: %s", resp.StatusCode, string(body))
		}

		var errResp handler.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if errResp.Error == "" {
			t.Fatal("expected non-empty error message")
		}
	})

	t.Run("Sending ID in body is rejected", func(t *testing.T) {
		payload := []byte(`{
			"id": "custom-id-should-fail",
			"amount": 1000,
			"description": "Valid description",
			"category": "food",
			"date": "2026-08-15"
		}`)

		resp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400 when body includes id, got %d", resp.StatusCode)
		}
	})

	t.Run("Wrong Content-Type on POST", func(t *testing.T) {
		payload := []byte(`amount=1000&description=test`)

		resp, err := http.Post(ts.url+"/expenses", "text/plain", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("expected status 415, got %d", resp.StatusCode)
		}
	})

	t.Run("Wrong Content-Type on PUT", func(t *testing.T) {
		payload := []byte(`amount=1000&description=test`)

		req, err := http.NewRequest(http.MethodPut, ts.url+"/expenses/some-id", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		req.Header.Set("Content-Type", "text/plain")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("expected status 415, got %d", resp.StatusCode)
		}
	})

	t.Run("Response Content-Type is application/json", func(t *testing.T) {
		// 1. Check GET /health
		healthResp, err := http.Get(ts.url + "/health")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer healthResp.Body.Close()

		if ct := healthResp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type 'application/json', got %q", ct)
		}

		// 2. Check GET /expenses
		listResp, err := http.Get(ts.url + "/expenses")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer listResp.Body.Close()

		if ct := listResp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type 'application/json', got %q", ct)
		}

		// 3. Check POST /expenses (201 Created)
		payload := []byte(`{
			"amount": 1000,
			"description": "Test",
			"category": "test",
			"date": "2026-08-15"
		}`)
		postResp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer postResp.Body.Close()

		if ct := postResp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type 'application/json', got %q", ct)
		}

		// 4. Check 400 Bad Request error response
		errResp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader([]byte(`{}`)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer errResp.Body.Close()

		if ct := errResp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type 'application/json', got %q", ct)
		}
	})

	t.Run("Update — full replacement (missing fields returns 400)", func(t *testing.T) {
		// Create an initial expense
		createPayload := []byte(`{
			"amount": 2000,
			"description": "Original",
			"category": "misc",
			"date": "2026-08-15"
		}`)
		createResp, err := http.Post(ts.url+"/expenses", "application/json", bytes.NewReader(createPayload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var created model.Expense
		json.NewDecoder(createResp.Body).Decode(&created)
		createResp.Body.Close()

		// Attempt PUT with only partial fields
		partialPayload := []byte(`{
			"amount": 3000
		}`)

		req, err := http.NewRequest(http.MethodPut, ts.url+"/expenses/"+created.ID, bytes.NewReader(partialPayload))
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400 for partial PUT update, got %d", resp.StatusCode)
		}
	})
}
