#!/bin/sh
set -eu

dist_directory="${1:-dist}"
issuer="${AGENTRA_CERTIFICATE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"
identity="${AGENTRA_CERTIFICATE_IDENTITY:-}"

[ -n "$identity" ] || {
  echo "AGENTRA_CERTIFICATE_IDENTITY is required" >&2
  exit 1
}
command -v cosign >/dev/null 2>&1 || {
  echo "cosign is required" >&2
  exit 1
}

verify_blob() {
  artifact="$1"
  bundle="${artifact}.sigstore.json"
  [ -f "$artifact" ] || {
    echo "Missing signed artifact: $artifact" >&2
    exit 1
  }
  [ -f "$bundle" ] || {
    echo "Missing Sigstore bundle: $bundle" >&2
    exit 1
  }
  cosign verify-blob \
    --bundle "$bundle" \
    --certificate-identity "$identity" \
    --certificate-oidc-issuer "$issuer" \
    "$artifact"
}

verify_blob "$dist_directory/checksums.txt"

sbom_count=0
for sbom in "$dist_directory"/*.spdx.json; do
  [ -f "$sbom" ] || continue
  verify_blob "$sbom"
  sbom_count=$((sbom_count + 1))
done

[ "$sbom_count" -eq 6 ] || {
  echo "Expected 6 signed SBOMs, found $sbom_count" >&2
  exit 1
}

echo "Verified checksums.txt and $sbom_count SBOM Sigstore bundles."
