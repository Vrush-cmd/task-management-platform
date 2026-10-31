package task

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"taskflow/internal/database"
)

// TestPostgresRepository runs against a real PostgreSQL database. It is skipped
// unless TEST_DATABASE_URL is set and the migration has been applied, e.g.:
//
//	docker compose up -d db
//	TEST_DATABASE_URL="postgres://taskflow:taskflow@localhost:5432/taskflow?sslmode=disable" go test ./...
func TestPostgresRepository(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL repository test")
	}

	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewPostgresRepository(db)
	svc := NewService(repo)

	created, err := svc.Create(ctx, Input{Title: "repo test", Description: "stored in postgres"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), created.ID) })

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID || got.Title != "repo test" || got.Description != "stored in postgres" || got.Completed {
		t.Errorf("Get = %+v", got)
	}
	if d := got.CreatedAt.Sub(created.CreatedAt); d < -time.Millisecond || d > time.Millisecond {
		t.Errorf("CreatedAt round trip off by %v", d)
	}

	updated, err := svc.Update(ctx, created.ID, Input{Title: "renamed", Description: ""})
	if err != nil || updated.Title != "renamed" || updated.Description != "" {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) && !updated.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("UpdatedAt went backwards: %v -> %v", created.UpdatedAt, updated.UpdatedAt)
	}

	toggled, err := svc.ToggleCompleted(ctx, created.ID)
	if err != nil || !toggled.Completed {
		t.Fatalf("ToggleCompleted = %+v, %v", toggled, err)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, task := range list {
		if task.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Error("created task missing from List")
	}

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
