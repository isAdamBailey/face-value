package httpapi

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/isAdamBailey/face-value/backend/internal/db"
	"github.com/isAdamBailey/face-value/backend/internal/storage"
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
