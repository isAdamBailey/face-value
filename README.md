# Face Value

[![Laravel Forge Site Deployment Status](https://img.shields.io/endpoint?url=https%3A%2F%2Fforge.laravel.com%2Fsite-badges%2Fe5ee7886-b263-42a9-bc97-3ee7717c2b01&style=plastic)](https://forge.laravel.com/adam-f6w/stunning-fern-s7f/3460341)

Photo in, price out. Upload a photo of something you own; a vision model
identifies it, and the app looks up current eBay listings to estimate an
average asking price. Not a sale, not an appraisal — a starting point. Access
is restricted to a small, pre-set allowlist of email addresses via
passwordless magic-link login.

## Stack

- **Backend**: Go, [chi](https://github.com/go-chi/chi), pgx, sqlc, golang-migrate
- **Vision**: Anthropic Messages API (Claude Haiku 4.5 by default)
- **Pricing**: eBay Browse API, behind a `pricing.Source` interface
- **Images**: Amazon S3 (private bucket, presigned URLs)
- **Frontend**: Nuxt 4, Tailwind CSS v4, Pinia
- **Database**: PostgreSQL 18
- **Local dev**: Docker Compose (Postgres, Mailpit, MinIO)

## Project layout

```
backend/    Go API server (cmd/server, cmd/migrate, internal/...)
frontend/   Nuxt 4 SPA
scripts/    Forge deploy script and API daemon wrapper
docs/       Deployment, eBay, and Anthropic setup
```

## Deployment

Production is [face-value.adambailey.io](https://face-value.adambailey.io) on the shared Forge server. A push to `main` deploys it. See [docs/DEPLOY.md](docs/DEPLOY.md). eBay and Anthropic credentials are in [docs/EBAY_SETUP.md](docs/EBAY_SETUP.md) and [docs/ANTHROPIC_SETUP.md](docs/ANTHROPIC_SETUP.md).

## Local development

1. Copy `.env.example` to `.env` and fill in the values (Anthropic API key,
   eBay sandbox keys, allowed emails, etc.).
2. Start everything with Docker Compose:

   ```sh
   docker compose up --build
   ```

   - Backend API: http://localhost:8080 (`/healthz` for a liveness check)
   - Frontend: http://localhost:3000
   - Mailpit (magic-link emails): http://localhost:8025
   - MinIO console: http://localhost:9001
   - Postgres: localhost:5432

### Backend only

```sh
cd backend
go run ./cmd/server
go test ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...
```

### Frontend only

```sh
cd frontend
npm install
npm run dev
npm run lint
npm run test
npm run build
```
