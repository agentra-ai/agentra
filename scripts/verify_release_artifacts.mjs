import { createHash } from "node:crypto";
import { access, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const repositoryRoot = path.resolve(import.meta.dirname, "..");
const distDirectory = path.join(repositoryRoot, "dist");
const requireSignatures = process.argv.includes("--require-signatures");
const platforms = [
  ["darwin", "amd64", "tar.gz"],
  ["darwin", "arm64", "tar.gz"],
  ["linux", "amd64", "tar.gz"],
  ["linux", "arm64", "tar.gz"],
  ["windows", "amd64", "zip"],
  ["windows", "arm64", "zip"],
];

function invariant(condition, message) {
  if (!condition) throw new Error(message);
}

const archives = platforms.map(([operatingSystem, architecture, extension]) =>
  `agentra_${operatingSystem}_${architecture}.${extension}`,
);
const sboms = archives.map((archive) => `${archive}.spdx.json`);
const expectedNames = [...archives, ...sboms, "install.ps1", "install.sh"].sort();

const checksumText = await readFile(path.join(distDirectory, "checksums.txt"), "utf8");
const entries = checksumText
  .trim()
  .split(/\r?\n/)
  .map((line) => {
    const match = /^([a-f0-9]{64})\s{2}([^/\\]+)$/.exec(line);
    invariant(match, `invalid checksums.txt line: ${line}`);
    return { digest: match[1], name: match[2] };
  });

const names = entries.map(({ name }) => name);
invariant(new Set(names).size === names.length, "checksums.txt contains duplicate asset names");
invariant(
  JSON.stringify([...names].sort()) === JSON.stringify(expectedNames),
  `checksums.txt asset set mismatch: ${names.sort().join(", ")}`,
);

for (const entry of entries) {
  const filePath = entry.name.startsWith("install.")
    ? path.join(repositoryRoot, "scripts", entry.name)
    : path.join(distDirectory, entry.name);
  const contents = await readFile(filePath);
  const actual = createHash("sha256").update(contents).digest("hex");
  invariant(actual === entry.digest, `SHA-256 mismatch for ${entry.name}`);
}

for (const [index, sbomName] of sboms.entries()) {
  const sbom = JSON.parse(await readFile(path.join(distDirectory, sbomName), "utf8"));
  invariant(sbom.spdxVersion === "SPDX-2.3", `${sbomName} must use SPDX 2.3`);
  invariant(sbom.name === archives[index], `${sbomName} describes ${sbom.name}, expected ${archives[index]}`);
  invariant(Array.isArray(sbom.packages) && sbom.packages.length > 0, `${sbomName} has no packages`);
}

if (requireSignatures) {
  for (const name of ["checksums.txt", ...sboms]) {
    const bundlePath = path.join(distDirectory, `${name}.sigstore.json`);
    await access(bundlePath);
    const bundle = JSON.parse(await readFile(bundlePath, "utf8"));
    invariant(bundle && typeof bundle === "object", `${path.basename(bundlePath)} is not a Sigstore bundle`);
  }
}

console.log(
  `Release artifacts valid: ${archives.length} archives, ${sboms.length} SPDX SBOMs, 2 installers${
    requireSignatures ? ", and 7 Sigstore bundles" : ""
  }`,
);
