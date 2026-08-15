package store

import (
	"database/sql"
	"errors"

	"expense-tracker/internal/model"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("expense not found")

type ExpenseStore struct {
	db *sql.DB
}

func NewExpenseStore(db *sql.DB) *ExpenseStore {
	return &ExpenseStore{db: db}
}

func (s *ExpenseStore) Create(expense model.Expense) (model.Expense, error) {
	if expense.ID == "" {
		expense.ID = uuid.New().String()
	}
	query := `INSERT INTO expenses (id, amount, description, category, date) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, expense.ID, expense.Amount, expense.Description, expense.Category, expense.Date)
	if err != nil {
		return model.Expense{}, err
	}
	return expense, nil
}

func (s *ExpenseStore) List() ([]model.Expense, error) {
	query := `SELECT id, amount, description, category, date FROM expenses ORDER BY date DESC, id ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expenses := make([]model.Expense, 0)
	for rows.Next() {
		var e model.Expense
		if err := rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Category, &e.Date); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return expenses, nil
}

func (s *ExpenseStore) GetByID(id string) (model.Expense, error) {
	query := `SELECT id, amount, description, category, date FROM expenses WHERE id = ?`
	var e model.Expense
	err := s.db.QueryRow(query, id).Scan(&e.ID, &e.Amount, &e.Description, &e.Category, &e.Date)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Expense{}, ErrNotFound
		}
		return model.Expense{}, err
	}
	return e, nil
}

func (s *ExpenseStore) Update(id string, expense model.Expense) (model.Expense, error) {
	query := `UPDATE expenses SET amount = ?, description = ?, category = ?, date = ? WHERE id = ?`
	res, err := s.db.Exec(query, expense.Amount, expense.Description, expense.Category, expense.Date, id)
	if err != nil {
		return model.Expense{}, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return model.Expense{}, err
	}
	if rowsAffected == 0 {
		return model.Expense{}, ErrNotFound
	}
	expense.ID = id
	return expense, nil
}

func (s *ExpenseStore) Delete(id string) error {
	query := `DELETE FROM expenses WHERE id = ?`
	res, err := s.db.Exec(query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
