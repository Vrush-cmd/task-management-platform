package task

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) (http.Handler, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	return mux, repo
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeTask(t *testing.T, rec *httptest.ResponseRecorder) Task {
	t.Helper()
	var got Task
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode task from %q: %v", rec.Body.String(), err)
	}
	return got
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("error Content-Type = %q, want application/json", ct)
	}
	var e errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return e.Error
}

func TestHandlerCreate(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/api/v1/tasks",
		`{"title":"Finish Go task manager","description":"Implement the basic CRUD API"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body)
	}
	got := decodeTask(t, rec)
	if got.ID == "" || got.Title != "Finish Go task manager" ||
		got.Description != "Implement the basic CRUD API" || got.Completed {
		t.Errorf("unexpected task: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: %+v", got)
	}
}

func TestHandlerCreateWithoutDescription(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"just a title"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := decodeTask(t, rec); got.Description != "" {
		t.Errorf("Description = %q, want empty", got.Description)
	}
}

func TestHandlerCreateBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"malformed JSON", `{"title":`},
		{"wrong type", `{"title": 42}`},
		{"unknown field", `{"title":"x","priority":"high"}`},
		{"trailing data", `{"title":"x"} {"title":"y"}`},
		{"missing title", `{"description":"no title"}`},
		{"blank title", `{"title":"   "}`},
		{"title too long", `{"title":"` + strings.Repeat("a", 201) + `"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := newTestHandler(t)
			rec := do(t, h, http.MethodPost, "/api/v1/tasks", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body)
			}
			if msg := errorMessage(t, rec); msg == "" {
				t.Error("error message is empty")
			}
			if len(repo.tasks) != 0 {
				t.Error("invalid request must not create a task")
			}
		})
	}
}

func TestHandlerCreateBodyTooLarge(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"title":"x","description":"` + strings.Repeat("a", maxBodyBytes) + `"}`
	rec := do(t, h, http.MethodPost, "/api/v1/tasks", body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestHandlerList(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := do(t, h, http.MethodGet, "/api/v1/tasks", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("empty list body = %q, want []", got)
	}

	do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"one"}`)
	do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"two"}`)

	rec = do(t, h, http.MethodGet, "/api/v1/tasks", "")
	var list []Task
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("len(list) = %d, want 2", len(list))
	}
}

func TestHandlerGet(t *testing.T) {
	h, _ := newTestHandler(t)
	created := decodeTask(t, do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"find me"}`))

	rec := do(t, h, http.MethodGet, "/api/v1/tasks/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := decodeTask(t, rec); got.ID != created.ID || got.Title != "find me" {
		t.Errorf("got %+v", got)
	}

	rec = do(t, h, http.MethodGet, "/api/v1/tasks/"+unknownID, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: status = %d, want 404", rec.Code)
	}
	errorMessage(t, rec)

	rec = do(t, h, http.MethodGet, "/api/v1/tasks/not-a-uuid", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed id: status = %d, want 400", rec.Code)
	}
	errorMessage(t, rec)
}

func TestHandlerUpdate(t *testing.T) {
	h, _ := newTestHandler(t)
	created := decodeTask(t, do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"old","description":"old"}`))
	path := "/api/v1/tasks/" + created.ID

	rec := do(t, h, http.MethodPut, path, `{"title":"new","description":"new desc"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if got := decodeTask(t, rec); got.Title != "new" || got.Description != "new desc" {
		t.Errorf("got %+v", got)
	}

	if rec := do(t, h, http.MethodPut, path, `{"title":""}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty title: status = %d, want 400", rec.Code)
	}
	if rec := do(t, h, http.MethodPut, "/api/v1/tasks/"+unknownID, `{"title":"x"}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: status = %d, want 404", rec.Code)
	}
	if rec := do(t, h, http.MethodPut, "/api/v1/tasks/nope", `{"title":"x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed id: status = %d, want 400", rec.Code)
	}
}

func TestHandlerToggleComplete(t *testing.T) {
	h, _ := newTestHandler(t)
	created := decodeTask(t, do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"toggle"}`))
	path := "/api/v1/tasks/" + created.ID + "/complete"

	rec := do(t, h, http.MethodPatch, path, "")
	if rec.Code != http.StatusOK || !decodeTask(t, rec).Completed {
		t.Fatalf("first toggle: status %d body %s", rec.Code, rec.Body)
	}
	rec = do(t, h, http.MethodPatch, path, "")
	if rec.Code != http.StatusOK || decodeTask(t, rec).Completed {
		t.Fatalf("second toggle: status %d body %s", rec.Code, rec.Body)
	}

	if rec := do(t, h, http.MethodPatch, "/api/v1/tasks/"+unknownID+"/complete", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: status = %d, want 404", rec.Code)
	}
}

func TestHandlerDelete(t *testing.T) {
	h, _ := newTestHandler(t)
	created := decodeTask(t, do(t, h, http.MethodPost, "/api/v1/tasks", `{"title":"bye"}`))
	path := "/api/v1/tasks/" + created.ID

	rec := do(t, h, http.MethodDelete, path, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 response has a body: %q", rec.Body)
	}
	if rec := do(t, h, http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: status = %d, want 404", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, path, ""); rec.Code != http.StatusNotFound {
		t.Errorf("second delete: status = %d, want 404", rec.Code)
	}
}

func TestHandlerRoutingErrors(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := do(t, h, http.MethodDelete, "/api/v1/tasks", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE collection: status = %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") == "" {
		t.Error("405 response is missing the Allow header")
	}
	errorMessage(t, rec)

	if rec := do(t, h, http.MethodPost, "/api/v1/tasks/"+unknownID+"/complete", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST complete: status = %d, want 405", rec.Code)
	}

	rec = do(t, h, http.MethodGet, "/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown route: status = %d, want 404", rec.Code)
	}
	errorMessage(t, rec)
}

func TestHandlerInternalErrorHidesDetails(t *testing.T) {
	h, repo := newTestHandler(t)
	repo.err = errors.New("password authentication failed for user taskflow")

	rec := do(t, h, http.MethodGet, "/api/v1/tasks", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if msg := errorMessage(t, rec); strings.Contains(msg, "password") {
		t.Errorf("internal error leaked to client: %q", msg)
	}
}
