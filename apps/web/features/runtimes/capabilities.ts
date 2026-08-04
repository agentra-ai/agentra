import type { RuntimeAdapterContract } from "@/shared/types";

export const RUNTIME_CAPABILITIES = [
  "discover",
  "models",
  "execute",
  "stream",
  "resume",
  "cancel",
  "model_selection",
  "system_prompt",
  "max_turns",
  "tool_restrictions",
  "skills",
  "mcp",
  "usage",
  "artifacts",
] as const;

export function formatCapabilityName(capability: string): string {
  return capability
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function summarizeCapabilities(adapter: RuntimeAdapterContract): {
  supported: number;
  total: number;
} {
  const declared = RUNTIME_CAPABILITIES.filter(
    (capability) => adapter.capabilities[capability],
  );
  return {
    supported: declared.filter(
      (capability) =>
        adapter.capabilities[capability]?.level !== undefined &&
        adapter.capabilities[capability]?.level !== "unsupported",
    ).length,
    total: declared.length,
  };
}
