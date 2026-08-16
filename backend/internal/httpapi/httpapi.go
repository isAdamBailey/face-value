// Package httpapi wires up HTTP routes and handlers for the API.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler holds the dependencies needed to serve the API.
type Handler struct{}

// NewHandler constructs a Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Register attaches all API routes to the given router.
func (h *Handler) Register(r chi.Router) {
	r.Get("/healthz", healthz)
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
