#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

ENV_FILE=""
# Forge symlinks the shared .env relative to the site's configured Web
# Directory (frontend, for this monorepo), not the release/site root — see
# docs/DEPLOY.md. Check that first, then fall back to the older locations.
for candidate in "$ROOT/frontend/.env" "$ROOT/.env" "$ROOT/../.env"; do
  if [[ -f "$candidate" ]]; then
    ENV_FILE="$(cd "$(dirname "$candidate")" && pwd)/$(basename "$candidate")"
    break
  fi
done

if [[ -z "$ENV_FILE" ]]; then
  echo "No .env found (checked $ROOT/frontend/.env, $ROOT/.env, and $ROOT/../.env)" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source "$ENV_FILE"
set +a

cd "$ROOT/backend"
exec ./server
