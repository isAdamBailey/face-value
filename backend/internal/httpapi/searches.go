package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/isAdamBailey/face-value/backend/internal/db"
	"github.com/isAdamBailey/face-value/backend/internal/storage"
)

const (
	defaultListLimit = 24
	maxListLimit     = 100
)

// createSearchResponse is the JSON body returned by POST /api/searches.
type createSearchResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// createSearch accepts a multipart image upload, preprocesses and stores
// it, inserts a pending search row, and kicks off the appraisal pipeline
// in the background. The full round trip is 5-20s, so this responds 202
// immediately rather than blocking the request.
func (h *Handler) createSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)
	if err := r.ParseMultipartForm(h.maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "image exceeds the upload size limit")
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image field is required")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read uploaded image")
		return
	}

	if _, err := storage.DetectContentType(data); err != nil {
		writeError(w, http.StatusBadRequest, "unsupported image type: only JPEG, PNG, and WEBP are accepted")
		return
	}

	processed, width, height, err := storage.Preprocess(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not process image")
		return
	}

	key, err := h.imageStore.Put(r.Context(), bytes.NewReader(processed), "image/jpeg")
	if err != nil {
		log.Printf("httpapi: store image: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	imgWidth := int32(width)
	imgHeight := int32(height)
	search, err := h.queries.CreateSearch(r.Context(), db.CreateSearchParams{
		UserEmail:   user.Email,
		ImageKey:    key,
		ImageWidth:  &imgWidth,
		ImageHeight: &imgHeight,
	})
	if err != nil {
		log.Printf("httpapi: create search: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.appraisal.Start(db.FromUUID(search.ID), processed, "image/jpeg")

	writeJSON(w, http.StatusAccepted, createSearchResponse{
		ID:     db.FromUUID(search.ID).String(),
		Status: search.Status,
	})
}

// listSearchesResponse is the JSON body returned by GET /api/searches.
type listSearchesResponse struct {
	Items      []searchSummary `json:"items"`
	NextCursor *string         `json:"next_cursor,omitempty"`
}

// listSearches returns a keyset-paginated page of the user's searches for
// the home page grid.
func (h *Handler) listSearches(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	limit := defaultListLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > maxListLimit {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}

	// No cursor means "start of the list": a cursor timestamp far enough
	// in the future that every real row satisfies created_at < cursor.
	cursorCreatedAt := time.Now().Add(24 * time.Hour)
	var cursorID uuid.UUID
	if c := r.URL.Query().Get("cursor"); c != "" {
		t, id, err := decodeCursor(c)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursorCreatedAt, cursorID = t, id
	}

	rows, err := h.queries.ListSearchesForUser(r.Context(), db.ListSearchesForUserParams{
		UserEmail:       user.Email,
		Limit:           int32(limit),
		CursorCreatedAt: db.ToTimestamptz(cursorCreatedAt),
		CursorID:        db.ToUUID(cursorID),
	})
	if err != nil {
		log.Printf("httpapi: list searches: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]searchSummary, len(rows))
	for i, row := range rows {
		items[i] = h.searchSummaryFromRow(r.Context(), row)
	}

	resp := listSearchesResponse{Items: items}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		next := encodeCursor(last.CreatedAt.Time, db.FromUUID(last.ID))
		resp.NextCursor = &next
	}

	writeJSON(w, http.StatusOK, resp)
}

// getSearch returns full detail for one search, including comps. It's also
// the frontend's poll target while a search is still processing.
func (h *Handler) getSearch(w http.ResponseWriter, r *http.Request) {
	row, ok := h.loadOwnedSearch(w, r)
	if !ok {
		return
	}

	comps, err := h.queries.ListCompsBySearch(r.Context(), row.ID)
	if err != nil {
		log.Printf("httpapi: list comps for %s: %v", db.FromUUID(row.ID), err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, h.searchDetailFromRow(r.Context(), row, comps))
}

// rerunRequest is the request body for POST /api/searches/{id}/rerun.
type rerunRequest struct {
	SearchQuery string `json:"search_query"`
}

// rerunSearch re-prices a search against an edited query without touching
// the image or re-running vision identification — the fix for when vision
// gets a model number wrong.
func (h *Handler) rerunSearch(w http.ResponseWriter, r *http.Request) {
	row, ok := h.loadOwnedSearch(w, r)
	if !ok {
		return
	}

	var req rerunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := strings.TrimSpace(req.SearchQuery)
	if query == "" {
		writeError(w, http.StatusBadRequest, "search_query is required")
		return
	}

	updated, err := h.queries.UpdateSearchQuery(r.Context(), db.UpdateSearchQueryParams{
		ID:          row.ID,
		SearchQuery: &query,
	})
	if err != nil {
		log.Printf("httpapi: update search query for %s: %v", db.FromUUID(row.ID), err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.appraisal.Rerun(db.FromUUID(row.ID), query)

	writeJSON(w, http.StatusAccepted, createSearchResponse{
		ID:     db.FromUUID(updated.ID).String(),
		Status: updated.Status,
	})
}

// deleteSearch removes a search's row and its S3 object.
func (h *Handler) deleteSearch(w http.ResponseWriter, r *http.Request) {
	row, ok := h.loadOwnedSearch(w, r)
	if !ok {
		return
	}

	if err := h.imageStore.Delete(r.Context(), row.ImageKey); err != nil {
		log.Printf("httpapi: delete image %q for %s: %v", row.ImageKey, db.FromUUID(row.ID), err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.queries.DeleteSearch(r.Context(), row.ID); err != nil {
		log.Printf("httpapi: delete search %s: %v", db.FromUUID(row.ID), err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// loadOwnedSearch looks up the {id} path param and verifies it belongs to
// the authenticated user, writing an error response and returning
// ok=false if not. A search owned by someone else 404s rather than 403s,
// so its existence isn't confirmed to a caller who shouldn't see it.
func (h *Handler) loadOwnedSearch(w http.ResponseWriter, r *http.Request) (db.Search, bool) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return db.Search{}, false
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "search not found")
		return db.Search{}, false
	}

	row, err := h.queries.GetSearch(r.Context(), db.ToUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "search not found")
		return db.Search{}, false
	}
	if err != nil {
		log.Printf("httpapi: get search %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return db.Search{}, false
	}
	if row.UserEmail != user.Email {
		writeError(w, http.StatusNotFound, "search not found")
		return db.Search{}, false
	}

	return row, true
}
