package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository is the persistence contract used by the Service.
type Repository interface {
	Create(ctx context.Context, t Task) error
	List(ctx context.Context) ([]Task, error)
	Get(ctx context.Context, id string) (Task, error)
	Update(ctx context.Context, id, title, description string, updatedAt time.Time) (Task, error)
	ToggleCompleted(ctx context.Context, id string, updatedAt time.Time) (Task, error)
	Delete(ctx context.Context, id string) error
}

// PostgresRepository stores tasks in PostgreSQL using database/sql.
// All queries are parameterised.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository returns a repository backed by db.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// id is cast to text so it always scans into a Go string, whatever the driver.
const taskColumns = `id::text, title, description, completed, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("scan task: %w", err)
	}
	return t, nil
}

// Create inserts a new task.
func (r *PostgresRepository) Create(ctx context.Context, t Task) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (id, title, description, completed, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.Title, t.Description, t.Completed, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// List returns all tasks, newest first.
func (r *PostgresRepository) List(ctx context.Context) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

// Get returns one task by ID, or ErrNotFound.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Task, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id)
	return scanTask(row)
}

// Update replaces a task's title and description, or returns ErrNotFound.
func (r *PostgresRepository) Update(ctx context.Context, id, title, description string, updatedAt time.Time) (Task, error) {
	row := r.db.QueryRowContext(ctx,
		`UPDATE tasks SET title = $2, description = $3, updated_at = $4
		 WHERE id = $1
		 RETURNING `+taskColumns,
		id, title, description, updatedAt)
	return scanTask(row)
}

// ToggleCompleted flips the completed flag in a single atomic statement.
func (r *PostgresRepository) ToggleCompleted(ctx context.Context, id string, updatedAt time.Time) (Task, error) {
	row := r.db.QueryRowContext(ctx,
		`UPDATE tasks SET completed = NOT completed, updated_at = $2
		 WHERE id = $1
		 RETURNING `+taskColumns,
		id, updatedAt)
	return scanTask(row)
}

// Delete removes a task, or returns ErrNotFound.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
