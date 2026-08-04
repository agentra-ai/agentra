import { describe, expect, it } from "vitest";
import { isCurrentRunEvent, isFreshRunSnapshot } from "./run-event";

describe("Run-aware live UI events", () => {
  it("rejects delayed events from a superseded Run", () => {
    expect(isCurrentRunEvent("run-b", "run-a")).toBe(false);
    expect(isCurrentRunEvent("run-b", "run-b")).toBe(true);
  });

  it("allows identity discovery and non-attempt legacy events", () => {
    expect(isCurrentRunEvent(null, "run-a")).toBe(true);
    expect(isCurrentRunEvent("run-a", undefined)).toBe(true);
  });

  it("rejects an old snapshot after a newer dispatch", () => {
    expect(isFreshRunSnapshot("run-a", "run-b", "run-a")).toBe(false);
    expect(isFreshRunSnapshot("run-a", "run-b", "run-b")).toBe(true);
    expect(isFreshRunSnapshot("run-a", "run-a", "run-b")).toBe(true);
  });
});
