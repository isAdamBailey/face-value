#!/usr/bin/env bash
set -euo pipefail

# Forge always uses zero-downtime deployments for Nuxt sites (mandatory,
# not configurable). By the time this script runs, Forge has already
# cloned the new release and `cd`'d into it — see the site's Deploy
# Script, which calls this from inside the $CREATE_RELEASE() block. Do
# not `git pull` here: there is nothing to pull, and this is not
# necessarily the same directory as $FORGE_SITE_PATH (which points at
# `current`, i.e. the *previous* release, until $ACTIVATE_RELEASE() runs).
ROOT="$PWD"

export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

# .env is a Forge "shared path", symlinked in automatically for
# zero-downtime sites. Since the site's Web Directory is "frontend" (see
# docs/DEPLOY.md), Forge links it relative to that, not the repo root —
# check both rather than assume. Fail loudly and immediately if neither
# exists rather than silently proceeding: every later failure this could
# cause (e.g. cmd/migrate's "DATABASE_URL is required") is far more
# confusing to debug from the deploy log alone.
ENV_CANDIDATES=("$ROOT/frontend/.env" "$ROOT/.env")
ENV_FILE=""
for candidate in "${ENV_CANDIDATES[@]}"; do
  if [[ -f "$candidate" ]]; then
    ENV_FILE="$candidate"
    break
  fi
done

if [[ -z "$ENV_FILE" ]]; then
  echo "ERROR: no .env found at any of: ${ENV_CANDIDATES[*]}" >&2
  echo "Check the site's Deployments settings and confirm .env is listed" >&2
  echo "as a shared path, and that the deploy log shows a 'Linking" >&2
  echo "environment file' step." >&2
  echo "Contents of $ROOT:" >&2
  ls -la "$ROOT" >&2
  exit 1
fi

echo "Using env file: $ENV_FILE"
set -a
# shellcheck disable=SC1091
source "$ENV_FILE"
set +a

cd "$ROOT/backend"
go build -o server ./cmd/server
go run ./cmd/migrate up
cd "$ROOT"

cd "$ROOT/frontend"
npm ci
npm run build
cd "$ROOT"

chmod +x scripts/run-api.sh
