import { existsSync, readFileSync, unlinkSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const envPath = resolve(root, ".env");
let createdEnv = false;

if (!existsSync(envPath)) {
  const bootstrap = spawnSync("bash", ["scripts/bootstrap-env.sh", envPath], {
    cwd: root,
    encoding: "utf8",
  });
  if (bootstrap.status !== 0) {
    throw new Error(`could not create isolated Compose fixture: ${bootstrap.stderr.trim()}`);
  }
  createdEnv = true;
}

process.on("exit", () => {
  if (createdEnv && existsSync(envPath)) unlinkSync(envPath);
});
const requiredEnv = {
  ...process.env,
  POSTGRES_PASSWORD: "a".repeat(64),
  JWT_SECRET: "b".repeat(64),
  MINIO_ACCESS_KEY: "agentra_0123456789abcdef",
  MINIO_SECRET_KEY: "c".repeat(64),
};

function compose(args, env = requiredEnv) {
  return spawnSync("docker", ["compose", ...args], {
    cwd: root,
    env,
    encoding: "utf8",
  });
}

function config(args = []) {
  const result = compose([...args, "config", "--format", "json"]);
  if (result.status !== 0) {
    throw new Error(`docker compose config failed: ${result.stderr.trim()}`);
  }
  return JSON.parse(result.stdout);
}

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function envValue(source, key) {
  const matches = source
    .split(/\r?\n/)
    .filter((line) => line.startsWith(`${key}=`));
  assert(matches.length === 1, `${key} must occur exactly once in .env.example`);
  return matches[0].slice(key.length + 1);
}

const base = config();
const allProfiles = config(["--profile", "*"]);
const development = config(["-f", "docker-compose.yml", "-f", "docker-compose.dev.yml"]);

assert(!base.services["postgres-console"], "Adminer must be disabled by default");
assert(!base.services.gateway, "No Docker-socket gateway service may exist");
assert(base.services.postgres && !base.services.postgres.ports, "PostgreSQL must not publish a host port by default");
assert(base.services.minio && !base.services.minio.ports, "MinIO API/console must not publish host ports by default");

assert(allProfiles.services["postgres-console"]?.profiles?.includes("debug"), "Adminer must require the debug profile");

for (const serviceName of ["server", "web"]) {
  const ports = base.services[serviceName]?.ports ?? [];
  assert(ports.length === 1, `${serviceName} must publish exactly one application port`);
  assert(ports[0].host_ip === "127.0.0.1", `${serviceName} must bind loopback by default`);
}
const developmentPorts = development.services.postgres?.ports ?? [];
assert(developmentPorts.length === 1, "Development override must publish PostgreSQL once");
assert(developmentPorts[0].host_ip === "127.0.0.1", "Development PostgreSQL must bind loopback");

const composeSource = readFileSync(resolve(root, "docker-compose.yml"), "utf8");
for (const forbidden of [
  "POSTGRES_PASSWORD:-agentra",
  "MINIO_ACCESS_KEY:-agentra",
  "MINIO_SECRET_KEY:-agentra123",
  "JWT_SECRET:-change-me",
]) {
  assert(!composeSource.includes(forbidden), `Compose contains insecure fallback: ${forbidden}`);
}

const example = readFileSync(resolve(root, ".env.example"), "utf8");
for (const key of ["POSTGRES_PASSWORD", "DATABASE_URL", "JWT_SECRET", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY"]) {
  assert(envValue(example, key) === "", `${key} must be blank in .env.example`);
}

const missingSecrets = compose(["config", "--quiet"], {
  ...process.env,
  POSTGRES_PASSWORD: "",
  JWT_SECRET: "",
  MINIO_ACCESS_KEY: "",
  MINIO_SECRET_KEY: "",
});
assert(missingSecrets.status !== 0, "Compose must fail when required secrets are empty");

console.log("Compose security contract valid.");
