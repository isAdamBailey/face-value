# Anthropic Vision Setup

Face Value identifies photographed items with Claude via the
[Anthropic Messages API](https://docs.claude.com/en/api/messages).
`internal/vision/anthropic.go` uses the official Go SDK
(`github.com/anthropics/anthropic-sdk-go`) and
[structured outputs](https://docs.claude.com/en/docs/build-with-claude/structured-outputs),
so the response always matches the `Identification` JSON schema.

## 1. API key

1. Sign in at [console.anthropic.com](https://console.anthropic.com/).
2. **Settings → API Keys → Create Key**. A dedicated key per environment
   (local vs production) makes rotation easier.
3. Put the key in `ANTHROPIC_API_KEY`.

## 2. Model

`ANTHROPIC_VISION_MODEL` defaults to `claude-haiku-4-5`: fast, cheap, and
good enough to read brands, model numbers and item types from a photo. If
identification quality isn't good enough on real photos, try
`claude-sonnet-5`. Whatever model you pick must support structured outputs.
The request sets a low `temperature` for repeatable search queries; newer
models such as Sonnet 5 and Opus 5 reject sampling parameters, so remove
`Temperature` in `internal/vision/anthropic.go` when switching to one.

Every search is one Messages API call with one JPEG, so cost per appraisal is
small; check [current pricing](https://www.anthropic.com/pricing#api) before
switching to a larger model.

## 3. Environment variables

| Variable | Value |
| --- | --- |
| `ANTHROPIC_API_KEY` | API key from step 1 |
| `ANTHROPIC_VISION_MODEL` | Claude model ID (default `claude-haiku-4-5`) |

## 4. Prompt tuning

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

## 5. Verifying it works

Upload a real photo and confirm the search reaches at least `status: "pricing"`
with `title`/`brand`/`model`/`search_query` populated — that confirms the
Anthropic call succeeded. If it fails at the identify stage, `error_message`
on the `searches` row will contain the API error (401 means a bad/missing key;
404 usually means `ANTHROPIC_VISION_MODEL` is not a valid model ID).
