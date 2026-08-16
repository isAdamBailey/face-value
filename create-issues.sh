#!/usr/bin/env bash
# Creates the Face Value build-order issues.
#
# Usage:
#   1. gh issue create --title "Build plan: Face Value v1" --body-file PLAN.md
#   2. Note the issue number it prints, set TRACKING below.
#   3. Run this from inside the face-value repo:  bash create-issues.sh
#
# Idempotency: this script has none. Run it once. If it fails partway,
# delete the issues it made before re-running.

set -euo pipefail

TRACKING="${TRACKING:-1}"   # number of the tracking issue holding PLAN.md
REPO="isAdamBailey/face-value"

new() {
  local title="$1"; shift
  local body; body="$(cat)"
  gh issue create --repo "$REPO" --title "$title" \
    --body "${body}"$'\n\n---\nPart of #'"${TRACKING}"$'. Full spec lives there.'
}

# ---------------------------------------------------------------- 1
new "1. Scaffold monorepo" <<'EOF'
Stand up the skeleton so every later issue has somewhere to land.

- [ ] `backend/` with `cmd/server`, `cmd/migrate`, `internal/`, `migrations/`, `queries/`, `sqlc.yaml`
- [ ] `go.mod` as `github.com/isAdamBailey/face-value`, chi + pgx wired, `/healthz` responding
- [ ] `frontend/` Nuxt 4 + Tailwind v4 + Pinia, dev server up
- [ ] `docker-compose.yml`: postgres:16, mailpit, minio (+ bucket-init one-shot), backend, frontend
- [ ] `.env.example` per spec §6; `internal/config` loads it and **fails fast** on missing required vars
- [ ] `.gitignore`, `.github/workflows/` (go test, golangci-lint, npm lint/test/build), `CLAUDE.md`

Copy `.gitignore`, CI workflows, `scripts/run-api.sh`, and `frontend/ecosystem.config.cjs`
from `isAdamBailey/massa`, then rename per the conventions table in the spec:
repo `face-value`, Go module `face-value`, DB role/name `facevalue`, PM2 `facevalue-web`.

**Done when:** `docker compose up --build` gives a green `/healthz` and a Nuxt page at :3000.
EOF

# ---------------------------------------------------------------- 2
new "2. Port magic-link auth from massa" <<'EOF'
Auth is solved in massa. Copy it; do not rewrite it.

- [ ] Migrations for `allowed_users`, `magic_link_tokens`, `sessions` (needs `pgcrypto`)
- [ ] `internal/auth`: token issue/verify, signed session cookies, allowlist check, 5/hr rate limit
- [ ] `internal/email`: smtp + ses senders
- [ ] `POST /api/auth/magic-link`, `GET /api/auth/verify`, `POST /api/auth/logout`, `GET /api/me`
- [ ] `pages/login.vue`, `stores/auth.ts`, `middleware/auth.global.ts`

**Blocks everything else.** Do not start feature work until login round-trips end to end.

**Done when:** a link requested locally lands in Mailpit, consuming it sets a cookie,
and `/api/me` returns the user. A non-allowlisted address gets no email and no token row.

Depends on #1.
EOF

# ---------------------------------------------------------------- 3
new "3. Schema and sqlc for searches + comps" <<'EOF'
- [ ] `migrations/000002_appraisals.{up,down}.sql` — `searches` and `comps` exactly as spec §3
- [ ] Index `searches (user_email, created_at DESC)` and `comps (search_id)`
- [ ] `queries/*.sql` + generated sqlc code
- [ ] Down migration actually reverses cleanly

All money columns are `NUMERIC(12,2)`. Nothing in this project parses a price as a float —
use `pgtype.Numeric` or `shopspring/decimal` at every boundary.

`comps.excluded` is a flag, not a filter: outliers are stored, marked, and shown dimmed.

**Done when:** `go run ./cmd/migrate up` then `down` then `up` is clean, and generated
code compiles.

Depends on #1.
EOF

# ---------------------------------------------------------------- 4
new "4. Price statistics" <<'EOF'
Pure functions in `internal/pricing/stats.go`. No I/O, no DB — the highest-value tests
in the repo live here.

- [ ] Sorted input; Q1/Q3 via linear interpolation; IQR fences at `[Q1-1.5*IQR, Q3+1.5*IQR]`
- [ ] Headline number = **trimmed mean of non-excluded comps**
- [ ] Also return plain mean, median, min, max over the full set
- [ ] `n < 3`: trimmed mean falls back to median, set `low_sample`
- [ ] Table-driven tests: empty, n=1, n=2, tight cluster, one extreme high, bimodal (parts vs working)

Comment the rationale in code: eBay active listings have a long right tail (aspirational
pricing, mislabeled bundles) and a short left tail (parts/broken). Raw mean reads
consistently high; median throws away information; IQR-trimmed mean is the middle.

**Done when:** tests pass and the bimodal case produces a number you'd actually believe.

Depends on #1.
EOF

