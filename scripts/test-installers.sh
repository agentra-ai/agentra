#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="9.9.9-test"
TAG="v$VERSION"
TEMPORARY="$(mktemp -d "${TMPDIR:-/tmp}/agentra-installer-test.XXXXXX")"
cleanup() {
  rm -rf -- "$TEMPORARY"
}
trap cleanup EXIT

release_dir="$TEMPORARY/releases/$TAG"
mkdir -p "$release_dir"
fixture_dir="$TEMPORARY/fixture"
mkdir -p "$fixture_dir"
printf '%s\n' '#!/usr/bin/env sh' 'echo "agentra 9.9.9-test (commit: fixture)"' > "$fixture_dir/agentra"
chmod 0755 "$fixture_dir/agentra"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

: > "$release_dir/checksums.txt"
for platform in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
  asset="agentra_${platform}.tar.gz"
  tar -czf "$release_dir/$asset" -C "$fixture_dir" agentra
  printf '%s  %s\n' "$(sha256_file "$release_dir/$asset")" "$asset" >> "$release_dir/checksums.txt"
done

run_success_case() {
  os_name="$1"
  machine="$2"
  install_dir="$TEMPORARY/install-${os_name}-${machine}"
  AGENTRA_VERSION="$VERSION" \
  AGENTRA_RELEASE_BASE_URL="file://$TEMPORARY/releases" \
  AGENTRA_INSTALL_DIR="$install_dir" \
  AGENTRA_UNAME_S="$os_name" \
  AGENTRA_UNAME_M="$machine" \
    "$REPO_ROOT/scripts/install.sh" > "$TEMPORARY/output-${os_name}-${machine}.txt"
  test -x "$install_dir/agentra"
  "$install_dir/agentra" version | grep -q '9.9.9-test'
}

run_success_case Darwin x86_64
run_success_case Darwin arm64
run_success_case Linux x86_64
run_success_case Linux aarch64

tampered="$release_dir/agentra_linux_amd64.tar.gz"
printf 'tampered' >> "$tampered"
failed_install="$TEMPORARY/tampered-install"
mkdir -p "$failed_install"
printf '%s\n' '#!/usr/bin/env sh' 'echo sentinel' > "$failed_install/agentra"
chmod 0755 "$failed_install/agentra"
if AGENTRA_VERSION="$VERSION" \
  AGENTRA_RELEASE_BASE_URL="file://$TEMPORARY/releases" \
  AGENTRA_INSTALL_DIR="$failed_install" \
  AGENTRA_UNAME_S=Linux \
  AGENTRA_UNAME_M=x86_64 \
    "$REPO_ROOT/scripts/install.sh" > "$TEMPORARY/tampered.out" 2>&1; then
  echo "Expected tampered archive installation to fail." >&2
  exit 1
fi
grep -q 'checksum mismatch' "$TEMPORARY/tampered.out"
grep -q 'sentinel' "$failed_install/agentra"

if AGENTRA_VERSION="$VERSION" \
  AGENTRA_RELEASE_BASE_URL="file://$TEMPORARY/releases" \
  AGENTRA_INSTALL_DIR="$TEMPORARY/unsupported" \
  AGENTRA_UNAME_S=FreeBSD \
  AGENTRA_UNAME_M=x86_64 \
    "$REPO_ROOT/scripts/install.sh" > "$TEMPORARY/unsupported.out" 2>&1; then
  echo "Expected unsupported platform installation to fail." >&2
  exit 1
fi
grep -q 'unsupported operating system' "$TEMPORARY/unsupported.out"

echo "Unix installer contract tests passed."
