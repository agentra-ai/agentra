#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing env file: $ENV_FILE"
  echo "Create .env from .env.example, or run 'make worktree-env' and use .env.worktree."
  exit 1
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

POSTGRES_DB="${POSTGRES_DB:-agentra}"
POSTGRES_USER="${POSTGRES_USER:-agentra}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required; run ./scripts/bootstrap-env.sh}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
TARGET_POSTGRES_DB="${2:-$POSTGRES_DB}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$REPO_ROOT/docker-compose.yml" -f "$REPO_ROOT/docker-compose.dev.yml" --env-file "$ENV_FILE")

export PGPASSWORD="$POSTGRES_PASSWORD"

echo "==> Ensuring shared PostgreSQL container is running on configured port ${POSTGRES_PORT}..."
"${COMPOSE[@]}" up -d postgres

echo "==> Waiting for PostgreSQL to be ready..."
until "${COMPOSE[@]}" exec -T postgres pg_isready -U "$POSTGRES_USER" -d postgres > /dev/null 2>&1; do
  sleep 1
done

echo "==> Ensuring database '$TARGET_POSTGRES_DB' exists..."
db_exists="$("${COMPOSE[@]}" exec -T postgres \
  psql -U "$POSTGRES_USER" -d postgres -Atqc "SELECT 1 FROM pg_database WHERE datname = '$TARGET_POSTGRES_DB'")"

if [ "$db_exists" != "1" ]; then
	"${COMPOSE[@]}" exec -T postgres \
		psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 \
		-c "CREATE DATABASE \"$TARGET_POSTGRES_DB\"" \
		> /dev/null
fi

echo "✓ PostgreSQL ready. Database: $TARGET_POSTGRES_DB"
