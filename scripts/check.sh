#!/usr/bin/env bash
set -euo pipefail

# ==========================================================================
# Full verification pipeline: typecheck → unit tests → Go tests → E2E
# Usage: bash scripts/check.sh
# ==========================================================================

ENV_FILE="${ENV_FILE:-.env}"
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
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
PORT="${PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
LOCAL_BIND_HOST="${AGENTRA_LOCAL_BIND_HOST:-127.0.0.1}"
BACKEND_HEALTHCHECK_URL="${SERVER_HEALTHCHECK_URL:-http://${LOCAL_BIND_HOST}:${PORT}/health}"
FRONTEND_BASE_URL="${PLAYWRIGHT_BASE_URL:-${FRONTEND_ORIGIN:-http://${LOCAL_BIND_HOST}:${FRONTEND_PORT}}}"
PLAYWRIGHT_BASE_URL="$FRONTEND_BASE_URL"
export PLAYWRIGHT_BASE_URL

BACKEND_PID=""
FRONTEND_PID=""
STARTED_BACKEND=false
STARTED_FRONTEND=false
EXIT_CODE=0

# --------------------------------------------------------------------------
# Cleanup: kill only services this script started
# --------------------------------------------------------------------------
cleanup() {
  echo ""
  if [ "$STARTED_BACKEND" = true ] && [ -n "$BACKEND_PID" ]; then
    kill "$BACKEND_PID" 2>/dev/null && wait "$BACKEND_PID" 2>/dev/null || true
    echo "    Stopped backend (PID $BACKEND_PID)"
  fi
  if [ "$STARTED_FRONTEND" = true ] && [ -n "$FRONTEND_PID" ]; then
    kill "$FRONTEND_PID" 2>/dev/null && wait "$FRONTEND_PID" 2>/dev/null || true
    echo "    Stopped frontend (PID $FRONTEND_PID)"
  fi
  echo ""
  if [ "$EXIT_CODE" -eq 0 ]; then
    echo "✓ All checks passed."
  else
    echo "✗ Checks FAILED."
  fi
  exit "$EXIT_CODE"
}
trap cleanup EXIT

# --------------------------------------------------------------------------
# Utility: wait until a URL responds
# --------------------------------------------------------------------------
wait_for_url() {
  local url=$1 name=$2 max_wait=${3:-60}
  local elapsed=0
  echo "    Waiting for $name at $url..."
  while ! curl -sf "$url" > /dev/null 2>&1; do
    sleep 1
    elapsed=$((elapsed + 1))
    if [ "$elapsed" -ge "$max_wait" ]; then
      echo "    ERROR: $name did not start within ${max_wait}s"
      EXIT_CODE=1
      exit 1
    fi
  done
  echo "    $name ready (${elapsed}s)"
}

# --------------------------------------------------------------------------
# Step 0: Ensure DB
# --------------------------------------------------------------------------
echo "==> Using env file: $ENV_FILE"
echo "==> Checking PostgreSQL..."
bash scripts/ensure-postgres.sh "$ENV_FILE"

# --------------------------------------------------------------------------
# Step 1: TypeScript typecheck
# --------------------------------------------------------------------------
echo ""
echo "==> [1/5] TypeScript typecheck..."
pnpm typecheck || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 2: TypeScript unit tests (Vitest)
# --------------------------------------------------------------------------
echo ""
echo "==> [2/5] TypeScript unit tests..."
pnpm test || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 3: Go tests
# --------------------------------------------------------------------------
echo ""
echo "==> [3/5] Go tests..."
(cd server && go test ./...) || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 4: Start services for E2E (only if not already running)
# --------------------------------------------------------------------------
echo ""
echo "==> [4/5] Starting services for E2E..."

if curl -sf "$BACKEND_HEALTHCHECK_URL" > /dev/null 2>&1; then
  echo "    Backend already running at $BACKEND_HEALTHCHECK_URL"
else
  echo "    Starting backend..."
  (cd server && go run ./cmd/server) > /tmp/agentra-check-backend.log 2>&1 &
  BACKEND_PID=$!
  STARTED_BACKEND=true
  wait_for_url "$BACKEND_HEALTHCHECK_URL" "Backend" 90
fi

if curl -sf "$FRONTEND_BASE_URL" > /dev/null 2>&1; then
  echo "    Frontend already running at $FRONTEND_BASE_URL"
else
  echo "    Starting frontend..."
  pnpm dev:web > /tmp/agentra-check-frontend.log 2>&1 &
  FRONTEND_PID=$!
  STARTED_FRONTEND=true
  wait_for_url "$FRONTEND_BASE_URL" "Frontend" 120
fi

# --------------------------------------------------------------------------
# Step 5: E2E tests (Playwright)
# --------------------------------------------------------------------------
echo ""
echo "==> [5/5] E2E tests (Playwright)..."
pnpm exec playwright test || { EXIT_CODE=1; exit 1; }
