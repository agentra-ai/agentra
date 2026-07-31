#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env.worktree}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/secrets.sh
source "$SCRIPT_DIR/lib/secrets.sh"

shared_env_file="${AGENTRA_SHARED_ENV_FILE:-}"
if [ -z "$shared_env_file" ]; then
  primary_worktree="$(git worktree list --porcelain 2>/dev/null | awk '/^worktree / {sub(/^worktree /, ""); print; exit}')"
  if [ -n "$primary_worktree" ]; then
    shared_env_file="$primary_worktree/.env"
  fi
fi
if [ -z "$shared_env_file" ] || [ ! -f "$shared_env_file" ]; then
  echo "Shared main-checkout .env not found." >&2
  echo "Run ./scripts/bootstrap-env.sh in the primary checkout, or set AGENTRA_SHARED_ENV_FILE." >&2
  exit 1
fi

shared_value() {
  awk -F= -v key="$1" '$1 == key {sub(/^[^=]*=/, ""); value=$0; found++} END {if (found == 1) print value}' "$shared_env_file" | tr -d '\r'
}

postgres_user="$(shared_value POSTGRES_USER)"
postgres_password="$(shared_value POSTGRES_PASSWORD)"
postgres_port="$(shared_value POSTGRES_PORT)"
postgres_user="${postgres_user:-agentra}"
postgres_port="${postgres_port:-5432}"
if [ -z "$postgres_password" ]; then
  echo "POSTGRES_PASSWORD is missing from shared environment: $shared_env_file" >&2
  exit 1
fi
if [[ "$postgres_password" =~ [^a-zA-Z0-9._~-] ]]; then
  echo "Shared POSTGRES_PASSWORD must be URL-safe for worktree DATABASE_URL generation." >&2
  exit 1
fi

if [ -f "$ENV_FILE" ] && [ "${FORCE:-0}" != "1" ]; then
  echo "Refusing to overwrite existing $ENV_FILE. Re-run with FORCE=1 if you want to regenerate it."
  exit 1
fi

worktree_name="${WORKTREE_NAME:-$(basename "$PWD")}"
slug="$(printf '%s' "$worktree_name" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/_/g; s/__*/_/g; s/^_//; s/_$//')"
if [ -z "$slug" ]; then
  slug="agentra"
fi

hash_value="$(printf '%s' "$PWD" | cksum | awk '{print $1}')"
offset=$((hash_value % 1000))

postgres_db="agentra_${slug}_${offset}"
backend_port=$((18080 + offset))
frontend_port=$((13000 + offset))
local_bind_host="127.0.0.1"
callback_host="localhost"
api_origin="http://localhost:${backend_port}"
ws_origin="ws://localhost:${backend_port}/ws"
frontend_origin="http://localhost:${frontend_port}"
healthcheck_url="${api_origin}/readyz"
jwt_secret="$(generate_secret_hex 32)"

umask 077

cat > "$ENV_FILE" <<EOF
POSTGRES_DB=${postgres_db}
POSTGRES_USER=${postgres_user}
POSTGRES_PASSWORD=${postgres_password}
POSTGRES_PORT=${postgres_port}
DATABASE_URL=postgres://${postgres_user}:${postgres_password}@localhost:${postgres_port}/${postgres_db}?sslmode=disable

PORT=${backend_port}
SERVER_HEALTHCHECK_URL=${healthcheck_url}
JWT_SECRET=${jwt_secret}
AGENTRA_LOCAL_BIND_HOST=${local_bind_host}
AGENTRA_CALLBACK_HOST=${callback_host}
AGENTRA_SERVER_URL=${ws_origin}
AGENTRA_APP_URL=${frontend_origin}

GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URI=${frontend_origin}/auth/callback

FRONTEND_PORT=${frontend_port}
FRONTEND_ORIGIN=${frontend_origin}
NEXT_PUBLIC_SITE_URL=${frontend_origin}
NEXT_PUBLIC_API_URL=${api_origin}
NEXT_PUBLIC_WS_URL=${ws_origin}
NEXT_PUBLIC_CLI_CALLBACK_HOSTS=localhost,127.0.0.1
EOF

chmod 600 "$ENV_FILE"

echo "Generated $ENV_FILE for worktree '$worktree_name'"
echo "  Shared Postgres: localhost:${postgres_port}"
echo "  Database: ${postgres_db}"
echo "  Backend:  ${api_origin}"
echo "  Frontend: ${frontend_origin}"
echo ""
echo "Next steps:"
echo "  make setup-worktree"
echo "  make start-worktree"
