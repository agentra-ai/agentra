import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const repositoryRoot = path.resolve(import.meta.dirname, "..");
const scriptPath = path.join(repositoryRoot, "scripts", "release_metadata.mjs");
const metadata = JSON.parse(
  await readFile(path.join(repositoryRoot, "release", "metadata.json"), "utf8"),
);
const tag = `v${metadata.version}`;

function invariant(condition, message) {
  if (!condition) throw new Error(message);
}

const valid = await execFileAsync(process.execPath, [
  scriptPath,
  "--version",
  "--tag",
  tag,
]);
invariant(
  valid.stdout.trim() === metadata.version,
  `canonical version output was ${JSON.stringify(valid.stdout.trim())}`,
);

let rejected = false;
try {
  await execFileAsync(process.execPath, [
    scriptPath,
    "--version",
    "--tag",
    `${tag}-mismatch`,
  ]);
} catch (error) {
  rejected = String(error.stderr).includes("does not match metadata tag");
}
invariant(rejected, "release metadata contract must reject a mismatched Git tag");

console.log(`Release metadata contract valid: ${tag}`);
