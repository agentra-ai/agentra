#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/secrets.sh
source "$SCRIPT_DIR/lib/secrets.sh"

usage() {
  echo "Usage: $0 [--check] [env-file]" >&2
}

env_value() {
  local env_file="$1"
  local key="$2"
  awk -F= -v key="$key" '$1 == key {sub(/^[^=]*=/, ""); value=$0; found++} END {if (found == 1) print value}' "$env_file" | tr -d '\r'
}

validate_env() {
  local env_file="$1"
  local mode=""
  local key=""
  local value=""
  local errors=0

  if [ ! -f "$env_file" ]; then
    echo "Environment file does not exist: $env_file" >&2
    return 1
  fi

  for key in POSTGRES_PASSWORD JWT_SECRET MINIO_ACCESS_KEY MINIO_SECRET_KEY DATABASE_URL; do
    value="$(env_value "$env_file" "$key")"
    if [ -z "$value" ]; then
      echo "Missing or duplicate required value: $key" >&2
      errors=$((errors + 1))
    fi
  done

  local postgres_password jwt_secret minio_access_key minio_secret_key database_url
  postgres_password="$(env_value "$env_file" POSTGRES_PASSWORD)"
  jwt_secret="$(env_value "$env_file" JWT_SECRET)"
  minio_access_key="$(env_value "$env_file" MINIO_ACCESS_KEY)"
  minio_secret_key="$(env_value "$env_file" MINIO_SECRET_KEY)"
  database_url="$(env_value "$env_file" DATABASE_URL)"

  case "$postgres_password" in
    ""|agentra|postgres|password|change-me*)
      echo "POSTGRES_PASSWORD is missing or uses a known default." >&2
      errors=$((errors + 1))
      ;;
  esac
  if [ "${#postgres_password}" -lt 32 ]; then
    echo "POSTGRES_PASSWORD must contain at least 32 characters." >&2
    errors=$((errors + 1))
  fi

  case "$jwt_secret" in
    ""|change-me*|agentra-dev-secret-change-in-production)
      echo "JWT_SECRET is missing or uses a known default." >&2
      errors=$((errors + 1))
      ;;
  esac
  if [ "${#jwt_secret}" -lt 64 ]; then
    echo "JWT_SECRET must contain at least 64 characters." >&2
    errors=$((errors + 1))
  fi

  case "$minio_access_key" in
    ""|agentra|minioadmin)
      echo "MINIO_ACCESS_KEY is missing or uses a known default." >&2
      errors=$((errors + 1))
      ;;
  esac
  case "$minio_secret_key" in
    ""|agentra|agentra123|minioadmin|password|change-me*)
      echo "MINIO_SECRET_KEY is missing or uses a known default." >&2
      errors=$((errors + 1))
      ;;
  esac
  if [ "${#minio_secret_key}" -lt 32 ]; then
    echo "MINIO_SECRET_KEY must contain at least 32 characters." >&2
    errors=$((errors + 1))
  fi

  if [ -n "$postgres_password" ] && [[ "$database_url" != *":${postgres_password}@"* ]]; then
    echo "DATABASE_URL does not contain the configured POSTGRES_PASSWORD." >&2
    errors=$((errors + 1))
  fi

  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) mode="" ;;
    *) mode="$(stat -c '%a' "$env_file" 2>/dev/null || stat -f '%Lp' "$env_file" 2>/dev/null || true)" ;;
  esac
  if [ -n "$mode" ]; then
    if [ "$mode" != "600" ]; then
      echo "Environment file permissions are $mode; expected 600." >&2
      errors=$((errors + 1))
    fi
  fi

  if [ "$errors" -ne 0 ]; then
    return 1
  fi
  echo "Environment file is secure: $env_file"
}

check_only=0
if [ "${1:-}" = "--check" ]; then
  check_only=1
  shift
fi
if [ "$#" -gt 1 ]; then
  usage
  exit 2
fi

target="${1:-$REPO_ROOT/.env}"
if [ "$check_only" = "1" ]; then
  validate_env "$target"
  exit 0
fi

if [ -e "$target" ]; then
  echo "Refusing to overwrite existing environment file: $target" >&2
  echo "Use '$0 --check $target' to validate it." >&2
  exit 1
fi
if [ ! -d "$(dirname "$target")" ]; then
  echo "Target directory does not exist: $(dirname "$target")" >&2
  exit 1
fi

template="$REPO_ROOT/.env.example"
if [ ! -f "$template" ]; then
  echo "Environment template not found: $template" >&2
  exit 1
fi

postgres_password="$(generate_secret_hex 32)"
jwt_secret="$(generate_secret_hex 32)"
minio_access_key="agentra_$(generate_secret_hex 8)"
minio_secret_key="$(generate_secret_hex 32)"
temporary="$(mktemp "${target}.tmp.XXXXXX")"
cleanup() {
  if [ -n "${temporary:-}" ] && [ -e "$temporary" ]; then
    rm -f -- "$temporary"
  fi
}
trap cleanup EXIT
umask 077

awk \
  -v postgres_password="$postgres_password" \
  -v jwt_secret="$jwt_secret" \
  -v minio_access_key="$minio_access_key" \
  -v minio_secret_key="$minio_secret_key" '
  BEGIN { FS=OFS="=" }
  $1 == "POSTGRES_PASSWORD" { print "POSTGRES_PASSWORD=" postgres_password; next }
  $1 == "DATABASE_URL" { print "DATABASE_URL=postgres://agentra:" postgres_password "@127.0.0.1:5432/agentra?sslmode=disable"; next }
  $1 == "JWT_SECRET" { print "JWT_SECRET=" jwt_secret; next }
  $1 == "MINIO_ACCESS_KEY" { print "MINIO_ACCESS_KEY=" minio_access_key; next }
  $1 == "MINIO_SECRET_KEY" { print "MINIO_SECRET_KEY=" minio_secret_key; next }
  { print }
' "$template" > "$temporary"

chmod 600 "$temporary"
validate_env "$temporary" >/dev/null
mv "$temporary" "$target"
temporary=""
trap - EXIT

echo "Generated secure environment file: $target"
echo "Secrets were written with owner-only permissions and were not printed."
echo "Next: docker compose up -d --build"
