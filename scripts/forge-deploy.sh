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
# automatically (Forge does this by default for zero-downtime sites).
if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

cd "$ROOT/backend"
go build -o server ./cmd/server
DATABASE_URL="$DATABASE_URL" go run ./cmd/migrate up
cd "$ROOT"

cd "$ROOT/frontend"
npm ci
npm run build
cd "$ROOT"

chmod +x scripts/run-api.sh
