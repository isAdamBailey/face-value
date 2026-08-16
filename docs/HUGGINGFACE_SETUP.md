# Hugging Face Vision Setup

Face Value identifies photographed items via a vision-language model served
through the [Hugging Face Inference Providers](https://huggingface.co/docs/inference-providers)
router — an OpenAI-compatible `chat/completions` endpoint that can route to
several underlying providers depending on the model. `internal/vision/huggingface.go`
talks to it with plain `net/http`, no vendor SDK.

## 1. Create an account and token

1. Sign up at [huggingface.co](https://huggingface.co/) if you don't have an
   account.
2. Go to **Settings → Access Tokens** and create a new token. A **Fine-grained**
   token scoped to "Make calls to Inference Providers" is enough — the app
   only calls `chat/completions`, nothing else.
3. Put the token in `HF_TOKEN`.

## 2. Confirm the model is servable

HF model availability on the router shifts over time — a model that's
servable today may be deprecated or moved to a different provider later.
`.env.example` defaults `HF_VISION_MODEL` to `Qwen/Qwen2.5-VL-72B-Instruct`,
confirmed present on the router's model list as of the initial build (see
issue #7 / commit history), but **reconfirm before relying on it**:

```sh
curl -s https://router.huggingface.co/v1/models \
  -H "Authorization: Bearer $HF_TOKEN" | grep -i "qwen2.5-vl"
```

If it's gone or you want a different vision-language model, list current
image-input chat models on the router and swap `HF_VISION_MODEL` — no code
change needed, the model ID is never hardcoded.

## 3. Billing

Inference Providers billing depends on the routed provider and model; check
the model's page on huggingface.co for its pricing before committing to it for
production use. A free tier covers light personal use for most models, but
confirm current terms rather than assuming.

## 4. Environment variables

| Variable | Value |
| --- | --- |
| `HF_TOKEN` | Access token from step 1 |
| `HF_VISION_MODEL` | A vision-language model ID currently servable on the router (step 2) |
| `HF_API_BASE` | `https://router.huggingface.co/v1` (shouldn't need to change) |

## 5. Prompt tuning

The system prompt (`internal/vision/prompt.go`) is a starting point, not a
tuned one. Before relying on this for real appraisals, run it against 5–10
photos of things you actually own and check:

- `title`/`brand`/`model` are specific enough to be useful, not generic
- `search_query` reads like something you'd actually type into eBay search —
  3–8 words, no adjectives or condition words
- `confidence` roughly tracks how right the identification actually was
- low-confidence photos correctly trigger the detail page's low-confidence
  banner (`confidence < 0.35` or empty `search_query`)

Identification quality dominates output quality here — the eBay half is
deterministic plumbing once the search query is good. This is worth real time,
not a token pass.

## 6. Verifying it works

Upload a real photo and confirm the search reaches at least `status: "pricing"`
with `title`/`brand`/`model`/`search_query` populated — that confirms the HF
call succeeded and the response parsed. If it fails at the identify stage,
`error_message` on the `searches` row will contain the HTTP status HF
returned (401 means a bad/missing token; check the daemon logs for the full
response body on other errors).
