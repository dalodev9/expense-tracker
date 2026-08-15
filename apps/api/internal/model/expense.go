package model

import (
	"errors"
	"strings"
	"time"
)

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

// Validate validates CreateExpenseRequest according to the rules:
// - amount: Required. Must be a positive integer (> 0).
// - description: Required. Must be a non-empty string after trimming whitespace.
// - category: Required. Must be a non-empty string after trimming whitespace.
// - date: Required. Must parse successfully with time.Parse("2006-01-02", date).
// Returns the first validation error encountered.
func (r CreateExpenseRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if strings.TrimSpace(r.Description) == "" {
		return errors.New("description is required")
	}
	if strings.TrimSpace(r.Category) == "" {
		return errors.New("category is required")
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return errors.New("date must be in YYYY-MM-DD format")
	}
	return nil
}

// Validate validates UpdateExpenseRequest using the same rules as CreateExpenseRequest.
func (r UpdateExpenseRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if strings.TrimSpace(r.Description) == "" {
		return errors.New("description is required")
	}
	if strings.TrimSpace(r.Category) == "" {
		return errors.New("category is required")
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return errors.New("date must be in YYYY-MM-DD format")
	}
	return nil
}
