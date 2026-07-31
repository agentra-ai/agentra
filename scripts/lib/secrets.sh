#!/usr/bin/env bash

generate_secret_hex() {
  local bytes="${1:?byte count is required}"
  local value=""

  if command -v openssl >/dev/null 2>&1; then
    value="$(openssl rand -hex "$bytes")"
  elif [ -r /dev/urandom ] && command -v od >/dev/null 2>&1; then
    value="$(LC_ALL=C od -An -N "$bytes" -tx1 /dev/urandom | tr -d ' \n')"
  else
    echo "No cryptographically secure random source found (requires openssl or /dev/urandom + od)." >&2
    return 1
  fi

  if [ "${#value}" -ne "$((bytes * 2))" ]; then
    echo "Secure random generator returned an unexpected length." >&2
    return 1
  fi
  printf '%s' "$value"
}
