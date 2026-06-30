import { describe, it, expect } from "vitest";
import { LandingProofScene, FallbackGradient } from "./index";

describe("landing-theater-scene public exports", () => {
  it("exports LandingProofScene as the default scene component", () => {
    expect(typeof LandingProofScene).toBe("function");
  });
  it("exports FallbackGradient as a function", () => {
    expect(typeof FallbackGradient).toBe("function");
  });
});
