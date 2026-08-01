export interface BuildInfo {
  version: string;
  commit: string;
  label: string;
}

export function normalizeBuildVersion(rawVersion: string | undefined): string {
  const version = rawVersion?.trim().replace(/^v/, "") ?? "";
  return version || "dev";
}

export function normalizeBuildCommit(rawCommit: string | undefined): string {
  return rawCommit?.trim() || "unknown";
}

export function buildVersionLabel(version: string): string {
  return version === "dev" ? version : `v${version}`;
}

const version = normalizeBuildVersion(
  process.env.NEXT_PUBLIC_AGENTRA_VERSION,
);
const commit = normalizeBuildCommit(process.env.NEXT_PUBLIC_AGENTRA_COMMIT);

export const buildInfo: BuildInfo = {
  version,
  commit,
  label: buildVersionLabel(version),
};
