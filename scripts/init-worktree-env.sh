#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env.worktree}"

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
postgres_port=5432
backend_port=$((18080 + offset))
frontend_port=$((13000 + offset))
local_bind_host="127.0.0.1"
callback_host="localhost"
api_origin="http://localhost:${backend_port}"
ws_origin="ws://localhost:${backend_port}/ws"
frontend_origin="http://localhost:${frontend_port}"
healthcheck_url="${api_origin}/health"

cat > "$ENV_FILE" <<EOF
POSTGRES_DB=${postgres_db}
POSTGRES_USER=agentra
POSTGRES_PASSWORD=agentra
POSTGRES_PORT=${postgres_port}
DATABASE_URL=postgres://agentra:agentra@localhost:${postgres_port}/${postgres_db}?sslmode=disable

PORT=${backend_port}
SERVER_HEALTHCHECK_URL=${healthcheck_url}
JWT_SECRET=change-me-in-production
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

echo "Generated $ENV_FILE for worktree '$worktree_name'"
echo "  Shared Postgres: localhost:${postgres_port}"
echo "  Database: ${postgres_db}"
echo "  Backend:  ${api_origin}"
echo "  Frontend: ${frontend_origin}"
echo ""
echo "Next steps:"
echo "  make setup-worktree"
echo "  make start-worktree"
