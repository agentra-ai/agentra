# Verifiable GHCR Images

Tagged releases publish three multi-architecture components to one official package:

```text
ghcr.io/agentra-ai/agentra:server-v0.6.0
ghcr.io/agentra-ai/agentra:gateway-v0.6.0
ghcr.io/agentra-ai/agentra:web-v0.6.0
```

Each tag contains `linux/amd64` and `linux/arm64` images. BuildKit attaches an SPDX SBOM and max-mode provenance; the workflow also adds a Cosign keyless signature and GitHub artifact attestation for the image-index digest.

The first public images with this contract appear after the next `v*` tag runs `.github/workflows/docker.yml`. Older Docker Hub images do not satisfy this contract.

## Pull and verify

```bash
VERSION=v0.6.0
IMAGE="ghcr.io/agentra-ai/agentra:server-$VERSION"

docker pull "$IMAGE"
cosign verify "$IMAGE" \
  --certificate-identity "https://github.com/agentra-ai/agentra/.github/workflows/docker.yml@refs/tags/$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify "oci://$IMAGE" -R agentra-ai/agentra
```

Pin the resolved `sha256:` digest in production deployments. Rolling `*-latest` and minor-version tags are convenience selectors, not immutable deployment evidence.

See [Deployment and Release Guide](../DEPLOYMENT.md) for the complete release and self-host workflow.
