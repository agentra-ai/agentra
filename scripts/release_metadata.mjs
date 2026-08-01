import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const repositoryRoot = path.resolve(import.meta.dirname, "..");
const metadataPath = path.join(repositoryRoot, "release", "metadata.json");

function invariant(condition, message) {
  if (!condition) throw new Error(message);
}

const metadata = JSON.parse(await readFile(metadataPath, "utf8"));
invariant(metadata.schema_version === 1, "release metadata schema_version must be 1");
invariant(
  /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/.test(metadata.version),
  `invalid release version: ${metadata.version}`,
);
invariant(/^\d{4}-\d{2}-\d{2}$/.test(metadata.release_date), "release_date must use YYYY-MM-DD");
invariant(["stable", "prerelease"].includes(metadata.channel), `invalid release channel: ${metadata.channel}`);

const version = metadata.version;
const tag = `v${version}`;
const args = process.argv.slice(2);
const sync = args.includes("--sync");
const printVersion = args.includes("--version");
const tagIndex = args.indexOf("--tag");
if (tagIndex >= 0) {
  const suppliedTag = args[tagIndex + 1];
  invariant(suppliedTag === tag, `release tag ${suppliedTag || "<missing>"} does not match metadata tag ${tag}`);
}

const releaseURL = `https://github.com/agentra-ai/agentra/releases/tag/${tag}`;
const markedFiles = [
  {
    path: "README.md",
    block: `<!-- RELEASE_METADATA_START -->\nCurrent release: [${tag}](${releaseURL}) · Released ${metadata.release_date}\n<!-- RELEASE_METADATA_END -->`,
  },
  {
    path: "README.zh-CN.md",
    block: `<!-- RELEASE_METADATA_START -->\n当前版本：[${tag}](${releaseURL}) · 发布于 ${metadata.release_date}\n<!-- RELEASE_METADATA_END -->`,
  },
  {
    path: "docs/ROADMAP.md",
    block: `<!-- RELEASE_METADATA_START -->\n- Current Release: [${tag}](${releaseURL}) (${metadata.release_date}, ${metadata.channel})\n<!-- RELEASE_METADATA_END -->`,
  },
];

function replaceMarkedBlock(content, expectedBlock, relativePath) {
  const start = "<!-- RELEASE_METADATA_START -->";
  const end = "<!-- RELEASE_METADATA_END -->";
  const startIndex = content.indexOf(start);
  const endIndex = content.indexOf(end, startIndex + start.length);
  invariant(startIndex >= 0 && endIndex > startIndex, `${relativePath} is missing release metadata markers`);
  return content.slice(0, startIndex) + expectedBlock + content.slice(endIndex + end.length);
}

for (const entry of markedFiles) {
  const absolutePath = path.join(repositoryRoot, entry.path);
  const content = await readFile(absolutePath, "utf8");
  const expected = replaceMarkedBlock(content, entry.block, entry.path);
  if (sync) {
    if (content !== expected) await writeFile(absolutePath, expected);
  } else {
    invariant(content === expected, `${entry.path} release metadata is stale; run pnpm sync:release-metadata`);
  }
}

for (const relativePath of ["package.json", "apps/web/package.json"]) {
  const absolutePath = path.join(repositoryRoot, relativePath);
  const packageJSON = JSON.parse(await readFile(absolutePath, "utf8"));
  if (sync) {
    if (packageJSON.version !== version) {
      packageJSON.version = version;
      await writeFile(absolutePath, `${JSON.stringify(packageJSON, null, 2)}\n`);
    }
  } else {
    invariant(packageJSON.version === version, `${relativePath} version ${packageJSON.version} does not match ${version}`);
  }
}

if (printVersion) {
  process.stdout.write(`${version}\n`);
} else if (!sync) {
  console.log(`Release metadata valid: ${tag}`);
}
