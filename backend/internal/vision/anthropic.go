package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/invopop/jsonschema"
)

// maxTokens comfortably covers one Identification object; hitting it means
// the JSON was truncated.
const maxTokens = 1024

// requestTimeout and maxRetries bound the SDK's retry loop so a stalled
// attempt plus one retry fits inside the pipeline's 60s vision timeout.
const (
	requestTimeout = 25 * time.Second
	maxRetries     = 1
)

// temperature is kept low so the same photo produces the same search query.
// Haiku 4.5 accepts it; newer models (Sonnet 5, Opus 5) reject sampling
// parameters, so drop it if ANTHROPIC_VISION_MODEL moves to one of those.
const temperature = 0.1

// identificationSchema constrains the response to Identification via
// structured outputs. It is reflected from the struct so the two can't drift:
// every field is required and additional properties are rejected.
var identificationSchema = func() map[string]any {
	reflected := (&jsonschema.Reflector{DoNotReference: true}).Reflect(&Identification{})
	b, err := json.Marshal(reflected)
	if err != nil {
		panic(fmt.Sprintf("vision: marshal identification schema: %v", err))
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		panic(fmt.Sprintf("vision: unmarshal identification schema: %v", err))
	}
	delete(schema, "$schema")
	delete(schema, "$id")
	return schema
}()

// Client identifies items via the Anthropic Messages API.
type Client struct {
	api   anthropic.Client
	model string
}

// NewClient returns a Client for the given API key and model ID. The model
// ID always comes from config (ANTHROPIC_VISION_MODEL) — never hardcode one
// here. The SDK's environment autoload (ANTHROPIC_BASE_URL, auth tokens,
// profiles) is disabled so config.go is the only source of settings. Extra
// options (e.g. option.WithBaseURL) are intended for tests.
func NewClient(apiKey, model string, opts ...option.RequestOption) *Client {
	opts = append([]option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(requestTimeout),
		option.WithMaxRetries(maxRetries),
	}, opts...)
	return &Client{
		api:   anthropic.NewClient(opts...),
		model: model,
	}
}

// Identify implements Provider.
func (c *Client) Identify(ctx context.Context, img []byte, mime string) (Identification, string, error) {
	resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:       anthropic.Model(c.model),
		MaxTokens:   maxTokens,
		Temperature: anthropic.Float(temperature),
		System:      []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64(mime, base64.StdEncoding.EncodeToString(img)),
				anthropic.NewTextBlock(identifyPrompt),
			),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: identificationSchema},
		},
	})
	if err != nil {
		return Identification{}, c.model, fmt.Errorf("identify: %w", err)
	}

	if resp.StopReason == anthropic.StopReasonRefusal {
		return Identification{}, c.model, fmt.Errorf("identify: model refused (category %q): %s",
			resp.StopDetails.Category, resp.StopDetails.Explanation)
	}
	if resp.StopReason != anthropic.StopReasonEndTurn {
		return Identification{}, c.model, fmt.Errorf("identify: unexpected stop reason %q", resp.StopReason)
	}

	var raw string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			raw += tb.Text
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
