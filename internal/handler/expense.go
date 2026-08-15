package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"expense-tracker/internal/model"
	"expense-tracker/internal/store"
)

type ExpenseHandler struct {
	store *store.ExpenseStore
}

func NewExpenseHandler(store *store.ExpenseStore) *ExpenseHandler {
	return &ExpenseHandler{store: store}
}

// SetupRouter creates and configures the HTTP request multiplexer with all routes.
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

type ErrorResponse struct {
	Error string `json:"error"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func checkContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	mediaType := strings.TrimSpace(strings.Split(ct, ";")[0])
	return mediaType == "application/json"
}

// Health handles GET /health.
func (h *ExpenseHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// Create handles POST /expenses.
func (h *ExpenseHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !checkContentType(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var req model.CreateExpenseRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.Create(model.Expense{
		Amount:      req.Amount,
		Description: req.Description,
		Category:    req.Category,
		Date:        req.Date,
	})
	if err != nil {
		log.Printf("error creating expense: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// List handles GET /expenses.
func (h *ExpenseHandler) List(w http.ResponseWriter, r *http.Request) {
	expenses, err := h.store.List()
	if err != nil {
		log.Printf("error listing expenses: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, expenses)
}

// Get handles GET /expenses/{id}.
func (h *ExpenseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	expense, err := h.store.GetByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "expense not found")
			return
		}
		log.Printf("error getting expense %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, expense)
}

// Update handles PUT /expenses/{id}.
func (h *ExpenseHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if !checkContentType(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var req model.UpdateExpenseRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.store.Update(id, model.Expense{
		Amount:      req.Amount,
		Description: req.Description,
		Category:    req.Category,
		Date:        req.Date,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "expense not found")
			return
		}
		log.Printf("error updating expense %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /expenses/{id}.
func (h *ExpenseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.store.Delete(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "expense not found")
			return
		}
		log.Printf("error deleting expense %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