# ---------------------------------------------------------------- 5
new "5. eBay Browse API client" <<'EOF'
`internal/ebay` + the `pricing.Source` interface it satisfies.

- [ ] Define `pricing.Source` / `pricing.Comp` / `pricing.Query` first (spec §4.1)
- [ ] Application token via client credentials, cached in memory behind a mutex, refreshed at 90% of TTL — never one token per request
- [ ] `GET /buy/browse/v1/item_summary/search`, header `X-EBAY-C-MARKETPLACE-ID`
- [ ] Skip comps whose currency ≠ marketplace default (no FX in v1)
- [ ] Exclude live `AUCTION` items — a current bid is not a price signal
- [ ] Empty results → one retry with a shortened query (drop trailing keyword, or brand+model); log both attempts
- [ ] 429 → exponential backoff, max 3 attempts
- [ ] `httptest` fixtures: happy path, empty `itemSummaries`, 429-then-success, token refresh, mixed currency

`EBAY_API_BASE` is env-driven so this develops against sandbox. Sandbox has almost no
inventory — empty results there are expected, not a bug.

Depends on #1.
EOF

# ---------------------------------------------------------------- 6
new "6. Hugging Face vision client" <<'EOF'
`internal/vision`. Plain `net/http` against the OpenAI-compatible router — no vendor SDK.

