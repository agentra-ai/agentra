import { access, readFile } from "node:fs/promises";
import { constants } from "node:fs";
import path from "node:path";
import process from "node:process";

const repositoryRoot = path.resolve(import.meta.dirname, "..");
const manifestPath = path.join(repositoryRoot, "docs", "capabilities.json");
const allowedStatuses = new Set(["stable", "beta", "experimental", "planned"]);
const allowedSurfaces = new Set(["api", "web", "cli", "daemon", "gateway", "mcp", "deployment"]);

function invariant(condition, message) {
  if (!condition) throw new Error(message);
}

async function assertPathExists(relativePath, capabilityId, kind) {
  invariant(!path.isAbsolute(relativePath), `${capabilityId}: ${kind} path must be repository-relative`);
  const resolved = path.resolve(repositoryRoot, relativePath);
  invariant(
    resolved === repositoryRoot || resolved.startsWith(`${repositoryRoot}${path.sep}`),
    `${capabilityId}: ${kind} path escapes the repository`,
  );
  try {
    await access(resolved, constants.R_OK);
  } catch {
    throw new Error(`${capabilityId}: ${kind} path does not exist: ${relativePath}`);
  }
}

const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
invariant(manifest.schema_version === 1, "capability manifest schema_version must be 1");
invariant(Array.isArray(manifest.capabilities), "capabilities must be an array");

const ids = new Set();
for (const capability of manifest.capabilities) {
  const label = capability.id || "<missing-id>";
  invariant(/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(label), `invalid capability id: ${label}`);
  invariant(!ids.has(label), `duplicate capability id: ${label}`);
  ids.add(label);

  invariant(typeof capability.name === "string" && capability.name.length > 0, `${label}: name is required`);
  invariant(allowedStatuses.has(capability.status), `${label}: invalid status ${capability.status}`);
  invariant(typeof capability.summary === "string" && capability.summary.length > 0, `${label}: summary is required`);
  invariant(Array.isArray(capability.surfaces) && capability.surfaces.length > 0, `${label}: surfaces are required`);
  for (const surface of capability.surfaces) {
    invariant(allowedSurfaces.has(surface), `${label}: invalid surface ${surface}`);
  }

  const files = capability.evidence?.files;
  const tests = capability.evidence?.tests ?? [];
  invariant(Array.isArray(files), `${label}: evidence.files must be an array`);
  invariant(Array.isArray(tests), `${label}: evidence.tests must be an array`);
  if (capability.status !== "planned") {
    invariant(files.length > 0, `${label}: implemented capabilities need file evidence`);
  }
  if (capability.status === "stable") {
    invariant(tests.length > 0, `${label}: stable capabilities need automated test evidence`);
    invariant(!capability.limitations?.length, `${label}: stable capabilities cannot declare known limitations`);
  }

  await Promise.all([
    ...files.map((file) => assertPathExists(file, label, "file evidence")),
    ...tests.map((file) => assertPathExists(file, label, "test evidence")),
  ]);
}

console.log(`Capability manifest valid: ${manifest.capabilities.length} capabilities`);
