package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	maxTokens   = 500
	temperature = 0.1
)

// Client identifies items via the Hugging Face Inference Providers router,
// using its OpenAI-compatible chat completions endpoint. Plain net/http —
// no vendor SDK.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	model      string
}

// NewClient returns a Client for the given router base URL, API token, and
// model ID. The model ID always comes from config (HF_VISION_MODEL) —
// never hardcode one here.
func NewClient(baseURL, token, model string) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		model:      model,
	}
}

// SetHTTPClient overrides the HTTP client used for requests. Intended for
// tests.
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imageURLContent struct {
	Type     string `json:"type"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

func newImageURLContent(dataURL string) imageURLContent {
	c := imageURLContent{Type: "image_url"}
	c.ImageURL.URL = dataURL
	return c
}

type chatRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	Messages    []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Identify implements Provider.
func (c *Client) Identify(ctx context.Context, img []byte, mime string) (Identification, string, error) {
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img)

	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: []any{
			textContent{Type: "text", Text: identifyPrompt},
			newImageURLContent(dataURL),
		}},
	}

	raw, err := c.chat(ctx, messages)
	if err != nil {
		return Identification{}, c.model, fmt.Errorf("identify: %w", err)
	}

	ident, parseErr := parseIdentification(raw)
	if parseErr == nil {
		return ident, c.model, nil
	}

	// Retry once, appending the raw output and asking for valid JSON per
	// spec §4.3. A second failure is a hard error.
	messages = append(messages,
		chatMessage{Role: "assistant", Content: raw},
		chatMessage{Role: "user", Content: "That was not valid JSON: " + raw + "\n\nReturn only valid JSON matching the schema."},
	)

	raw, err = c.chat(ctx, messages)
	if err != nil {
		return Identification{}, c.model, fmt.Errorf("identify (retry): %w", err)
	}

	ident, err = parseIdentification(raw)
	if err != nil {
		return Identification{}, c.model, fmt.Errorf("identify: model did not return valid JSON after retry: %w", err)
	}

	return ident, c.model, nil
}

func (c *Client) chat(ctx context.Context, messages []chatMessage) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:       c.model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Messages:    messages,
	})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("huggingface: unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("huggingface: no choices returned")
	}

	return cr.Choices[0].Message.Content, nil
}
