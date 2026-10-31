package task

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Service applies task rules and coordinates repository calls.
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() (string, error)
}

// NewService returns a Service that stores tasks in repo.
func NewService(repo Repository) *Service {
	return &Service{
		repo:  repo,
		now:   func() time.Time { return time.Now().UTC() },
		newID: newUUID,
	}
}

// Create validates the input and stores a new, incomplete task.
func (s *Service) Create(ctx context.Context, in Input) (Task, error) {
	in, err := validate(in)
	if err != nil {
		return Task{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Task{}, fmt.Errorf("generate id: %w", err)
	}

	now := s.now()
	t := Task{
		ID:          id,
		Title:       in.Title,
		Description: in.Description,
		Completed:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return Task{}, err
	}
	return t, nil
}

// List returns all tasks.
func (s *Service) List(ctx context.Context) ([]Task, error) {
	return s.repo.List(ctx)
}

// Get returns one task.
func (s *Service) Get(ctx context.Context, id string) (Task, error) {
	id, err := normalizeID(id)
	if err != nil {
		return Task{}, err
	}
	return s.repo.Get(ctx, id)
}

// Update replaces a task's title and description.
func (s *Service) Update(ctx context.Context, id string, in Input) (Task, error) {
	id, err := normalizeID(id)
	if err != nil {
		return Task{}, err
	}
	in, err = validate(in)
	if err != nil {
		return Task{}, err
	}
	return s.repo.Update(ctx, id, in.Title, in.Description, s.now())
}

// ToggleCompleted flips a task between complete and incomplete.
func (s *Service) ToggleCompleted(ctx context.Context, id string) (Task, error) {
	id, err := normalizeID(id)
	if err != nil {
		return Task{}, err
	}
	return s.repo.ToggleCompleted(ctx, id, s.now())
}

// Delete removes a task.
func (s *Service) Delete(ctx context.Context, id string) error {
	id, err := normalizeID(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// validate trims the input and enforces the title rules (1–200 characters).
func validate(in Input) (Input, error) {
	title := strings.TrimSpace(in.Title)
	switch n := utf8.RuneCountInString(title); {
	case n == 0:
		return Input{}, &ValidationError{Message: "title is required"}
	case n > MaxTitleLength:
		return Input{}, &ValidationError{
			Message: fmt.Sprintf("title must be at most %d characters (got %d)", MaxTitleLength, n),
		}
	}
	return Input{Title: title, Description: strings.TrimSpace(in.Description)}, nil
}

func normalizeID(id string) (string, error) {
	if !isValidUUID(id) {
		return "", ErrInvalidID
	}
	return strings.ToLower(id), nil
}
