# Face Value

Photo in, price out. Upload a photo of something you own; a vision model
identifies it, and the app looks up current eBay listings to estimate an
average asking price. Not a sale, not an appraisal — a starting point. Access
is restricted to a small, pre-set allowlist of email addresses via
passwordless magic-link login.

## Stack

- **Backend**: Go, [chi](https://github.com/go-chi/chi), pgx, sqlc, golang-migrate
- **Vision**: Hugging Face Inference Providers (OpenAI-compatible router)
- **Pricing**: eBay Browse API, behind a `pricing.Source` interface
- **Images**: Amazon S3 (private bucket, presigned URLs)
- **Frontend**: Nuxt 4, Tailwind CSS v4, Pinia
- **Database**: PostgreSQL 16
- **Local dev**: Docker Compose (Postgres, Mailpit, MinIO)

## Project layout

```
backend/    Go API server (cmd/server, cmd/migrate, internal/...)
frontend/   Nuxt 4 SPA
scripts/    Forge deploy script and API daemon wrapper
docs/       Deployment, eBay, and Hugging Face setup
```

## Deployment

See [docs/DEPLOY.md](docs/DEPLOY.md) for production on a VPS with
[Laravel Forge](https://forge.laravel.com) (push-to-deploy from GitHub), and
[docs/EBAY_SETUP.md](docs/EBAY_SETUP.md) / [docs/HUGGINGFACE_SETUP.md](docs/HUGGINGFACE_SETUP.md)
for the two external API credentials it needs.

## Local development

1. Copy `.env.example` to `.env` and fill in the values (Hugging Face token,
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
