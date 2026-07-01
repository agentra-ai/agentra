import { describe, it, expect } from "vitest";
import { backgroundFragmentShader } from "./shaders";

describe("backgroundFragmentShader", () => {
  it("is a non-empty GLSL string containing the warm-cool color stops", () => {
    expect(backgroundFragmentShader).toContain("f3c5a0"); // warm peach
    expect(backgroundFragmentShader).toContain("2c4a5a"); // cool teal
  });
  it("uses uTime for the drifting light spots", () => {
    expect(backgroundFragmentShader).toContain("uTime");
  });
});
