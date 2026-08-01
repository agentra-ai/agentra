import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function invariant(condition, message) {
  if (!condition) throw new Error(message);
}

async function text(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}

const [goreleaser, ci, release, containerRelease, unixInstaller, windowsInstaller, updater, dockerfile] = await Promise.all([
  text(".goreleaser.yml"),
  text(".github/workflows/ci.yml"),
  text(".github/workflows/release.yml"),
  text(".github/workflows/docker.yml"),
  text("scripts/install.sh"),
  text("scripts/install.ps1"),
  text("server/internal/cli/update.go"),
  text("Dockerfile"),
]);

const installerMode = (await stat(path.join(repositoryRoot, "scripts/install.sh"))).mode;
invariant((installerMode & 0o111) !== 0, "scripts/install.sh must be executable");
invariant(/goos:\s*[\s\S]*- windows/.test(goreleaser), "GoReleaser must build Windows binaries");
invariant(/goos: windows\s+[\s\S]*formats:\s*\n\s*- zip/.test(goreleaser), "Windows releases must use zip archives");
invariant(goreleaser.includes('name_template: "checksums.txt"'), "GoReleaser checksums.txt is required");
invariant(goreleaser.includes("algorithm: sha256"), "GoReleaser release checksums must use SHA-256");
invariant(/checksum:[\s\S]*\.\/scripts\/install\.sh[\s\S]*\.\/scripts\/install\.ps1/.test(goreleaser), "Release checksums must cover both installers");
invariant(goreleaser.includes("sboms:") && goreleaser.includes("artifacts: archive") && goreleaser.includes(".spdx.json"), "GoReleaser must create archive SPDX SBOMs");
invariant(/signs:[\s\S]*cmd: cosign[\s\S]*artifacts: checksum/.test(goreleaser), "GoReleaser must keylessly sign checksums");
invariant(/signs:[\s\S]*cmd: cosign[\s\S]*artifacts: sbom/.test(goreleaser), "GoReleaser must keylessly sign SBOMs");
invariant(goreleaser.includes("homebrew_casks:"), "GoReleaser must publish the Homebrew cask");
invariant(goreleaser.includes("HOMEBREW_TAP_GITHUB_TOKEN"), "Homebrew publishing must use the dedicated tap token");
invariant(goreleaser.includes("./scripts/install.sh") && goreleaser.includes("./scripts/install.ps1"), "Release must upload both installers");
for (const symbol of ["buildinfo.Version", "buildinfo.Commit"]) {
  invariant(goreleaser.includes(symbol), `GoReleaser must inject ${symbol}`);
  invariant(dockerfile.includes(symbol), `Container builds must inject ${symbol}`);
}
invariant(dockerfile.includes("NEXT_PUBLIC_AGENTRA_VERSION=${VERSION}"), "Web containers must receive the canonical release version");
invariant(dockerfile.includes("NEXT_PUBLIC_AGENTRA_COMMIT=${COMMIT}"), "Web containers must receive the release commit");

for (const runner of ["ubuntu-latest", "macos-latest", "windows-latest"]) {
  invariant(ci.includes(runner), `CI installer matrix is missing ${runner}`);
}
invariant(ci.includes("docker://rhysd/actionlint:1.7.12"), "CI must lint workflow semantics with pinned actionlint");
invariant(ci.includes("test-installers.ps1"), "CI must execute the Windows installer fixture");
invariant(release.includes("goreleaser-action@v7") && release.includes("args: check"), "Release must validate GoReleaser configuration before publishing");
for (const permission of ["id-token: write", "attestations: write"]) {
  invariant(release.includes(permission), `CLI release workflow is missing ${permission}`);
  invariant(containerRelease.includes(permission), `Container release workflow is missing ${permission}`);
}
invariant(release.includes("download-syft@v0") && release.includes("syft-version: v1.50.0"), "CLI release must install a pinned Syft version");
invariant(release.includes("cosign-installer@v4.1.2"), "CLI release must install pinned Cosign");
invariant(release.includes("verify_release_artifacts.mjs --require-signatures") && release.includes("verify-release-signatures.sh"), "CLI release must verify its generated artifact set and signatures");
invariant(release.includes("actions/attest@v4") && release.includes("subject-checksums: ./dist/checksums.txt"), "CLI release must attest all checksummed subjects");
for (const workflow of [release, containerRelease]) {
  invariant(workflow.includes("id: release-metadata"), "Tagged release workflows must validate release metadata");
  invariant(workflow.includes('release_metadata.mjs --version --tag "$RELEASE_TAG"'), "Tagged release workflows must reject tags that differ from release metadata");
  invariant(workflow.includes("RELEASE_TAG: ${{ github.ref_name }}"), "Tagged release workflows must validate the pushed tag");
}

invariant(containerRelease.includes("ghcr.io/${{ github.repository_owner }}/agentra"), "Containers must publish to the project GHCR namespace");
invariant(containerRelease.includes("VERSION=${{ steps.release-metadata.outputs.version }}"), "Container builds must use the canonical metadata version");
invariant(!containerRelease.includes("VERSION=${{ github.ref_name }}"), "Container builds must not inject the raw Git tag as a version");
invariant(containerRelease.includes("linux/amd64,linux/arm64"), "Container images must publish amd64 and arm64 manifests");
invariant(containerRelease.includes("provenance: mode=max") && containerRelease.includes("sbom: true"), "Container builds must attach max provenance and SBOM attestations");
invariant(containerRelease.includes("cosign sign --yes") && containerRelease.includes("cosign-installer@v4.1.2"), "Container image digests must be keylessly signed");
invariant(containerRelease.includes("actions/attest@v4") && containerRelease.includes("subject-digest: ${{ steps.build.outputs.digest }}"), "Container digests must receive GitHub provenance attestations");
invariant(!containerRelease.includes("DOCKERHUB_TOKEN") && !containerRelease.includes("DOCKER_USERNAME"), "Container publishing must not depend on personal Docker Hub credentials");

invariant(unixInstaller.includes("sha256_file") && unixInstaller.includes("checksum mismatch"), "Unix installer must verify SHA-256");
invariant(windowsInstaller.includes("Get-FileHash -Algorithm SHA256") && windowsInstaller.includes("checksum mismatch"), "Windows installer must verify SHA-256");
invariant(updater.includes("verifyReleaseChecksum") && updater.includes("checksums.txt"), "CLI self-update must verify release checksums");

console.log("Release and installer contract valid.");
