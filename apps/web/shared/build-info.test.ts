import { describe, expect, it } from "vitest";
import {
  buildVersionLabel,
  normalizeBuildCommit,
  normalizeBuildVersion,
} from "./build-info";

describe("build info", () => {
  it("normalizes release versions to canonical semver", () => {
    expect(normalizeBuildVersion(" v0.6.0 ")).toBe("0.6.0");
    expect(buildVersionLabel("0.6.0")).toBe("v0.6.0");
  });

  it("keeps local builds explicitly identifiable", () => {
    expect(normalizeBuildVersion(undefined)).toBe("dev");
    expect(normalizeBuildCommit(" ")).toBe("unknown");
    expect(buildVersionLabel("dev")).toBe("dev");
  });
});
