#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BOOTSTRAP="$REPO_ROOT/scripts/bootstrap-env.sh"
temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/agentra-bootstrap-test.XXXXXX")"
cleanup() {
  case "$temporary_root" in
    "${TMPDIR:-/tmp}"/agentra-bootstrap-test.*) rm -rf -- "$temporary_root" ;;
    *) echo "Refusing to clean unexpected test path: $temporary_root" >&2 ;;
  esac
}
trap cleanup EXIT

env_one="$temporary_root/one.env"
env_two="$temporary_root/two.env"
output="$($BOOTSTRAP "$env_one")"

value() {
  awk -F= -v key="$2" '$1 == key {sub(/^[^=]*=/, ""); print; exit}' "$1"
}

postgres_one="$(value "$env_one" POSTGRES_PASSWORD)"
jwt_one="$(value "$env_one" JWT_SECRET)"
minio_access_one="$(value "$env_one" MINIO_ACCESS_KEY)"
minio_secret_one="$(value "$env_one" MINIO_SECRET_KEY)"

for secret in "$postgres_one" "$jwt_one" "$minio_access_one" "$minio_secret_one"; do
  if [ -z "$secret" ]; then
    echo "Generated secret is empty." >&2
    exit 1
  fi
  if [[ "$output" == *"$secret"* ]]; then
    echo "Bootstrap output leaked a generated secret." >&2
    exit 1
  fi
done

mode="$(stat -c '%a' "$env_one" 2>/dev/null || stat -f '%Lp' "$env_one")"
if [ "$mode" != "600" ]; then
  echo "Generated file mode is $mode, want 600." >&2
  exit 1
fi

"$BOOTSTRAP" --check "$env_one" >/dev/null
checksum_before="$(cksum "$env_one")"
if "$BOOTSTRAP" "$env_one" >/dev/null 2>&1; then
  echo "Bootstrap overwrote an existing environment file." >&2
  exit 1
fi
if [ "$(cksum "$env_one")" != "$checksum_before" ]; then
  echo "Existing environment file changed after refused overwrite." >&2
  exit 1
fi

"$BOOTSTRAP" "$env_two" >/dev/null
if [ "$(value "$env_two" POSTGRES_PASSWORD)" = "$postgres_one" ] ||
   [ "$(value "$env_two" JWT_SECRET)" = "$jwt_one" ] ||
   [ "$(value "$env_two" MINIO_SECRET_KEY)" = "$minio_secret_one" ]; then
  echo "Independent bootstraps reused a secret." >&2
  exit 1
fi

known="$temporary_root/known.env"
printf '%s\n' \
  'POSTGRES_PASSWORD=agentra' \
  'DATABASE_URL=postgres://agentra:agentra@127.0.0.1:5432/agentra' \
  'JWT_SECRET=change-me-in-production' \
  'MINIO_ACCESS_KEY=agentra' \
  'MINIO_SECRET_KEY=agentra123' > "$known"
chmod 600 "$known"
if "$BOOTSTRAP" --check "$known" >/dev/null 2>&1; then
  echo "Validator accepted known default credentials." >&2
  exit 1
fi

worktree_env="$temporary_root/worktree.env"
AGENTRA_SHARED_ENV_FILE="$env_one" WORKTREE_NAME=bootstrap-test "$REPO_ROOT/scripts/init-worktree-env.sh" "$worktree_env" >/dev/null
worktree_jwt="$(value "$worktree_env" JWT_SECRET)"
if [ "${#worktree_jwt}" -lt 64 ] || [ "$worktree_jwt" = "change-me-in-production" ]; then
  echo "Worktree generator did not create a strong JWT secret." >&2
  exit 1
fi
worktree_mode="$(stat -c '%a' "$worktree_env" 2>/dev/null || stat -f '%Lp' "$worktree_env")"
if [ "$worktree_mode" != "600" ]; then
  echo "Worktree environment mode is $worktree_mode, want 600." >&2
  exit 1
fi
if [ "$(value "$worktree_env" POSTGRES_PASSWORD)" != "$postgres_one" ]; then
  echo "Worktree environment did not reuse the shared PostgreSQL credential." >&2
  exit 1
fi

make_env="$temporary_root/make.env"
make -s -C "$REPO_ROOT" ensure-env MAIN_ENV_FILE="$make_env" ENV_FILE="$make_env" >/dev/null
"$BOOTSTRAP" --check "$make_env" >/dev/null

echo "Secure environment bootstrap tests passed."
