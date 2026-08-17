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

# .env is a Forge "shared path", symlinked into every release directory
# automatically (Forge does this by default for zero-downtime sites). Fail
# loudly and immediately if it's missing rather than silently proceeding —
# every later failure this could cause (e.g. cmd/migrate's "DATABASE_URL is
# required") is far more confusing to debug from the deploy log alone.
if [[ ! -f "$ROOT/.env" ]]; then
  echo "ERROR: $ROOT/.env not found. Forge should symlink it here as a" >&2
  echo "shared path before running this script — check the site's" >&2
  echo "Deployments settings and confirm .env is listed there, and that" >&2
  echo "the deploy log shows a 'Linking environment file' step." >&2
  echo "Contents of $ROOT:" >&2
  ls -la "$ROOT" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source "$ROOT/.env"
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
