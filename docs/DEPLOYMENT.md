# Deployment and Release Guide — Agentra

Agentra supports two paths:

- Tagged releases publish verifiable CLI assets to GitHub Releases and multi-architecture images to GHCR.
- Self-hosters can build the same source locally with Docker Compose.

The supply-chain workflow described here becomes public with the next `v*` tag. Releases created before that tag may not contain SBOMs, Sigstore bundles, GHCR images, or GitHub attestations.

## Tagged release pipeline

Pushing a semantic tag triggers two independent workflows:

```text
vX.Y.Z
  ├─ release.yml
  │    ├─ Darwin/Linux/Windows CLI archives (amd64 + arm64)
  │    ├─ SHA-256 checksums covering archives, SBOMs, and installers
  │    ├─ SPDX 2.3 SBOM per archive
  │    ├─ Cosign keyless bundles for checksums and SBOMs
  │    ├─ GitHub build-provenance attestation
  │    └─ Homebrew Cask update
  └─ docker.yml
       ├─ server, gateway, and web images
       ├─ linux/amd64 + linux/arm64 manifest per image tag
       ├─ BuildKit SBOM and max-mode provenance
       ├─ Cosign keyless signature for each image digest
       └─ GitHub registry-backed provenance attestation
```

Create and push a tag only after the repository checks pass:

```bash
make check
VERSION="v$(node scripts/release_metadata.mjs --version)"
node scripts/release_metadata.mjs --tag "$VERSION"
git tag "$VERSION"
git push origin "$VERSION"
```

The container workflow uses the repository-scoped `GITHUB_TOKEN`; no personal Docker Hub credentials are required. The only cross-repository secret is `HOMEBREW_TAP_GITHUB_TOKEN`, which needs permission to update `agentra-ai/homebrew-tap`.

### Published image tags

All components share the official package `ghcr.io/agentra-ai/agentra`:

```bash
VERSION=vX.Y.Z
docker pull "ghcr.io/agentra-ai/agentra:server-$VERSION"
docker pull "ghcr.io/agentra-ai/agentra:gateway-$VERSION"
docker pull "ghcr.io/agentra-ai/agentra:web-$VERSION"

# Stable releases also publish rolling aliases.
docker pull ghcr.io/agentra-ai/agentra:server-vX.Y
docker pull ghcr.io/agentra-ai/agentra:server-latest
```

Each tag is a multi-platform manifest for `linux/amd64` and `linux/arm64`.

## Verify a CLI release

Install Cosign and GitHub CLI from their official distributions, then download the archive, checksum file, and checksum bundle from the same release. The workflow identity is intentionally pinned to this repository and tag:

```bash
VERSION=vX.Y.Z
ASSET=agentra_linux_amd64.tar.gz
BASE="https://github.com/agentra-ai/agentra/releases/download/$VERSION"

curl -fLO "$BASE/$ASSET"
curl -fLO "$BASE/checksums.txt"
curl -fLO "$BASE/checksums.txt.sigstore.json"

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/agentra-ai/agentra/.github/workflows/release.yml@refs/tags/$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt

grep "  $ASSET$" checksums.txt | shasum -a 256 -c -
gh attestation verify "$ASSET" -R agentra-ai/agentra
```

Every archive also has a sibling `${ASSET}.spdx.json` SBOM and `${ASSET}.spdx.json.sigstore.json` bundle. Verify it with the same `cosign verify-blob` identity before inspecting its dependency inventory.

The shell and PowerShell installers always enforce SHA-256 integrity. Cosign and GitHub provenance verification are explicit because a clean machine cannot securely bootstrap those independent verification tools from the artifact it is trying to verify.

## Verify a container image

```bash
VERSION=vX.Y.Z
IMAGE="ghcr.io/agentra-ai/agentra:server-$VERSION"

docker pull "$IMAGE"

cosign verify "$IMAGE" \
  --certificate-identity "https://github.com/agentra-ai/agentra/.github/workflows/docker.yml@refs/tags/$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

gh attestation verify "oci://$IMAGE" -R agentra-ai/agentra
docker buildx imagetools inspect "$IMAGE"
```

The BuildKit SBOM and provenance are attached to the OCI image index. Verification must use the image digest resolved from the selected tag; never treat a mutable tag alone as an audit record.

## Docker Compose self-host

Prerequisites: Docker 24+, Docker Compose v2, and at least 4 GB RAM.

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra

# Generates independent PostgreSQL, JWT, and MinIO credentials in a 0600 file.
./scripts/bootstrap-env.sh

# Review public URLs and optional integrations before starting.
nano .env
docker compose up -d --build
```

The default profile keeps PostgreSQL and MinIO internal, binds Web/API to loopback, and does not start Adminer. Use `--profile debug` only when that privileged surface is intentionally required.

Verify the deployment:

```bash
curl http://127.0.0.1:8080/livez
curl http://127.0.0.1:8080/readyz
open http://127.0.0.1:3000
docker compose run --rm migrate
```

Install the host-side daemon separately and connect it to the self-hosted service:

```bash
curl -fsSLO https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.sh
sh install.sh
rm install.sh
agentra setup --deployment self-host
```

Windows users run `scripts/install.ps1`; Homebrew users run `brew install --cask agentra-ai/tap/agentra`.

## Secrets reference

| Secret | Purpose | Source |
|---|---|---|
| `JWT_SECRET` | API authentication | `scripts/bootstrap-env.sh` |
| `POSTGRES_PASSWORD` | PostgreSQL | `scripts/bootstrap-env.sh` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | object storage | `scripts/bootstrap-env.sh` |
| `RESEND_API_KEY` | email OTP when `EMAIL_PROVIDER=resend` | Resend account |
| `SMTP_PASSWORD` | email OTP when `EMAIL_PROVIDER=smtp` and authentication is enabled | SMTP provider |
| `GOOGLE_CLIENT_*` | optional OAuth | Google Cloud console |
| `HOMEBREW_TAP_GITHUB_TOKEN` | release workflow only | fine-grained token scoped to the tap repository |

GHCR publishing, Cosign keyless signing, and GitHub attestations use short-lived workflow OIDC plus `GITHUB_TOKEN`; they do not require stored signing keys.

For a public deployment, set `APP_ENV=production`, `EMAIL_PROVIDER` to `resend`
or `smtp`, and `EMAIL_FROM` to a sender on a verified domain. SMTP supports
`SMTP_TLS_MODE=starttls` (default), implicit TLS with `tls`, and unauthenticated
private relays with `none`. Registration can be restricted with
`AGENTRA_SIGNUP_DISABLED`, `AGENTRA_SIGNUP_ALLOWLIST`, and
`AGENTRA_WORKSPACE_CREATION_DISABLED`; invitations remain usable when public
signup is disabled.

## Current security boundary

- Release archives, checksums, SBOMs, and OCI image digests receive identity-bound supply-chain signatures/attestations on the next tag.
- macOS code signing/notarization and Windows Authenticode are separate platform trust systems and are not implemented yet.
- Published release verification proves workflow identity and artifact integrity; it does not replace review of the tagged source or runtime hardening.
