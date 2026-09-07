package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/isAdamBailey/face-value/backend/internal/db"
	"github.com/isAdamBailey/face-value/backend/internal/storage"
	"github.com/isAdamBailey/face-value/backend/internal/users"
)

// fakeQuerier is a minimal db.Querier covering only the search reads these
// tests exercise; anything else panics, which would mean a handler reached
// further than the test expects.
type fakeQuerier struct {
	db.Querier

	search db.Search
	rows   []db.Search

	listedAll   int
	listedEmail string
}

func (f *fakeQuerier) GetSearch(context.Context, pgtype.UUID) (db.Search, error) {
	return f.search, nil
}

func (f *fakeQuerier) ListSearches(context.Context, db.ListSearchesParams) ([]db.Search, error) {
	f.listedAll++
	return f.rows, nil
}

func (f *fakeQuerier) ListSearchesByUser(_ context.Context, arg db.ListSearchesByUserParams) ([]db.Search, error) {
	f.listedEmail = arg.UserEmail
	return f.rows, nil
}

func (f *fakeQuerier) ListCompsBySearch(context.Context, pgtype.UUID) ([]db.Comp, error) {
	return nil, nil
}

// fakeImageStore presigns to a fixed URL; the search handlers only ever
// call URL and Delete.
type fakeImageStore struct{ storage.ImageStore }

func (fakeImageStore) URL(context.Context, string) (string, error) {
	return "https://example.test/image.jpg", nil
}

func searchRow(id uuid.UUID, email string) db.Search {
	return db.Search{
		ID:        db.ToUUID(id),
		UserEmail: email,
		Status:    "complete",
		ImageKey:  "key",
		CreatedAt: db.ToTimestamptz(time.Now().UTC()),
	}
}

// request builds a request carrying user as the authenticated caller and,
// when id is non-empty, {id} as the chi path param.
func request(method, target string, user users.User, id string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	ctx := context.WithValue(r.Context(), userContextKey, user)

	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}

	return r.WithContext(ctx)
}

func TestGetSearchIsSharedAcrossUsers(t *testing.T) {
	id := uuid.New()
	h := &Handler{
		queries:    &fakeQuerier{search: searchRow(id, "owner@example.com")},
		imageStore: fakeImageStore{},
	}

	w := httptest.NewRecorder()
	h.getSearch(w, request(http.MethodGet, "/api/searches/"+id.String(),
		users.User{Email: "someone-else@example.com"}, id.String()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — searches are readable by every signed-in user", w.Code)
	}

	var got searchDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.UserEmail != "owner@example.com" {
		t.Errorf("user_email = %q, want the uploader's address", got.UserEmail)
	}
	if got.IsOwner {
		t.Error("is_owner = true for a reader who didn't upload the search")
	}
}

func TestGetSearchMarksTheUploaderAsOwner(t *testing.T) {
	id := uuid.New()
	h := &Handler{
		queries:    &fakeQuerier{search: searchRow(id, "owner@example.com")},
		imageStore: fakeImageStore{},
	}

	w := httptest.NewRecorder()
	h.getSearch(w, request(http.MethodGet, "/api/searches/"+id.String(),
		users.User{Email: "owner@example.com"}, id.String()))

	var got searchDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !got.IsOwner {
		t.Error("is_owner = false for the user who uploaded the search")
	}
}

func TestDeleteSearchRejectsNonOwner(t *testing.T) {
	id := uuid.New()
	h := &Handler{
		queries:    &fakeQuerier{search: searchRow(id, "owner@example.com")},
		imageStore: fakeImageStore{},
	}

	w := httptest.NewRecorder()
	h.deleteSearch(w, request(http.MethodDelete, "/api/searches/"+id.String(),
		users.User{Email: "someone-else@example.com"}, id.String()))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 — only the uploader may delete", w.Code)
	}
}

func TestListSearchesEmailFilter(t *testing.T) {
	q := &fakeQuerier{rows: []db.Search{searchRow(uuid.New(), "owner@example.com")}}
	h := &Handler{queries: q, imageStore: fakeImageStore{}}

	w := httptest.NewRecorder()
	h.listSearches(w, request(http.MethodGet, "/api/searches?email=%20Owner@Example.com%20",
		users.User{Email: "someone-else@example.com"}, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if q.listedEmail != "owner@example.com" {
		t.Errorf("filter = %q, want the normalized address", q.listedEmail)
	}
}

func TestListSearchesWithoutEmailFilterIsUnfiltered(t *testing.T) {
	q := &fakeQuerier{}
	h := &Handler{queries: q, imageStore: fakeImageStore{}}

	w := httptest.NewRecorder()
	h.listSearches(w, request(http.MethodGet, "/api/searches",
		users.User{Email: "someone@example.com"}, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if q.listedAll != 1 || q.listedEmail != "" {
		t.Errorf("listed all = %d, filtered = %q; want the unfiltered query so every user's searches are listed",
			q.listedAll, q.listedEmail)
	}
}
