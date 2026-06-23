import { describe, it, expect } from "vitest";
import { appendCapped } from "./ring-buffer";

describe("appendCapped", () => {
  it("appends to an empty array", () => {
    const result = appendCapped<string>([], "a", 5);
    expect(result).toEqual(["a"]);
  });

  it("appends below the cap without dropping", () => {
    const result = appendCapped<string>(["a", "b"], "c", 5);
    expect(result).toEqual(["a", "b", "c"]);
  });

  it("drops the oldest when at cap", () => {
    const result = appendCapped<string>(["a", "b", "c"], "d", 3);
    expect(result).toEqual(["b", "c", "d"]);
  });

  it("drops multiple old entries when far over capacity", () => {
    const result = appendCapped<string>(
      ["a", "b", "c", "d", "e", "f", "g"],
      "h",
      3
    );
    expect(result).toEqual(["f", "g", "h"]);
  });

  it("does not mutate the input array", () => {
    const prev: string[] = ["a", "b", "c"];
    const snapshot = [...prev];
    appendCapped<string>(prev, "d", 3);
    expect(prev).toEqual(snapshot);
  });

  it("handles a max of 1 by keeping only the latest", () => {
    const result = appendCapped<string>(["a", "b", "c"], "d", 1);
    expect(result).toEqual(["d"]);
  });

  it("returns the full array when max is 0 (no cap)", () => {
    const result = appendCapped<string>(["a", "b"], "c", 0);
    expect(result).toEqual(["a", "b", "c"]);
  });

  it("returns the full array when max is negative (no cap)", () => {
    const result = appendCapped<string>(["a", "b"], "c", -1);
    expect(result).toEqual(["a", "b", "c"]);
  });

  it("preserves item identity for objects", () => {
    const obj1 = { id: 1 };
    const obj2 = { id: 2 };
    const result = appendCapped<{ id: number }>([obj1], obj2, 1);
    expect(result).toEqual([obj2]);
  });
});
