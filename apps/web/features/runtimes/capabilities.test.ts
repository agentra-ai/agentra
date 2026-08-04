import { describe, expect, it } from "vitest";
import type { RuntimeAdapterContract } from "@/shared/types";
import {
  formatCapabilityName,
  RUNTIME_CAPABILITIES,
  summarizeCapabilities,
} from "./capabilities";

describe("runtime capabilities", () => {
  it("keeps the Runtime Adapter v1 vocabulary complete", () => {
    expect(RUNTIME_CAPABILITIES).toHaveLength(14);
    expect(RUNTIME_CAPABILITIES).toContain("usage");
    expect(RUNTIME_CAPABILITIES).toContain("artifacts");
  });

  it("counts native and adapter support without counting unsupported", () => {
    const adapter: RuntimeAdapterContract = {
      version: "v1",
      transport: "cli",
      capabilities: {
        execute: { level: "native" },
        cancel: { level: "adapter" },
        artifacts: { level: "unsupported" },
      },
    };
    expect(summarizeCapabilities(adapter)).toEqual({ supported: 2, total: 3 });
  });

  it("formats stable capability identifiers for display", () => {
    expect(formatCapabilityName("tool_restrictions")).toBe("Tool Restrictions");
  });
});
