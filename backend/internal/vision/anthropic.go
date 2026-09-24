package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Plain net/http rather than the official SDK: the SDK is a single very
// large generated package whose compile peaks at ~1.7GB RSS, which OOMs the
// production VPS where Forge runs `go build`.

const (
	defaultBaseURL   = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"

	// maxTokens comfortably covers one Identification object; hitting it
	// means the JSON was truncated.
	maxTokens = 1024

	// requestTimeout and maxRetries bound each call so a stalled attempt
	// plus one retry fits inside the pipeline's 60s vision timeout.
	requestTimeout = 25 * time.Second
	maxRetries     = 1
	maxRetryDelay  = 5 * time.Second

	// temperature is kept low so the same photo produces the same search
	// query. Haiku 4.5 accepts it; newer models (Sonnet 5, Opus 5) reject
	// sampling parameters, so drop it if ANTHROPIC_VISION_MODEL moves to
	// one of those.
	temperature = 0.1
)

// identificationSchema constrains the response to Identification via
// structured outputs. TestIdentificationSchema_MatchesStruct keeps it in
// sync with the struct's JSON tags.
var identificationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title":           map[string]any{"type": "string"},
		"brand":           map[string]any{"type": "string"},
		"model":           map[string]any{"type": "string"},
		"category":        map[string]any{"type": "string"},
		"condition_notes": map[string]any{"type": "string"},
		"search_query":    map[string]any{"type": "string"},
		"keywords":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"confidence":      map[string]any{"type": "number"},
	},
	"required": []string{
		"title", "brand", "model", "category", "condition_notes",
		"search_query", "keywords", "confidence",
	},
	"additionalProperties": false,
}

// Client identifies items via the Anthropic Messages API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
}

// NewClient returns a Client for the given API key and model ID. The model
// ID always comes from config (ANTHROPIC_VISION_MODEL) — never hardcode one
// here.
func NewClient(apiKey, model string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		model:      model,
	}
}

type contentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *imageSource `json:"source,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type message struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type outputConfig struct {
	Format struct {
		Type   string         `json:"type"`
		Schema map[string]any `json:"schema"`
	} `json:"format"`
}

type messagesRequest struct {
	Model        string       `json:"model"`
	MaxTokens    int          `json:"max_tokens"`
	Temperature  float64      `json:"temperature"`
	System       string       `json:"system"`
	Messages     []message    `json:"messages"`
	OutputConfig outputConfig `json:"output_config"`
}

type messagesResponse struct {
	StopReason  string `json:"stop_reason"`
	StopDetails *struct {
		Category    string `json:"category"`
		Explanation string `json:"explanation"`
	} `json:"stop_details"`
	Content []contentBlock `json:"content"`
}

// Identify implements Provider.
func (c *Client) Identify(ctx context.Context, img []byte, mime string) (Identification, string, error) {
	req := messagesRequest{
		Model:       c.model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		System:      systemPrompt,
		Messages: []message{{
			Role: "user",
			Content: []contentBlock{
				{Type: "image", Source: &imageSource{
					Type:      "base64",
					MediaType: mime,
					Data:      base64.StdEncoding.EncodeToString(img),
				}},
				{Type: "text", Text: identifyPrompt},
			},
		}},
	}
	req.OutputConfig.Format.Type = "json_schema"
	req.OutputConfig.Format.Schema = identificationSchema

	body, err := json.Marshal(req)
	if err != nil {
		return Identification{}, c.model, fmt.Errorf("identify: encode request: %w", err)
	}

	resp, err := c.send(ctx, body)
	if err != nil {
		return Identification{}, c.model, fmt.Errorf("identify: %w", err)
	}

	if resp.StopReason == "refusal" {
		var category, explanation string
		if resp.StopDetails != nil {
			category, explanation = resp.StopDetails.Category, resp.StopDetails.Explanation
		}
		return Identification{}, c.model, fmt.Errorf("identify: model refused (category %q): %s", category, explanation)
	}
	if resp.StopReason != "end_turn" {
		return Identification{}, c.model, fmt.Errorf("identify: unexpected stop reason %q", resp.StopReason)
	}

	var raw string
	for _, block := range resp.Content {
		if block.Type == "text" {
			raw += block.Text
		}
	}

	var ident Identification
	if err := json.Unmarshal([]byte(raw), &ident); err != nil {
		return Identification{}, c.model, fmt.Errorf("identify: decode model output: %w", err)
	}

	// Structured outputs can't enforce a numeric range, and
	// searches.confidence is NUMERIC(3,2).
	ident.Confidence = min(max(ident.Confidence, 0), 1)

	return ident, c.model, nil
}

// retryableError marks a failure worth one more attempt (network error,
// 429, 5xx/529 overloaded), carrying any server-requested delay.
type retryableError struct {
	err   error
	delay time.Duration
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// send POSTs to /v1/messages, retrying transient failures up to maxRetries
// times.
func (c *Client) send(ctx context.Context, body []byte) (messagesResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.post(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		var re *retryableError
		if !errors.As(err, &re) || attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return messagesResponse{}, ctx.Err()
		case <-time.After(re.delay):
		}
	}
	return messagesResponse{}, lastErr
}

func (c *Client) post(ctx context.Context, body []byte) (messagesResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return messagesResponse{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return messagesResponse{}, ctx.Err()
		}
		return messagesResponse{}, &retryableError{err: fmt.Errorf("request: %w", err), delay: time.Second}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		err := fmt.Errorf("anthropic: unexpected status %d: %s", resp.StatusCode, string(b))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return messagesResponse{}, &retryableError{err: err, delay: retryDelay(resp.Header.Get("retry-after"))}
		}
		return messagesResponse{}, err
	}

	var mr messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return messagesResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return mr, nil
}

// retryDelay honors a retry-after header in seconds, capped so a retry
// still fits inside the pipeline's vision timeout.
func retryDelay(header string) time.Duration {
	secs, err := strconv.Atoi(header)
	if err != nil || secs < 1 {
		return time.Second
	}
	return min(time.Duration(secs)*time.Second, maxRetryDelay)
}
