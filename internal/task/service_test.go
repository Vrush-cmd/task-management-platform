package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const unknownID = "11111111-1111-4111-8111-111111111111"

// newTestService returns a Service with a fixed clock and predictable IDs.
func newTestService(repo Repository) (*Service, *time.Time) {
	clock := time.Date(2026, 10, 30, 20, 0, 0, 0, time.UTC)
	svc := NewService(repo)
	svc.now = func() time.Time { return clock }
	n := 0
	svc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("00000000-0000-4000-8000-%012d", n), nil
	}
	return svc, &clock
}

func TestCreate(t *testing.T) {
	svc, clock := newTestService(newFakeRepo())

	got, err := svc.Create(context.Background(), Input{
		Title:       "  Finish Go task manager  ",
		Description: "Implement the basic CRUD API",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != "00000000-0000-4000-8000-000000000001" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Title != "Finish Go task manager" {
		t.Errorf("title not trimmed: %q", got.Title)
	}
	if got.Description != "Implement the basic CRUD API" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Completed {
		t.Error("new task should not be completed")
	}
	if !got.CreatedAt.Equal(*clock) || !got.UpdatedAt.Equal(*clock) {
		t.Errorf("timestamps = %v / %v, want %v", got.CreatedAt, got.UpdatedAt, *clock)
	}
}

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"empty", "", true},
		{"whitespace only", "   \t ", true},
		{"one character", "a", false},
		{"exactly 200 characters", strings.Repeat("a", 200), false},
		{"201 characters", strings.Repeat("a", 201), true},
		{"200 multi-byte characters", strings.Repeat("é", 200), false},
		{"201 multi-byte characters", strings.Repeat("é", 201), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestService(newFakeRepo())
			_, err := svc.Create(context.Background(), Input{Title: tc.title})

			var vErr *ValidationError
			if tc.wantErr && !errors.As(err, &vErr) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetAndList(t *testing.T) {
	svc, clock := newTestService(newFakeRepo())
	ctx := context.Background()

	empty, err := svc.List(ctx)
	if err != nil || len(empty) != 0 {
		t.Fatalf("List on empty repo = %v, %v", empty, err)
	}

	first, _ := svc.Create(ctx, Input{Title: "first"})
	*clock = clock.Add(time.Minute)
	second, _ := svc.Create(ctx, Input{Title: "second"})

	got, err := svc.Get(ctx, first.ID)
	if err != nil || got.Title != "first" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Errorf("List order = %+v, want newest first", list)
	}
}

func TestUpdate(t *testing.T) {
	svc, clock := newTestService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, Input{Title: "old", Description: "old desc"})

	*clock = clock.Add(time.Hour)
	got, err := svc.Update(ctx, created.ID, Input{Title: "new", Description: ""})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Title != "new" || got.Description != "" {
		t.Errorf("Update result = %+v", got)
	}
	if !got.UpdatedAt.Equal(*clock) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, *clock)
	}

	if _, err := svc.Update(ctx, created.ID, Input{Title: ""}); err == nil {
		t.Error("Update with empty title should fail")
	}
	if _, err := svc.Update(ctx, unknownID, Input{Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update unknown id: err = %v, want ErrNotFound", err)
	}
}

func TestToggleCompleted(t *testing.T) {
	svc, clock := newTestService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, Input{Title: "toggle me"})

	*clock = clock.Add(time.Minute)
	done, err := svc.ToggleCompleted(ctx, created.ID)
	if err != nil || !done.Completed {
		t.Fatalf("first toggle = %+v, %v; want completed", done, err)
	}
	if !done.UpdatedAt.Equal(*clock) {
		t.Errorf("UpdatedAt not refreshed: %v", done.UpdatedAt)
	}

	undone, err := svc.ToggleCompleted(ctx, created.ID)
	if err != nil || undone.Completed {
		t.Fatalf("second toggle = %+v, %v; want incomplete", undone, err)
	}

	if _, err := svc.ToggleCompleted(ctx, unknownID); !errors.Is(err, ErrNotFound) {
		t.Errorf("toggle unknown id: err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	svc, _ := newTestService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, Input{Title: "delete me"})

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after delete: err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete: err = %v, want ErrNotFound", err)
	}
}

func TestMalformedIDs(t *testing.T) {
	svc, _ := newTestService(newFakeRepo())
	ctx := context.Background()

	for _, id := range []string{"", "abc", "not-a-uuid", "1111111-1111-4111-8111-111111111111", "zzzzzzzz-1111-4111-8111-111111111111"} {
		if _, err := svc.Get(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Get(%q): err = %v, want ErrInvalidID", id, err)
		}
		if _, err := svc.Update(ctx, id, Input{Title: "x"}); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Update(%q): err = %v, want ErrInvalidID", id, err)
		}
		if _, err := svc.ToggleCompleted(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("ToggleCompleted(%q): err = %v, want ErrInvalidID", id, err)
		}
		if err := svc.Delete(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Delete(%q): err = %v, want ErrInvalidID", id, err)
		}
	}
}

func TestRepositoryErrorsPropagate(t *testing.T) {
	repo := newFakeRepo()
	repo.err = errors.New("db down")
	svc, _ := newTestService(repo)

	if _, err := svc.Create(context.Background(), Input{Title: "x"}); err == nil {
		t.Error("expected repository error from Create")
	}
	if _, err := svc.List(context.Background()); err == nil {
		t.Error("expected repository error from List")
	}
}

func TestNewUUID(t *testing.T) {
	a, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newUUID()
	if a == b {
		t.Error("two generated UUIDs were identical")
	}
	if !isValidUUID(a) {
		t.Errorf("generated UUID %q is not valid", a)
	}
	if a[14] != '4' {
		t.Errorf("UUID %q is not version 4", a)
	}
}
