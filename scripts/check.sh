#!/usr/bin/env bash
set -euo pipefail

# ==========================================================================
# Full verification pipeline: self-host contract → installer contract → capability contract → lint → typecheck → unit tests → Go tests → E2E
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
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required; run ./scripts/bootstrap-env.sh}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
PORT="${PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
LOCAL_BIND_HOST="${AGENTRA_LOCAL_BIND_HOST:-127.0.0.1}"
FRONTEND_BROWSER_HOST="${AGENTRA_FRONTEND_BROWSER_HOST:-localhost}"

# .env is also consumed by Docker Compose, where DATABASE_URL commonly uses
# the internal service hostname `postgres`. The verification services run on
# the host, so translate only that known Compose endpoint while preserving
# credentials, database name, and query parameters. External/custom URLs stay
# untouched, and CHECK_DATABASE_URL provides an explicit override.
if [ -n "${CHECK_DATABASE_URL:-}" ]; then
  DATABASE_URL="$CHECK_DATABASE_URL"
elif [ -n "${DATABASE_URL:-}" ]; then
  DATABASE_URL="${DATABASE_URL/@postgres:5432/@${LOCAL_BIND_HOST}:${POSTGRES_PORT}}"
else
  DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${LOCAL_BIND_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
fi
export DATABASE_URL

# Deployment env commonly points Next.js rewrites at the Compose hostname
# `server`, which is not resolvable by a frontend started on the host. Keep
# browser API calls same-origin, route those calls to the local check backend,
# and give Node-based E2E fixtures an explicit absolute URL.
LOCAL_API_BASE_URL="http://${LOCAL_BIND_HOST}:${PORT}"
REMOTE_API_URL="${CHECK_REMOTE_API_URL:-$LOCAL_API_BASE_URL}"
E2E_API_BASE="${CHECK_E2E_API_BASE:-$LOCAL_API_BASE_URL}"
NEXT_PUBLIC_API_URL="${CHECK_NEXT_PUBLIC_API_URL-}"
NEXT_PUBLIC_WS_URL="${CHECK_NEXT_PUBLIC_WS_URL:-ws://${LOCAL_BIND_HOST}:${PORT}/ws}"
export REMOTE_API_URL E2E_API_BASE NEXT_PUBLIC_API_URL NEXT_PUBLIC_WS_URL

# SERVER_HEALTHCHECK_URL and FRONTEND_ORIGIN are deployment/container values.
# Verification always targets the local services it starts unless explicit
# check URLs are provided.
BACKEND_HEALTHCHECK_URL="${CHECK_BACKEND_HEALTHCHECK_URL:-${LOCAL_API_BASE_URL}/livez}"
# Next.js dev treats 127.0.0.1 and localhost as different origins. Use its
# canonical local origin for browser requests while services remain bound to
# the loopback interface.
FRONTEND_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://${FRONTEND_BROWSER_HOST}:${FRONTEND_PORT}}"
PLAYWRIGHT_BASE_URL="$FRONTEND_BASE_URL"
FRONTEND_ORIGIN="${CHECK_FRONTEND_ORIGIN:-$FRONTEND_BASE_URL}"
export PLAYWRIGHT_BASE_URL FRONTEND_ORIGIN

BACKEND_PID=""
FRONTEND_PID=""
STARTED_BACKEND=false
STARTED_FRONTEND=false
EXIT_CODE=0

# Send a signal to a process and all descendants, deepest children first.
# pnpm and go run both spawn long-lived children; killing only their wrapper
# leaves the real dev server alive and makes cleanup wait forever.
signal_process_tree() {
  local root_pid=$1 signal_name=$2 child_pid
  for child_pid in $(pgrep -P "$root_pid" 2>/dev/null || true); do
    signal_process_tree "$child_pid" "$signal_name"
  done
  kill "-$signal_name" "$root_pid" 2>/dev/null || true
}

stop_process_tree() {
  local root_pid=$1 name=$2 attempt
  if ! kill -0 "$root_pid" 2>/dev/null; then
    return
  fi

  signal_process_tree "$root_pid" TERM
  for attempt in 1 2 3 4 5 6 7 8 9 10; do
    if ! kill -0 "$root_pid" 2>/dev/null; then
      break
    fi
    sleep 0.1
  done
  if kill -0 "$root_pid" 2>/dev/null; then
    signal_process_tree "$root_pid" KILL
  fi
  wait "$root_pid" 2>/dev/null || true
  echo "    Stopped $name (PID $root_pid)"
}

# --------------------------------------------------------------------------
# Cleanup: kill only services this script started
# --------------------------------------------------------------------------
cleanup() {
  local command_status=$?
  trap - EXIT
  if [ "$EXIT_CODE" -eq 0 ] && [ "$command_status" -ne 0 ]; then
    EXIT_CODE=$command_status
  fi

  echo ""
  if [ "$STARTED_BACKEND" = true ] && [ -n "$BACKEND_PID" ]; then
    stop_process_tree "$BACKEND_PID" "backend"
  fi
  if [ "$STARTED_FRONTEND" = true ] && [ -n "$FRONTEND_PID" ]; then
    stop_process_tree "$FRONTEND_PID" "frontend"
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

handle_signal() {
  EXIT_CODE=$1
  exit "$1"
}
trap 'handle_signal 130' INT
trap 'handle_signal 143' TERM

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

# Database-backed Go integration tests must never run against the developer's
# application database. Provision and migrate a sibling database by default;
# CI or custom environments can supply CHECK_TEST_DATABASE_URL explicitly.
if [ -n "${CHECK_TEST_DATABASE_URL:-}" ]; then
  TEST_DATABASE_URL="$CHECK_TEST_DATABASE_URL"
else
  TEST_POSTGRES_DB="${CHECK_TEST_POSTGRES_DB:-${POSTGRES_DB}_test}"
  bash scripts/ensure-postgres.sh "$ENV_FILE" "$TEST_POSTGRES_DB"
  DATABASE_URL_BASE="${DATABASE_URL%%\?*}"
  DATABASE_URL_QUERY="${DATABASE_URL#"$DATABASE_URL_BASE"}"
  TEST_DATABASE_URL="${DATABASE_URL_BASE%/*}/${TEST_POSTGRES_DB}${DATABASE_URL_QUERY}"
fi
export TEST_DATABASE_URL
(cd server && DATABASE_URL="$TEST_DATABASE_URL" go run ./cmd/migrate up)

# --------------------------------------------------------------------------
# Step 1: Self-host security contracts
# --------------------------------------------------------------------------
echo ""
echo "==> [1/9] Self-host security contracts..."
pnpm test:self-host || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 2: Installer and release contracts
# --------------------------------------------------------------------------
echo ""
echo "==> [2/9] Installer and release contracts..."
pnpm test:installers || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 3: Capability manifest
# --------------------------------------------------------------------------
echo ""
echo "==> [3/9] Capability manifest..."
pnpm test:capabilities || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 4: ESLint
# --------------------------------------------------------------------------
echo ""
echo "==> [4/9] ESLint..."
pnpm lint || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 5: TypeScript typecheck
# --------------------------------------------------------------------------
echo ""
echo "==> [5/9] TypeScript typecheck..."
pnpm typecheck || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 6: TypeScript unit tests (Vitest)
# --------------------------------------------------------------------------
echo ""
echo "==> [6/9] TypeScript unit tests..."
pnpm test || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 7: Go tests
# --------------------------------------------------------------------------
echo ""
echo "==> [7/9] Go tests..."
(cd server && go test ./...) || { EXIT_CODE=1; exit 1; }

# --------------------------------------------------------------------------
# Step 8: Start services for E2E (only if not already running)
# --------------------------------------------------------------------------
echo ""
echo "==> [8/9] Starting services for E2E..."

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
# Step 9: E2E tests (Playwright)
# --------------------------------------------------------------------------
echo ""
echo "==> [9/9] E2E tests (Playwright)..."
pnpm exec playwright test || { EXIT_CODE=1; exit 1; }
