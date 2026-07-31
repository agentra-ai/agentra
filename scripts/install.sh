#!/usr/bin/env sh
set -eu

REPOSITORY="agentra-ai/agentra"
DEFAULT_RELEASE_BASE_URL="https://github.com/${REPOSITORY}/releases/download"

say() {
  printf '%s\n' "$*"
}

fail() {
  printf 'agentra installer: %s\n' "$*" >&2
  exit 1
}

download() {
  source_url="$1"
  destination="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --connect-timeout 10 "$source_url" -o "$destination"
  elif command -v wget >/dev/null 2>&1; then
    wget -q --https-only --tries=3 --timeout=20 -O "$destination" "$source_url"
  else
    fail "curl or wget is required"
  fi
}

normalize_platform() {
  raw_os="${AGENTRA_UNAME_S:-$(uname -s)}"
  case "$raw_os" in
    Darwin) platform_os="darwin" ;;
    Linux) platform_os="linux" ;;
    *) fail "unsupported operating system: $raw_os" ;;
  esac

  raw_arch="${AGENTRA_UNAME_M:-$(uname -m)}"
  case "$raw_arch" in
    x86_64|amd64) platform_arch="amd64" ;;
    arm64|aarch64) platform_arch="arm64" ;;
    *) fail "unsupported architecture: $raw_arch" ;;
  esac
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    fail "sha256sum or shasum is required to verify the release"
  fi
}

resolve_install_dir() {
  if [ -n "${AGENTRA_INSTALL_DIR:-}" ]; then
    install_dir="$AGENTRA_INSTALL_DIR"
    return
  fi
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    install_dir="/usr/local/bin"
  else
    : "${HOME:?HOME is required when /usr/local/bin is not writable}"
    install_dir="$HOME/.local/bin"
  fi
}

temporary="$(mktemp -d "${TMPDIR:-/tmp}/agentra-install.XXXXXX")"
cleanup() {
  if [ -n "${temporary:-}" ] && [ -d "$temporary" ]; then
    rm -rf -- "$temporary"
  fi
}
trap cleanup EXIT HUP INT TERM

normalize_platform
resolve_install_dir

version="${AGENTRA_VERSION:-}"
if [ -z "$version" ]; then
  metadata="$temporary/latest.json"
  download "https://api.github.com/repos/${REPOSITORY}/releases/latest" "$metadata"
  version="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$metadata" | head -n 1)"
fi
version="${version#v}"
case "$version" in
  ""|*[!0-9A-Za-z._-]*) fail "invalid release version: $version" ;;
esac
tag="v$version"

release_base_url="${AGENTRA_RELEASE_BASE_URL:-$DEFAULT_RELEASE_BASE_URL}"
release_base_url="${release_base_url%/}"
release_url="$release_base_url/$tag"
asset="agentra_${platform_os}_${platform_arch}.tar.gz"
archive="$temporary/$asset"
checksums="$temporary/checksums.txt"

say "Installing Agentra $tag for ${platform_os}/${platform_arch}..."
download "$release_url/$asset" "$archive"
download "$release_url/checksums.txt" "$checksums"

expected="$(awk -v asset="$asset" '$2 == asset {hash=$1; count++} END {if (count == 1) print hash}' "$checksums")"
[ -n "$expected" ] || fail "checksums.txt does not contain exactly one entry for $asset"
actual="$(sha256_file "$archive")"
if [ "$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')" != "$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')" ]; then
  fail "SHA-256 checksum mismatch for $asset"
fi

tar -xzf "$archive" -C "$temporary" agentra
[ -f "$temporary/agentra" ] || fail "release archive does not contain agentra"

mkdir -p "$install_dir"
[ -w "$install_dir" ] || fail "install directory is not writable: $install_dir (set AGENTRA_INSTALL_DIR)"
target="$install_dir/agentra"
staged="$install_dir/.agentra.install.$$"
cp "$temporary/agentra" "$staged"
chmod 0755 "$staged"
mv -f "$staged" "$target"

say "Installed $target"
if "$target" version >/dev/null 2>&1; then
  say "Verified: $("$target" version 2>/dev/null | head -n 1)"
else
  fail "installed binary did not pass 'agentra version'"
fi

case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *) say "Add $install_dir to PATH before running agentra." ;;
esac
say "Next: agentra setup --deployment self-host"
