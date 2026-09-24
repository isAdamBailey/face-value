package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
	c := NewClient("test-key", "test-model")
	c.baseURL = srv.URL
	c.httpClient = srv.Client()
	return c
}

func messageResponseBody(stopReason, text string) []byte {
	b, _ := json.Marshal(map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "test-model",
		"stop_reason": stopReason,
		"content":     []map[string]any{{"type": "text", "text": text}},
		"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
	})
	return b
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
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

func TestIdentify_Success(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q, want /v1/messages", r.URL.Path)
		}
		if got := r.Header.Get("X-Api-Key"); got != "test-key" {
			t.Errorf("X-Api-Key = %q", got)
		}
		if got := r.Header.Get("Anthropic-Version"); got != anthropicVersion {
			t.Errorf("Anthropic-Version = %q", got)
		}

		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					Type   string `json:"type"`
					Source struct {
						MediaType string `json:"media_type"`
						Data      string `json:"data"`
					} `json:"source"`
				} `json:"content"`
			} `json:"messages"`
			OutputConfig struct {
				Format struct {
					Type string `json:"type"`
				} `json:"format"`
			} `json:"output_config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "test-model" {
			t.Errorf("model = %q", body.Model)
		}
		if body.OutputConfig.Format.Type != "json_schema" {
			t.Errorf("output_config.format.type = %q, want json_schema", body.OutputConfig.Format.Type)
		}
		img := body.Messages[0].Content[0]
		if img.Type != "image" || img.Source.MediaType != "image/jpeg" ||
			img.Source.Data != base64.StdEncoding.EncodeToString([]byte("fake-image-bytes")) {
			t.Errorf("first content block = %+v, want base64 jpeg image", img)
		}

		writeJSON(w, http.StatusOK, messageResponseBody("end_turn", cleanJSON))
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

func TestIdentify_Errors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   []byte
	}{
		{"truncated at max_tokens", http.StatusOK, messageResponseBody("max_tokens", `{"title": "Sony`)},
		{"refusal", http.StatusOK, messageResponseBody("refusal", "")},
		{"invalid json", http.StatusOK, messageResponseBody("end_turn", "not json")},
		{"empty text", http.StatusOK, messageResponseBody("end_turn", "")},
		{"api error", http.StatusUnauthorized, []byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, tc.status, tc.body)
			})

			if _, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg"); err == nil {
				t.Fatal("Identify: want error, got nil")
			}
		})
	}
}

func TestIdentify_ClampsConfidence(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, messageResponseBody("end_turn", strings.Replace(cleanJSON, `"confidence":0.82`, `"confidence":82`, 1)))
	})

	ident, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if ident.Confidence != 1 {
		t.Errorf("Confidence = %v, want clamped to 1", ident.Confidence)
	}
}

func TestIdentificationSchema_MatchesStruct(t *testing.T) {
	props := identificationSchema["properties"].(map[string]any)
	required := identificationSchema["required"].([]string)

	typ := reflect.TypeFor[Identification]()
	var tags []string
	for i := range typ.NumField() {
		tags = append(tags, strings.Split(typ.Field(i).Tag.Get("json"), ",")[0])
	}

	if !reflect.DeepEqual(required, tags) {
		t.Errorf("required = %v, want struct JSON tags %v", required, tags)
	}
	for _, tag := range tags {
		if _, ok := props[tag]; !ok {
			t.Errorf("schema properties missing %q", tag)
		}
	}
	if len(props) != len(tags) {
		t.Errorf("schema has %d properties, struct has %d fields", len(props), len(tags))
	}
}

func TestIdentify_RetriesTransientFailureOnce(t *testing.T) {
	var calls int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			writeJSON(w, 529, []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
			return
		}
		writeJSON(w, http.StatusOK, messageResponseBody("end_turn", cleanJSON))
	})

	ident, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg")
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	assertIdentification(t, ident)
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestIdentify_DoesNotRetryClientError(t *testing.T) {
	var calls int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeJSON(w, http.StatusBadRequest, []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`))
	})

	if _, _, err := newClient(t, srv).Identify(context.Background(), []byte("img"), "image/jpeg"); err == nil {
		t.Fatal("Identify: want error, got nil")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 4xx)", calls)
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
