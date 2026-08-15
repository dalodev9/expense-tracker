package model

import (
	"testing"
)

func TestCreateExpenseRequest_Validate(t *testing.T) {
	valid := CreateExpenseRequest{
		Amount:      100,
		Description: "Lunch",
		Category:    "Food",
		Date:        "2026-08-15",
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid request to pass, got: %v", err)
	}

	tests := []struct {
		name    string
		req     CreateExpenseRequest
		wantErr string
	}{
		{
			name:    "amount zero",
			req:     CreateExpenseRequest{Amount: 0, Description: "Lunch", Category: "Food", Date: "2026-08-15"},
			wantErr: "amount must be greater than zero",
		},
		{
			name:    "amount negative",
			req:     CreateExpenseRequest{Amount: -10, Description: "Lunch", Category: "Food", Date: "2026-08-15"},
			wantErr: "amount must be greater than zero",
		},
		{
			name:    "empty description",
			req:     CreateExpenseRequest{Amount: 100, Description: "   ", Category: "Food", Date: "2026-08-15"},
			wantErr: "description is required",
		},
		{
			name:    "empty category",
			req:     CreateExpenseRequest{Amount: 100, Description: "Lunch", Category: "  ", Date: "2026-08-15"},
			wantErr: "category is required",
		},
		{
			name:    "invalid date format",
			req:     CreateExpenseRequest{Amount: 100, Description: "Lunch", Category: "Food", Date: "15-08-2026"},
			wantErr: "date must be in YYYY-MM-DD format",
		},
		{
			name:    "invalid date day",
			req:     CreateExpenseRequest{Amount: 100, Description: "Lunch", Category: "Food", Date: "2026-02-30"},
			wantErr: "date must be in YYYY-MM-DD format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestUpdateExpenseRequest_Validate(t *testing.T) {
	valid := UpdateExpenseRequest{
		Amount:      200,
		Description: "Dinner",
		Category:    "Food",
		Date:        "2026-08-15",
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid request to pass, got: %v", err)
	}

	tests := []struct {
		name    string
		req     UpdateExpenseRequest
		wantErr string
	}{
		{
			name:    "amount zero",
			req:     UpdateExpenseRequest{Amount: 0, Description: "Dinner", Category: "Food", Date: "2026-08-15"},
			wantErr: "amount must be greater than zero",
		},
		{
			name:    "empty description",
			req:     UpdateExpenseRequest{Amount: 200, Description: "", Category: "Food", Date: "2026-08-15"},
			wantErr: "description is required",
		},
		{
			name:    "empty category",
			req:     UpdateExpenseRequest{Amount: 200, Description: "Dinner", Category: "", Date: "2026-08-15"},
			wantErr: "category is required",
		},
		{
			name:    "whitespace-only category",
			req:     UpdateExpenseRequest{Amount: 200, Description: "Dinner", Category: "   ", Date: "2026-08-15"},
			wantErr: "category is required",
		},
		{
			name:    "invalid date",
			req:     UpdateExpenseRequest{Amount: 200, Description: "Dinner", Category: "Food", Date: "invalid"},
			wantErr: "date must be in YYYY-MM-DD format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