- [ ] `vision.Provider` interface + `Identification` struct (spec §4.1)
- [ ] POST `$HF_API_BASE/chat/completions`, image as a base64 data URL in an `image_url` block
- [ ] Model from `HF_VISION_MODEL` — **never hardcoded**; confirm the chosen model is currently servable on the router before committing to it
- [ ] System prompt as a const in `prompt.go`; JSON-only, schema per spec §4.3
- [ ] Parse defensively: strip ```` ```json ```` fences, then one retry appending the raw output and "return only valid JSON"; second failure is a hard error
- [ ] Flag `low_confidence` when `confidence < 0.35` or `search_query` is empty
- [ ] Fixtures: clean JSON, fenced, prose-wrapped, malformed (exercises retry)

Then tune the prompt against 5–10 photos of things you actually own. Identification
quality dominates output quality — the eBay half is deterministic plumbing. Budget
real time here, not a token pass.

Depends on #1.
EOF

# ---------------------------------------------------------------- 7
new "7. S3 image storage and preprocessing" <<'EOF'
`internal/storage`, `aws-sdk-go-v2`, developed against MinIO.

- [ ] `ImageStore` interface; S3 impl with key layout `images/<yyyy>/<mm>/<uuid>.jpg`
- [ ] Store the **key** in `searches.image_key`, never a full URL
- [ ] Validate uploads by sniffing content (`http.DetectContentType`), not filename or client MIME; allow jpeg/png/webp; `http.MaxBytesReader` at `MAX_UPLOAD_BYTES`
- [ ] Resize longest edge to 1024px, JPEG q85, **strip EXIF** (removes GPS, normalizes orientation), store only the processed version
- [ ] `PutObject` with `ServerSideEncryption: AES256`
- [ ] Presigned GET URLs, TTL from `S3_PRESIGN_TTL`
- [ ] `DeleteObject` tolerates `NoSuchKey`

Bucket stays private — no public policy, no CORS needed (uploads go through the API,
reads are presigned GETs).

**Done when:** an upload round-trips through MinIO and the presigned URL renders in a browser.

Depends on #1.
EOF

# ---------------------------------------------------------------- 8
new "8. Appraisal pipeline and upload endpoint" <<'EOF'
`internal/appraisal` — the orchestration that ties vision + pricing + storage together.

- [ ] `POST /api/searches`: multipart, store image, insert row `status='pending'`, return **202** `{id, status}`
- [ ] Pipeline runs in a goroutine with a **detached context** (`context.Background()`, not `r.Context()`)
- [ ] Status transitions: `pending → identifying → pricing → complete`, or `failed` with a readable `error_message`
- [ ] `recover()` in the goroutine — a panic marks the row failed, never crashes the daemon
- [ ] Per-stage timeouts: 60s vision, 30s pricing, 2min overall cap
- [ ] Bounded concurrency via buffered-channel semaphore, `MAX_CONCURRENT_APPRAISALS` (start at 4)
- [ ] On boot, mark rows stuck in a non-terminal status >5min as `failed` (orphans from a restart)
- [ ] Never write partial rollups — stats and comps land in one transaction

The whole round trip is 5–20s. A synchronous POST works locally and dies behind nginx.

Depends on #3, #4, #5, #6, #7.
EOF

# ---------------------------------------------------------------- 9
new "9. Remaining API endpoints" <<'EOF'
- [ ] `GET /api/searches?limit=24&cursor=` — **cursor pagination on `(created_at, id)`**, not OFFSET; include presigned image URLs
- [ ] `GET /api/searches/{id}` — full detail with comps (excluded ones included, flagged) + presigned URL; this is the poll target
- [ ] `POST /api/searches/{id}/rerun` — re-price with an edited `search_query`, no re-upload
- [ ] `DELETE /api/searches/{id}` — row + S3 object
- [ ] All session-authenticated; scoped to `user_email`

OFFSET pagination breaks an infinite-scroll grid the moment a new search is inserted
above the cursor — rows duplicate and shift. Use the keyset.

The `rerun` endpoint is the difference between a toy and something usable: vision will
get model numbers wrong, and correcting the query without re-uploading is the fix.

Depends on #8.
EOF

# ---------------------------------------------------------------- 10
new "10. Home page: upload + grid" <<'EOF'
`pages/index.vue` — this is the product.

- [ ] Upload zone: drag-drop, file picker, `capture="environment"` so mobile opens the camera
- [ ] Client-side preview; client-side resize to max 2048px before upload
- [ ] On submit: POST → `{id}` → optimistically prepend a skeleton card → poll `GET /api/searches/{id}` every 2s, cap 60 attempts then show a "still working" state
- [ ] Grid `grid-cols-2 md:grid-cols-3 xl:grid-cols-4`: thumbnail, title, headline price, comp count, relative time
- [ ] Status treatments: `pending` shimmer, `failed` muted + retry
- [ ] Infinite scroll via `IntersectionObserver` on a sentinel, cursor from the API
- [ ] Empty state that explains what the app does
- [ ] `stores/searches.ts` owns polling — **not** the component, or navigating away orphans the poller
- [ ] Money via `Intl.NumberFormat` with the comp currency; never a hardcoded `$`
- [ ] Vitest: polling stops on `complete` and on `failed`; pagination doesn't duplicate rows

Copy says "average asking price" / "current listings" — never "sold" or "worth".

Depends on #9.
EOF

# ---------------------------------------------------------------- 11
new "11. Search detail page" <<'EOF'
`pages/search/[id].vue`.

- [ ] Large image + identification block (title, brand, model, category, condition notes)
- [ ] Headline trimmed mean, with mean / median / min / max as secondary stats
- [ ] Editable `search_query` + "Re-run pricing" → `POST .../rerun`, then resume polling
- [ ] Comps table: title, price, condition, buying option, link out to eBay
- [ ] Excluded outliers dimmed with an "outlier" tag + a toggle to hide them
- [ ] Low-confidence banner when the vision flag is set, prompting a query edit
- [ ] `low_sample` note when fewer than 3 comps backed the number

Showing the trimmed comps rather than hiding them is what makes the headline number
auditable — a user who disagrees can see exactly which listings were dropped.

Depends on #9.
EOF

# ---------------------------------------------------------------- 12
new "12. Forge deployment" <<'EOF'
- [ ] `scripts/forge-deploy.sh` and `scripts/run-api.sh` (port from massa)
- [ ] `docs/DEPLOY.md` mirroring massa's structure, per spec §7
- [ ] `docs/EBAY_SETUP.md`, `docs/HUGGINGFACE_SETUP.md`
- [ ] Private S3 bucket `face-value-images`, Block Public Access on, lifecycle → Standard-IA at 90 days
- [ ] Scoped IAM user: `s3:PutObject`/`GetObject`/`DeleteObject` on `arn:aws:s3:::face-value-images/images/*`, nothing else
- [ ] Forge site, project type Nuxt, web directory blank, `NUXT_PORT=3002` (massa likely holds 3001)
- [ ] Go daemon via Supervisor; copy the name into `FORGE_API_DAEMON`
- [ ] nginx: `/api/` and `/healthz` proxied to :8080, plus **`client_max_body_size 12M`** and **`proxy_read_timeout 120s`**

Those last two are absent from massa and both fail in ways that look like app bugs —
413s on upload and mystery 504s.

In Forge env: **omit `S3_ENDPOINT`** and set `S3_FORCE_PATH_STYLE=false`. Leaving the
MinIO endpoint set in production sends every S3 call to a host that doesn't exist.

**Done when:** push-to-deploy is green, `curl https://<domain>/healthz` returns OK,
and one photo goes end to end in production.

Depends on #10, #11.
EOF

# ---------------------------------------------------------------- 13
new "13. Production cutover and prompt tuning" <<'EOF'
- [ ] Swap `EBAY_API_BASE` to `https://api.ebay.com` with the production keyset
- [ ] Confirm real Browse results and sane comp counts against actual inventory
- [ ] Re-tune the vision prompt against real photos — sandbox tuning doesn't transfer
- [ ] Compare 2–3 VLMs on the same photo set before settling on `HF_VISION_MODEL`
- [ ] Audit UI copy: nothing claims "sold price", "market value", or "what it's worth"
- [ ] Sanity-check eBay daily call quota against expected usage

The honest framing isn't just legal caution — it's the thing that makes swapping in a
real sold-comps source later feel like an upgrade rather than a correction.

Depends on #12.
EOF

echo "Done. Review at: https://github.com/${REPO}/issues"
