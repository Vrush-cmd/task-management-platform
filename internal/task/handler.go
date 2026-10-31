package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Handler exposes the task Service over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler that uses svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register attaches the task routes to mux (base path /api/v1).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks", h.create)
	mux.HandleFunc("GET /api/v1/tasks", h.list)
	mux.HandleFunc("GET /api/v1/tasks/{id}", h.get)
	mux.HandleFunc("PUT /api/v1/tasks/{id}", h.update)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}/complete", h.toggleComplete)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", h.delete)

	// Method-less patterns are less specific than the ones above, so they only
	// catch requests that use the wrong method on a known path.
	mux.HandleFunc("/api/v1/tasks", methodNotAllowed("GET, HEAD, POST"))
	mux.HandleFunc("/api/v1/tasks/{id}", methodNotAllowed("GET, HEAD, PUT, DELETE"))
	mux.HandleFunc("/api/v1/tasks/{id}/complete", methodNotAllowed("PATCH"))

	// Anything else gets a JSON 404 rather than the default plain-text one.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "route not found")
	})
}

type taskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeTaskRequest(w, r)
	if !ok {
		return
	}
	t, err := h.svc.Create(r.Context(), in)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.svc.List(r.Context())
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeTaskRequest(w, r)
	if !ok {
		return
	}
	t, err := h.svc.Update(r.Context(), r.PathValue("id"), in)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) toggleComplete(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.ToggleCompleted(r.Context(), r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeTaskRequest reads a JSON body into an Input. On failure it writes the
// error response itself and returns ok=false.
func decodeTaskRequest(w http.ResponseWriter, r *http.Request) (Input, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req taskRequest
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		case errors.Is(err, io.EOF):
			writeError(w, http.StatusBadRequest, "request body must not be empty")
		default:
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		}
		return Input{}, false
	}
	// Reject trailing data after the first JSON value.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return Input{}, false
	}
	return Input{Title: req.Title, Description: req.Description}, true
}

// handleError maps service errors to HTTP status codes and JSON bodies.
func handleError(w http.ResponseWriter, err error) {
	var validationErr *ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Message)
	case errors.Is(err, ErrInvalidID):
		writeError(w, http.StatusBadRequest, ErrInvalidID.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, ErrNotFound.Error())
	default:
		// Log the details, but never leak them to the client.
		slog.Error("internal error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}
