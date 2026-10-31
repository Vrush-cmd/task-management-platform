// Package task contains the task model, business rules, persistence and HTTP
// handlers for TaskFlow.
package task

import (
	"errors"
	"time"
)

// MaxTitleLength is the maximum number of characters allowed in a task title.
const MaxTitleLength = 200

var (
	// ErrNotFound is returned when no task exists with the requested ID.
	ErrNotFound = errors.New("task not found")
	// ErrInvalidID is returned when a task ID is not a well-formed UUID.
	ErrInvalidID = errors.New("invalid task id: must be a UUID")
)

// ValidationError describes invalid user input.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Task is a single item on the user's to-do list.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Input is the user-editable part of a task.
type Input struct {
	Title       string
	Description string
}
