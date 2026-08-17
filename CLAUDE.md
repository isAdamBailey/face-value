# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Face Value is an image-in / value-out appraisal app. Upload a photo, a vision
model identifies the item, the eBay Browse API is queried for comparable
listings, and the app returns a price summary (average asking price across
current listings — never "sold value", Browse only returns active listings).

- **Backend**: Go, chi router, pgx v5/pgxpool, sqlc-generated queries,
  golang-migrate with embedded migrations
- **Frontend**: Nuxt 4 SPA (`ssr: false`), Tailwind v4, Pinia
- **Database**: PostgreSQL 16
- **Images**: Amazon S3 (private bucket, presigned GET URLs; MinIO locally)
- **Vision**: Hugging Face Inference Providers router (OpenAI-compatible)
- **Pricing**: eBay Browse API (active listings), behind a `pricing.Source`
  interface so a sold-comps source can drop in later
- **Local dev**: Docker Compose (Postgres + Mailpit + MinIO + backend + frontend)
- **Production**: Laravel Forge on a VPS — see `docs/DEPLOY.md`

Modeled on `isAdamBailey/massa` — same repo layout, same Go/Nuxt split, same
Forge deployment shape. Auth (magic link + email allowlist) is ported from
massa; don't rewrite it.

The build order and per-step checklists live as GitHub issues (#1 tracking,
sub-issues for each step).

## Commands

### Local stack

```sh
docker compose up --build
```

- Backend: http://localhost:8080 (`/healthz`)
- Frontend: http://localhost:3000
- Mailpit (catches magic-link emails in dev): http://localhost:8025
- MinIO console: http://localhost:9001
- Postgres: localhost:5432

### Backend (`backend/`)

Requires Go installed locally (`go 1.26`+).

```sh
go build ./...
go vet ./...
go test ./...
go run ./cmd/server        # requires DATABASE_URL and the rest of .env
go run ./cmd/migrate        # apply migrations standalone
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...
```

Regenerating sqlc code (run from `backend/`, where `sqlc.yaml` lives):

```sh
go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

Queries live in `queries/*.sql`; generated code lands in `internal/db/`.
After adding/changing a query, regenerate and check the generated
`*Params`/`Querier` signatures before wiring up callers.

### Frontend (`frontend/`)

```sh
npm run dev
npm run lint
npm run test          # vitest
npm run build
```

### Production deploy

Forge runs `scripts/forge-deploy.sh` on push. See `docs/DEPLOY.md`.

## Architecture

### Backend package structure (`backend/internal/`)

- `auth/` — magic link issue/verify, session cookies, allowlist (ported from massa)
- `config/` — env loading + fail-fast validation
- `db/` — sqlc-generated code + pgx pool + migration runner
- `email/` — smtp + ses senders (ported from massa)
- `ebay/` — OAuth application-token cache + Browse client
- `vision/` — `Provider` interface + Hugging Face implementation
- `pricing/` — `Source` interface + stats (mean/median/IQR-trimmed mean)
- `storage/` — `ImageStore` interface + S3 implementation
- `appraisal/` — orchestration: the pipeline tying vision → pricing → storage together
- `httpapi/` — chi router, handlers, middleware, DTOs

Key interfaces (`vision.Provider`, `pricing.Source`, `storage.ImageStore`) are
the seams for swapping implementations later — e.g. a sold-comps pricing
source. Define/extend interfaces before implementations.

### Data model

`searches` (one row per appraisal) and `comps` (eBay listings returned for a
search, with outliers flagged `excluded` rather than discarded). See
`backend/migrations/000003_appraisals.up.sql` for the full schema.

### Pipeline

Upload returns `202` immediately; vision → pricing → stats runs in a detached
goroutine (`internal/appraisal`), bounded by a semaphore
(`MAX_CONCURRENT_APPRAISALS`). The frontend polls `GET /api/searches/{id}`
for status. See `backend/internal/appraisal/appraisal.go` for timeouts and
panic-recovery.

### Money

Prices are `NUMERIC`/`decimal.Decimal`, never `float64` or `strconv.ParseFloat`.

## Conventions

The app is **Face Value**; the repo, Go module, DB role, S3 bucket, PM2
process, and domain all use specific forms that are not interchangeable —
see `docs/DEPLOY.md` for the concrete names in use.
