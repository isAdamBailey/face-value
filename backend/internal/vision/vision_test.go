package vision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const cleanJSON = `{"title":"Sony TC-377 reel-to-reel tape deck","brand":"Sony","model":"TC-377","category":"audio","condition_notes":"visible wear on case","search_query":"Sony TC-377 reel-to-reel","keywords":["tape deck","reel-to-reel"],"confidence":0.82}`

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c := NewClient(srv.URL, "test-token", "test-model")
	c.SetHTTPClient(srv.Client())
	return c
}

func chatResponseBody(content string) []byte {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	})
	return b
}

func assertIdentification(t *testing.T, ident Identification) {
	t.Helper()
	if ident.Brand != "Sony" {
		t.Errorf("Brand = %q, want Sony", ident.Brand)
	}
	if ident.SearchQuery != "Sony TC-377 reel-to-reel" {
		t.Errorf("SearchQuery = %q", ident.SearchQuery)
	}
	if ident.Confidence != 0.82 {
		t.Errorf("Confidence = %v, want 0.82", ident.Confidence)
	}
}

func TestIdentify_CleanJSON(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write(chatResponseBody(cleanJSON))
	})

	ident, model, err := newClient(t, srv).Identify(context.Background(), []byte("fake-image-bytes"), "image/jpeg")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if model != "test-model" {
		t.Errorf("model = %q, want test-model", model)
	}
	assertIdentification(t, ident)
}

func TestIdentify_FencedJSON(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(chatResponseBody("```json\n" + cleanJSON + "\n```"))
	})

	ident, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	assertIdentification(t, ident)
}

func TestIdentify_ProseWrappedJSON(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(chatResponseBody("Sure, here is the identification:\n" + cleanJSON + "\nLet me know if you need anything else."))
	})

	ident, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	assertIdentification(t, ident)
}

func TestIdentify_MalformedThenRetrySucceeds(t *testing.T) {
	var calls int32
	var sawRetryInstruction bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)

		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		if n == 2 {
			last := body.Messages[len(body.Messages)-1]
			if s, ok := last.Content.(string); ok && last.Role == "user" &&
				(len(s) > 0) {
				sawRetryInstruction = true
			}
			_, _ = w.Write(chatResponseBody(cleanJSON))
			return
		}

		_, _ = w.Write(chatResponseBody(`{"title": "oops, not valid json`))
	})

	ident, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	assertIdentification(t, ident)
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (initial + retry)", calls)
	}
	if !sawRetryInstruction {
		t.Errorf("retry request did not include a follow-up user message")
	}
}

func TestIdentify_MalformedTwiceIsHardError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(chatResponseBody("not json at all, no braces here"))
	})

	_, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg")
	if err == nil {
		t.Fatal("Identify: want error after two malformed responses, got nil")
	}
}

func TestIdentification_LowConfidence(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
		ID   Identification
	}{
		{"confident with query", true, Identification{Confidence: 0.8, SearchQuery: "brand model"}},
		{"below threshold", false, Identification{Confidence: 0.34, SearchQuery: "brand model"}},
		{"at threshold", true, Identification{Confidence: 0.35, SearchQuery: "brand model"}},
		{"empty query despite high confidence", false, Identification{Confidence: 0.9, SearchQuery: ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := !tc.ID.LowConfidence(); got != tc.ok {
				t.Errorf("LowConfidence() = %v, want ok=%v", tc.ID.LowConfidence(), tc.ok)
			}
		})
	}
}
