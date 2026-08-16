# Face Value — Implementation Plan

An image-in / value-out appraisal app. Upload a photo, a vision model identifies the
item, the eBay Browse API is queried for comparable listings, and the app returns a
price summary. Home page is the search UI plus a grid of previous searches.

Modeled on [`isAdamBailey/massa`](https://github.com/isAdamBailey/massa) — same repo
layout, same Go/Nuxt split, same Forge deployment shape.

**Naming conventions** — the app is **Face Value**; the two written forms are not
interchangeable, so use exactly these:

| Context | Form |
|---|---|
| Repo / GitHub | `face-value` (matches `black-circles`, `show-tracker`) |
| Go module | `github.com/isAdamBailey/face-value` |
| Postgres role + database | `facevalue` (no hyphen — avoids quoting every identifier) |
| S3 bucket | `face-value-images` |
| PM2 process | `facevalue-web` |
| Domain | `facevalue.example.com` |
| Display name in UI, README, `<title>` | Face Value |

---

## 1. Decisions already made

These are settled. Do not re-litigate them mid-build.

| Decision | Choice | Why |
|---|---|---|
| Price data | eBay **Browse API** (active listings) first, behind a `PriceSource` interface | Marketplace Insights (sold comps) is Limited Release and closed to new developers. Browse works today with standard production keys. |
| Vision | **Hugging Face Inference Providers** router, OpenAI-compatible endpoint | Account already exists. Provider-agnostic via `VisionProvider` interface. |
| Auth | **Magic-link + email allowlist**, same as massa | Personal tool, no signup flow, no password storage. |
| DB | PostgreSQL 16 | Matches massa. |
| Images | **Amazon S3** via `aws-sdk-go-v2` | Survives deploys and server rebuilds with no persistent-disk juggling; same pattern as shudderfly. |
| Deploy | Single VPS via Laravel Forge, nginx proxying `/api` to a Go daemon | Matches massa exactly. |

**Critical constraint to carry into the UI copy:** Browse returns *asking* prices for
*active* listings, not realized sale prices. The app must never label its output
"sold value" or "what it's worth." Use "average asking price" / "current listings."
This is both honest and the thing that makes the sold-comps swap meaningful later.

---

## 2. Repo layout

Mirror massa:

```
backend/                 Go API server
  cmd/
    server/main.go       HTTP server entrypoint
    migrate/main.go      golang-migrate runner
  internal/
    auth/                magic link issue/verify, session cookies, allowlist
    config/              env loading + validation
    db/                  sqlc-generated code + pgx pool
    email/               smtp + ses senders (port from massa)
    ebay/                OAuth token cache + Browse client
    vision/              VisionProvider interface + huggingface impl
    pricing/             PriceSource interface + stats (mean/median/outliers)
    storage/             ImageStore interface + local disk impl
    appraisal/           orchestration: the pipeline that ties it together
    httpapi/             chi router, handlers, middleware, DTOs
  migrations/            golang-migrate .up.sql / .down.sql pairs
  queries/               .sql files for sqlc
  sqlc.yaml
  go.mod
frontend/                Nuxt 4 SPA
  app/
    pages/index.vue      search UI + previous-search grid
    pages/search/[id].vue  single result detail
    pages/login.vue
    components/
    stores/              Pinia
    composables/
  ecosystem.config.cjs   PM2 (copy from massa)
  nuxt.config.ts
scripts/
  forge-deploy.sh
  run-api.sh
docs/
  DEPLOY.md
  EBAY_SETUP.md
  HUGGINGFACE_SETUP.md
.env.example
docker-compose.yml
CLAUDE.md
PRODUCT.md
DESIGN.md
README.md
```

Copy `backend/internal/auth`, `backend/internal/email`, `scripts/run-api.sh`,
`frontend/ecosystem.config.cjs`, and the magic-link migrations from massa verbatim
where possible. Auth is solved there; don't rewrite it.

---

## 3. Data model

Three tables beyond the auth tables inherited from massa
(`allowed_users`, `magic_link_tokens`, `sessions`).

```sql
-- migrations/000002_appraisals.up.sql

CREATE TABLE searches (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_email      TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
                    -- pending | identifying | pricing | complete | failed
    error_message   TEXT,

    image_key       TEXT        NOT NULL,   -- opaque key for ImageStore
    image_width     INT,
    image_height    INT,

    -- vision output
    title           TEXT,                   -- "Sony TC-377 reel-to-reel tape deck"
    brand           TEXT,
    model           TEXT,
    category        TEXT,
    condition_notes TEXT,
    search_query    TEXT,                   -- the string actually sent to eBay
    vision_model    TEXT,                   -- e.g. "Qwen/Qwen2.5-VL-72B-Instruct"
    vision_raw      JSONB,                  -- full parsed model response
    confidence      NUMERIC(3,2),           -- 0.00-1.00, model self-reported

    -- pricing rollup
    price_source    TEXT,                   -- 'ebay_browse' | 'ebay_sold' | ...
    currency        TEXT,
    comp_count      INT,
    price_mean      NUMERIC(12,2),
    price_median    NUMERIC(12,2),
    price_min       NUMERIC(12,2),
    price_max       NUMERIC(12,2),
    price_trimmed_mean NUMERIC(12,2),       -- the headline number

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX searches_user_created_idx ON searches (user_email, created_at DESC);

CREATE TABLE comps (
    id            BIGSERIAL PRIMARY KEY,
    search_id     UUID NOT NULL REFERENCES searches(id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,            -- eBay itemId
    title         TEXT NOT NULL,
    price         NUMERIC(12,2) NOT NULL,
    currency      TEXT NOT NULL,
    condition     TEXT,
    buying_option TEXT,                     -- FIXED_PRICE | AUCTION | BEST_OFFER
    item_url      TEXT,
    thumbnail_url TEXT,
    seller_country TEXT,
    excluded      BOOLEAN NOT NULL DEFAULT false,  -- outlier-trimmed
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX comps_search_idx ON comps (search_id);
```

Notes for the agent:
- `pgcrypto` must be enabled for `gen_random_uuid()` (massa's deploy doc already does this).
- Store **all** comps returned, marking outliers `excluded = true` rather than
  discarding. The detail page shows both, and it makes the stats auditable.
- Prices are `NUMERIC`, never float. Parse eBay's string prices with
  `shopspring/decimal` or `pgtype.Numeric` — never `strconv.ParseFloat`.

---

## 4. Backend

### 4.1 Interfaces (define these first, before any implementation)

```go
// internal/vision/vision.go
type Identification struct {
    Title          string   `json:"title"`
    Brand          string   `json:"brand"`
    Model          string   `json:"model"`
    Category       string   `json:"category"`
    ConditionNotes string   `json:"condition_notes"`
    SearchQuery    string   `json:"search_query"`
    Keywords       []string `json:"keywords"`
    Confidence     float64  `json:"confidence"`
}

type Provider interface {
    Identify(ctx context.Context, img []byte, mime string) (Identification, string, error)
    // returns identification, model identifier used, error
}
```

```go
// internal/pricing/pricing.go
type Comp struct {
    ExternalID    string
    Title         string
    Price         decimal.Decimal
    Currency      string
    Condition     string
    BuyingOption  string
    ItemURL       string
    ThumbnailURL  string
    SellerCountry string
}

type Query struct {
    Text        string
    CategoryID  string   // optional
    Marketplace string   // "EBAY_US"
    Limit       int
}

type Source interface {
    Name() string                                        // "ebay_browse"
    Find(ctx context.Context, q Query) ([]Comp, error)
}
```

```go
// internal/storage/storage.go
type ImageStore interface {
    Put(ctx context.Context, r io.Reader, mime string) (key string, err error)
    Get(ctx context.Context, key string) (io.ReadCloser, string, error)
    URL(key string) string
}
```

The whole point of the sold-comps escape hatch is `pricing.Source`. Implement
`ebayBrowseSource` now; a future `ebaySoldSource` or third-party source drops in
with a config switch and zero handler changes.

### 4.2 eBay Browse client (`internal/ebay`)

**Auth — application token (client credentials), not user token.**

```
POST https://api.ebay.com/identity/v1/oauth2/token
Authorization: Basic base64(EBAY_CLIENT_ID:EBAY_CLIENT_SECRET)
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials&scope=https%3A%2F%2Fapi.ebay.com%2Foauth%2Fapi_scope
```

Response includes `access_token` and `expires_in` (~7200s). Cache it in memory
behind a mutex; refresh at 90% of TTL. Never fetch a token per request.

Sandbox host is `api.sandbox.ebay.com` with separate credentials — make the host
an env var (`EBAY_API_BASE`) so the agent can develop against sandbox.

**Search:**

```
GET https://api.ebay.com/buy/browse/v1/item_summary/search
      ?q={query}
      &limit=50
      &filter=buyingOptions:{FIXED_PRICE},conditionIds:{...}
      &sort=price
Authorization: Bearer {token}
X-EBAY-C-MARKETPLACE-ID: EBAY_US
```

Handling rules:
- Read `itemSummaries[].price.value` + `.currency`. **Skip any comp whose currency
  differs from the marketplace default** rather than converting — no FX in v1.
- Prefer `FIXED_PRICE` and `BEST_OFFER`; exclude live `AUCTION` items, whose current
  bid is not a price signal.
- If `itemSummaries` is empty, retry once with a shortened query (drop the last
  keyword or fall back to `brand + model`). Record both attempts in `vision_raw`
  or a log — the agent should not silently return "no results" on first miss.
- Handle 429 with exponential backoff, max 3 attempts. Default production rate
  limit is on the order of 5,000 calls/day; log remaining quota headers if present.
- Any non-2xx after retries → set search `status = 'failed'` with a readable
  `error_message`. Never write partial rollups.

### 4.3 Vision client (`internal/vision/huggingface.go`)

```
POST https://router.huggingface.co/v1/chat/completions
Authorization: Bearer $HF_TOKEN
Content-Type: application/json
```

Body: OpenAI-shaped, with the image as a base64 data URL:

```json
{
  "model": "Qwen/Qwen2.5-VL-72B-Instruct",
  "max_tokens": 500,
  "temperature": 0.1,
  "messages": [
    { "role": "system", "content": "<see prompt below>" },
    { "role": "user", "content": [
      { "type": "text", "text": "Identify this item." },
      { "type": "image_url", "image_url": { "url": "data:image/jpeg;base64,..." } }
    ]}
  ]
}
```

Model ID goes in `HF_VISION_MODEL` env — do **not** hardcode. HF model availability
shifts; the agent should confirm the chosen model is currently servable on the
router before committing.

**System prompt** (store as a Go const, `internal/vision/prompt.go`):

> You identify physical objects in photographs for the purpose of looking up
> comparable listings on eBay. Respond with JSON only — no prose, no markdown
> fences. Schema:
> `{"title","brand","model","category","condition_notes","search_query","keywords":[],"confidence"}`
> `search_query` must be 3–8 words optimized as an eBay search: brand, model
> number, and item type, no adjectives, no condition words. If you cannot identify
> a brand or model, leave those empty and build `search_query` from the generic
> item type plus distinguishing visible features. `confidence` is 0.0–1.0
> reflecting how certain you are of the specific model identification.

Parsing: strip ```` ```json ```` fences defensively, `json.Unmarshal`, and if it
fails, retry **once** with an appended user message containing the raw output and
"Return only valid JSON matching the schema." Second failure → `status = 'failed'`.

Guardrail: if `confidence < 0.35` or `search_query` is empty, mark the search
complete but flag `low_confidence` in the response so the UI can prompt the user
to edit the query and re-run rather than presenting a bogus number confidently.

### 4.4 Image handling (`internal/storage`)

- Accept `multipart/form-data`, field `image`. Max 10 MB (`http.MaxBytesReader`).
- Validate by **sniffing content** (`http.DetectContentType`), not the filename or
  client-supplied MIME. Allow `image/jpeg`, `image/png`, `image/webp` only.
- Re-encode and downscale before sending to the model: longest edge 1024px, JPEG
  quality 85, strip EXIF. Use `golang.org/x/image/draw`. This cuts token cost,
  removes GPS metadata, and normalizes orientation.
- Store the downscaled version, not the original — this is an appraisal tool, not
  a photo archive.

**S3 implementation** (`internal/storage/s3.go`), using `aws-sdk-go-v2`:

- Key layout: `images/<yyyy>/<mm>/<uuid>.jpg`. Store only this key in
  `searches.image_key` — never a full URL, so the bucket or region can change
  without a data migration.
- Bucket stays **private**. Block Public Access on, no bucket policy granting reads.
- Serve reads as **presigned GET URLs**, generated by
  `GET /api/searches/{id}` (and the list endpoint) alongside each row, with a
  15-minute expiry. The Nuxt grid uses those URLs directly, so image bytes never
  transit the Go daemon and the VPS does no image bandwidth.
  - This replaces the `GET /api/images/{key}` proxy endpoint in §4.7. Drop it.
  - Practical consequence: a page left open longer than the expiry shows broken
    thumbnails. Have the searches store refetch the list on `visibilitychange`
    after 10+ minutes, or set expiry to 1 hour and accept it.
- Uploads go **server-side** (`PutObject` from the Go handler), not via presigned
  PUT from the browser. The server must resize and strip EXIF before storage, so
  the bytes have to pass through it anyway.
- `DELETE /api/searches/{id}` issues a `DeleteObject`. Tolerate `NoSuchKey` —
  a missing object should not block deleting the row.
- Set `ServerSideEncryption: AES256` on put. Free, one line, no key management.
- Credentials from env in dev; on the VPS, also env (Forge has no instance roles).
  The IAM user gets a scoped policy — `s3:PutObject`, `s3:GetObject`,
  `s3:DeleteObject` on `arn:aws:s3:::<bucket>/images/*` and nothing else.
- Add a lifecycle rule transitioning objects to Infrequent Access after 90 days.
  Appraisal images are written once and viewed rarely.

Keep the `ImageStore` interface: a local-disk impl is still worth writing for tests
and offline dev (or point the S3 impl at MinIO in docker-compose via a custom
endpoint — that's the better option since it exercises the real code path).

### 4.5 Price statistics (`internal/pricing/stats.go`)

Pure functions, no I/O — this is the most testable code in the project, so it gets
real table-driven tests.

1. Sort prices ascending.
2. If `n < 3`: report mean, median, min, max; trimmed mean = median; set a
   `low_sample` flag.
3. If `n >= 3`: compute IQR (Q1, Q3 via linear interpolation). Mark any comp
   outside `[Q1 - 1.5*IQR, Q3 + 1.5*IQR]` as `excluded = true`.
4. Headline number = **trimmed mean of non-excluded comps**. Also persist plain
   mean, median, min, max over the full set.

Rationale worth putting in a code comment: eBay active listings have a long right
tail (aspirational pricing, mislabeled bundles) and a short left tail (parts/broken
units). A raw mean is consistently wrong high. Median is robust but discards
information. IQR-trimmed mean is the reasonable middle.

### 4.6 Pipeline & concurrency (`internal/appraisal`)

The full round trip is 5–20 seconds. Do **not** block the HTTP request.

```
POST /api/searches
  → validate + store image
  → INSERT searches (status='pending')
  → go runPipeline(context.Background(), searchID)   // detached ctx, not r.Context()
  → 202 { id, status: "pending" }
```

`runPipeline`:
1. `status = 'identifying'` → `vision.Identify`
2. `status = 'pricing'` → `pricing.Source.Find`
3. compute stats, insert comps, update rollup, `status = 'complete'`, `completed_at = now()`
4. any error → `status = 'failed'`, `error_message` set

Requirements:
- Wrap the goroutine in a `recover()` — a panic must mark the row failed, not crash
  the daemon.
- Per-stage `context.WithTimeout`: 60s vision, 30s pricing. Overall cap 2 minutes.
- Bound concurrency with a buffered channel semaphore (start at 4). A single VPS
  should not fan out unbounded goroutines against two rate-limited APIs.
- On boot, mark any row still `pending`/`identifying`/`pricing` older than 5 minutes
  as `failed` — cleans up rows orphaned by a restart mid-pipeline.

### 4.7 HTTP API

All under `/api`, all session-cookie authenticated except the auth endpoints.

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/auth/magic-link` | request link (rate-limited 5/hr, from massa) |
| `GET` | `/api/auth/verify` | consume token, set cookie, redirect |
| `POST` | `/api/auth/logout` | clear session |
| `GET` | `/api/me` | current user, for Nuxt middleware |
| `POST` | `/api/searches` | multipart upload → 202 `{id, status}` |
| `GET` | `/api/searches` | paginated list for the grid: `?limit=24&cursor=` |
| `GET` | `/api/searches/{id}` | full detail incl. comps; poll target |
| `POST` | `/api/searches/{id}/rerun` | re-price with an edited `search_query` |
| `DELETE` | `/api/searches/{id}` | delete row + image |
| — | — | *(no image endpoint — list and detail responses include presigned S3 URLs)* |
| `GET` | `/healthz` | liveness, unauthenticated |

Cursor pagination on `(created_at, id)`, not `OFFSET` — the grid is
infinite-scroll and offset pagination breaks when new searches are inserted.

The `rerun` endpoint matters: vision will sometimes get the model number wrong, and
letting the user correct the query and re-price without re-uploading is the
difference between a toy and something usable.

---

## 5. Frontend (Nuxt 4)

Match massa: Nuxt 4, Tailwind v4, Pinia. SPA mode, `ssr: false` unless massa does
otherwise — cookie auth against a same-origin `/api` is simplest without SSR.

### Pages

**`pages/index.vue`** — the whole product.

- **Upload zone** (top): drag-drop + file picker + `capture="environment"` on
  mobile so it opens the camera. Client-side preview before submit. Client-side
  resize to max 2048px before upload to save bandwidth (the server resizes again
  to 1024 for the model — belt and braces).
- On submit: POST, receive `{id}`, optimistically prepend a skeleton card to the
  grid, begin polling `GET /api/searches/{id}` every 2s (cap 60 attempts, then
  show a "still working / refresh" state).
- **Grid** (below): responsive card grid, `grid-cols-2 md:grid-cols-3 xl:grid-cols-4`.
  Each card: thumbnail, title, headline price, comp count, relative timestamp,
  and a status treatment for `pending` (shimmer) / `failed` (muted + retry).
- Infinite scroll via `IntersectionObserver` on a sentinel div, cursor from the API.
- Empty state that explains what the app does rather than showing a bare grid.

**`pages/search/[id].vue`** — detail.
- Large image, identification block (title/brand/model/category/condition notes).
- Headline trimmed mean, with mean/median/min/max as secondary stats.
- Editable `search_query` input + "Re-run pricing" button → `POST .../rerun`.
- Comps table: title, price, condition, buying option, link out to eBay. Excluded
  outliers rendered dimmed with an "outlier" tag, plus a toggle to hide them.
- Low-confidence banner when the vision flag is set.

**`pages/login.vue`** — port from massa unchanged.

### Stores

- `stores/auth.ts` — user, `fetchMe`, `logout`.
- `stores/searches.ts` — list, cursor, `loadMore`, `create`, `poll(id)`, `remove`.
  Keep polling logic in the store, not the component, so navigating to the detail
  page doesn't orphan a poller.

### Middleware
- `middleware/auth.global.ts` — redirect to `/login` on 401 from `/api/me`.

### Conventions
- `$fetch` with `credentials: 'include'`, base URL from `useRuntimeConfig().public.apiBase`,
  which is empty in production (same-origin `/api`) and `http://localhost:8080` in dev.
- Format money with `Intl.NumberFormat` using the comp currency, never a hardcoded `$`.

---

## 6. Environment variables

`.env.example`:

```bash
# Server
PORT=8080
APP_BASE_URL=http://localhost:3000
DATABASE_URL=postgres://facevalue:facevalue@postgres:5432/facevalue?sslmode=disable

# Auth (from massa)
COOKIE_SIGNING_SECRET=dev-placeholder-change-me
COOKIE_SECURE=false
ALLOWED_EMAILS=you@example.com
MAGIC_LINK_FROM_EMAIL=login@facevalue.local
EMAIL_PROVIDER=smtp
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_USERNAME=
SMTP_PASSWORD=
SES_REGION=

# Vision
HF_TOKEN=
HF_VISION_MODEL=Qwen/Qwen2.5-VL-72B-Instruct
HF_API_BASE=https://router.huggingface.co/v1

# eBay
EBAY_CLIENT_ID=
EBAY_CLIENT_SECRET=
EBAY_API_BASE=https://api.sandbox.ebay.com
EBAY_MARKETPLACE_ID=EBAY_US
EBAY_COMP_LIMIT=50
PRICE_SOURCE=ebay_browse

# Images (S3)
S3_BUCKET=face-value-images-dev
S3_REGION=us-west-2
S3_ACCESS_KEY_ID=
S3_SECRET_ACCESS_KEY=
S3_ENDPOINT=http://minio:9000   # empty in production; set only for MinIO
S3_FORCE_PATH_STYLE=true        # false in production
S3_PRESIGN_TTL=15m
MAX_UPLOAD_BYTES=10485760

# Pipeline
MAX_CONCURRENT_APPRAISALS=4

# Frontend
NUXT_PUBLIC_API_BASE=http://localhost:8080
```

`internal/config` must **fail fast on boot** if `HF_TOKEN`, `EBAY_CLIENT_ID`,
`EBAY_CLIENT_SECRET`, `DATABASE_URL`, `COOKIE_SIGNING_SECRET`, `S3_BUCKET`, or the
S3 credentials are missing.
A daemon that starts and then fails every request is worse than one that won't start.

---

## 7. Forge deployment (`docs/DEPLOY.md`)

Identical to massa's setup, with three additions. Write the doc to mirror massa's
structure so the two are maintainable side by side.

**Architecture**

```
https://facevalue.example.com
         │
      Nginx (Forge-managed)
         ├─ /api/*, /healthz  → 127.0.0.1:8080  (Go daemon)
         └─ /*                → 127.0.0.1:3002  (Nuxt via PM2)

Postgres → 127.0.0.1 on the VPS
Images   → Amazon S3 (private bucket, presigned GETs straight to the browser)
```

Because images live in S3, the VPS is fully stateless apart from Postgres — nothing
in the release directory needs to survive a deploy.

**Steps**

1. **Server prereqs** — Postgres + `pgcrypto`; Go installed to `/usr/local/go`
   (massa's DEPLOY.md step 1 verbatim). If deploying alongside massa on the same
   server, Go and Postgres are already there — just create the new role/database.
2. **Forge site** — new site, project type **Nuxt**, web directory blank (repo root),
   server port `3002` if massa holds `3001`. Connect GitHub, enable push-to-deploy.
   Deploy script: `bash $FORGE_SITE_PATH/scripts/forge-deploy.sh`. Issue Let's Encrypt cert.
3. **S3 bucket** — create a private bucket (e.g. `face-value-images`) in your region
   with Block Public Access fully on. Create a dedicated IAM user, no console
   access, with an inline policy limited to `s3:PutObject`, `s3:GetObject`, and
   `s3:DeleteObject` on `arn:aws:s3:::face-value-images/images/*`. Put the access key
   pair in Forge env. Add a lifecycle rule → Standard-IA after 90 days.
   No CORS config is needed: the browser only issues GETs against presigned URLs,
   and uploads go through the API.
4. **Environment** — Forge → site → Environment. Production overrides:
   `DATABASE_URL` → `postgres://facevalue:PASSWORD@127.0.0.1:5432/facevalue?sslmode=disable`;
   `APP_BASE_URL` → `https://facevalue.example.com`; `COOKIE_SECURE=true`;
   `COOKIE_SIGNING_SECRET` → `openssl rand -base64 32`; `EMAIL_PROVIDER=ses`;
   `EBAY_API_BASE` → `https://api.ebay.com` (production keyset, not sandbox);
   `S3_BUCKET`/`S3_REGION`/`S3_ACCESS_KEY_ID`/`S3_SECRET_ACCESS_KEY` → real values;
   **omit `S3_ENDPOINT`** and set `S3_FORCE_PATH_STYLE=false` (those two exist only
   for MinIO — leaving `S3_ENDPOINT` set in production sends every request to a
   host that doesn't exist);
   omit `NUXT_PUBLIC_API_BASE`; add `NUXT_PORT=3002` and `FORGE_API_DAEMON=daemon-XXXXXXX`.
5. **Go API daemon** — Server → Daemons → New Daemon.
   Command `/home/forge/facevalue.example.com/current/scripts/run-api.sh`,
   directory `/home/forge/facevalue.example.com/current`, user `forge`.
   Copy the supervisor name into `FORGE_API_DAEMON`.
6. **Nginx** — add before `location /`, and note the larger body size, which massa
   does not need:
   ```nginx
   client_max_body_size 12M;

   location /api/ {
       proxy_pass http://127.0.0.1:8080;
       proxy_http_version 1.1;
       proxy_set_header Host $host;
       proxy_set_header X-Real-IP $remote_addr;
       proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
       proxy_set_header X-Forwarded-Proto $scheme;
       proxy_read_timeout 120s;
   }

   location = /healthz {
       proxy_pass http://127.0.0.1:8080;
       proxy_http_version 1.1;
       proxy_set_header Host $host;
   }
   ```
   `client_max_body_size` and `proxy_read_timeout` are the two settings that will
   silently break uploads and long appraisals if omitted.
7. **Deploy script** (`scripts/forge-deploy.sh`) — port massa's, adding the image dir:
   `git pull` → `export PATH=$PATH:/usr/local/go/bin` →
   `go build -o bin/server ./cmd/server` → `go run ./cmd/migrate up` →
   `cd frontend && npm ci && npm run build` → `pm2 reload ecosystem.config.cjs --update-env`
   → `sudo supervisorctl restart $FORGE_API_DAEMON`.
8. **Verify** — `curl -s https://facevalue.example.com/healthz`, `pm2 list`,
   request a magic link, upload one photo end to end.

**Troubleshooting table** — carry over massa's rows and add:

| Symptom | Fix |
|---|---|
| 413 on upload | `client_max_body_size` missing from nginx |
| Upload 504s | `proxy_read_timeout` too low, or pipeline blocking the request (it shouldn't) |
| Thumbnails break after a while | Presigned URLs expired on a long-open tab — refetch the list, or raise `S3_PRESIGN_TTL` |
| All images 403 | IAM policy missing `s3:GetObject`, or the key prefix in the policy doesn't match `images/*` |
| Every upload fails, works locally | `S3_ENDPOINT` still set to the MinIO host in Forge env |
| Every search fails at identify | `HF_TOKEN` unset, or `HF_VISION_MODEL` no longer servable on the router |
| Every search returns 0 comps | Still pointing at `api.sandbox.ebay.com`, which has almost no inventory |

---

## 8. Local development

`docker-compose.yml`: `postgres:16`, `mailpit`, `minio`, `backend` (air or plain
`go run`), `frontend` (`npm run dev`). MinIO stands in for S3 so dev exercises the
real `PutObject`/presign code path rather than a divergent local-disk branch. Add a
one-shot `mc` init container (or a `scripts/minio-init.sh`) that creates the bucket
on first boot — otherwise the first upload fails with `NoSuchBucket` and looks like
a code bug.

```bash
docker compose up --build
# Backend  http://localhost:8080  (/healthz)
# Frontend http://localhost:3000
# Mailpit  http://localhost:8025
# MinIO    http://localhost:9001  (console)
```

Note: presigned URLs from MinIO reference `S3_ENDPOINT` as the host. Set it to
`http://localhost:9000` (not `http://minio:9000`) if the browser needs to load
them directly, or run the API with both an internal and a public endpoint value.

---

## 9. Testing

- **`internal/pricing/stats_test.go`** — table-driven: empty, n=1, n=2, tight
  cluster, one extreme high outlier, bimodal (parts vs working units). This is
  where correctness lives.
- **`internal/ebay`** — `httptest.Server` returning canned Browse JSON fixtures.
  Cover: happy path, empty `itemSummaries`, 429-then-success, expired token refresh,
  mixed-currency filtering.
- **`internal/vision`** — canned HF responses: clean JSON, fenced JSON, prose-wrapped
  JSON, malformed JSON forcing the retry path.
- **`internal/appraisal`** — pipeline with fake `Provider` and `Source`; assert
  status transitions and that a panicking provider yields `status='failed'`.
- **Frontend** — Vitest on the searches store: polling stops on `complete`, stops
  on `failed`, cursor pagination doesn't duplicate rows.
- **CI** — copy massa's `.github/workflows`: `go test ./...`, golangci-lint,
  `npm run lint`, `npm run test`, `npm run build`.

---

## 10. Build order

Ship in this sequence; each step is independently verifiable.

0. **Create the repo.** `face-value`, private or public to taste, no README/gitignore
   from GitHub's templates (you're writing your own):

   ```bash
   gh repo create isAdamBailey/face-value --private --description \
     "Photo in, price out. Identifies items with a vision model and prices them against live eBay listings. Go, Nuxt 4, PostgreSQL."

   git clone git@github.com:isAdamBailey/face-value.git
   cd face-value
   mkdir -p backend/{cmd,internal,migrations,queries} frontend scripts docs .github/workflows
   printf 'module github.com/isAdamBailey/face-value\n\ngo 1.26\n' > backend/go.mod
   ```

   Seed from massa rather than from scratch: copy `.gitignore`,
   `.github/workflows/`, `scripts/run-api.sh`, `docker-compose.yml`, and
   `frontend/ecosystem.config.cjs` across, then find-and-replace `massa` →
   `facevalue`/`face-value` per the conventions table above. Copy `CLAUDE.md` too
   and rewrite it for this project — an agent working the repo will read it before
   this plan.

   Commit this plan as `PLAN.md` at the root in the first commit so the build
   history starts from the spec.

1. Scaffold repo layout, `go.mod`, Nuxt 4 app, docker-compose, `.env.example`.
2. Port auth wholesale from massa (magic link, allowlist, sessions, `/api/me`) +
   `pages/login.vue`. **Verify login works before writing any feature code.**
3. Migrations + sqlc for `searches` and `comps`.
4. `pricing/stats.go` + tests. Pure, no dependencies, fastest confidence win.
5. `internal/ebay` Browse client + token cache, against sandbox, with fixtures.
6. `internal/vision` HF client + prompt, tested against 5–10 real photos of things
   you actually own — this is where prompt tuning happens, and it needs real input.
7. `internal/storage` S3 impl + resize/EXIF-strip pipeline, against MinIO.
8. `internal/appraisal` orchestration + `POST /api/searches` returning 202.
9. Remaining endpoints: list (cursor), detail, rerun, delete, image serve.
10. `pages/index.vue`: upload + polling + grid + infinite scroll.
11. `pages/search/[id].vue`: detail, comps table, rerun.
12. `docs/DEPLOY.md`, `scripts/forge-deploy.sh`, `scripts/run-api.sh`; first deploy.
13. Swap `EBAY_API_BASE` to production, tune the prompt against real inventory.

---

## 11. Open items

Flagged rather than guessed. None block step 1.

- **Marketplace scope.** Plan assumes `EBAY_US` and single-currency. Multi-marketplace
  means an FX layer; deliberately deferred.
- **Sold comps.** If you later get Marketplace Insights access or subscribe to a
  third-party sold-data service, it lands as a second `pricing.Source`. Budget a
  day, not a week — that's what the interface buys.
- **HF model choice.** `Qwen2.5-VL-72B-Instruct` is a reasonable starting point,
  but confirm current router availability and compare 2–3 VLMs on your own photos
  before settling. Identification quality dominates output quality; the eBay half
  is deterministic plumbing.
- **Cost.** HF free tier will cover personal use; if it doesn't, the per-call cost
  is the only real running expense beyond the VPS.
- **Retention.** No cleanup job planned. If the grid grows past a few thousand rows,
  add a scheduled command that prunes old `failed` searches and their S3 objects;
  storage cost at this scale is under a dollar a month including requests. The
  lifecycle rule is future-proofing, not a real saving today.
