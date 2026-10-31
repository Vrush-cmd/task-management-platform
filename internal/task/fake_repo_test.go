package task

import (
	"context"
	"sort"
	"sync"
	"time"
)

// fakeRepo is an in-memory Repository used by the service and handler tests.
type fakeRepo struct {
	mu    sync.Mutex
	tasks map[string]Task
	err   error // when set, every method returns it
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tasks: map[string]Task{}}
}

func (f *fakeRepo) Create(_ context.Context, t Task) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.tasks[t.ID] = t
	return nil
}

func (f *fakeRepo) List(_ context.Context) ([]Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := make([]Task, 0, len(f.tasks))
	for _, t := range f.tasks {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Task{}, f.err
	}
	t, ok := f.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) Update(_ context.Context, id, title, description string, updatedAt time.Time) (Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Task{}, f.err
	}
	t, ok := f.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	t.Title, t.Description, t.UpdatedAt = title, description, updatedAt
	f.tasks[id] = t
	return t, nil
}

func (f *fakeRepo) ToggleCompleted(_ context.Context, id string, updatedAt time.Time) (Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Task{}, f.err
	}
	t, ok := f.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	t.Completed, t.UpdatedAt = !t.Completed, updatedAt
	f.tasks[id] = t
	return t, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(f.tasks, id)
	return nil
}
