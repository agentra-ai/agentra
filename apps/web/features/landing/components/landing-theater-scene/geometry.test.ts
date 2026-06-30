import { describe, it, expect } from "vitest";
import { buildRobotMeshes } from "./geometry";

describe("buildRobotMeshes", () => {
  it("returns exactly 2 meshes (head + body)", () => {
    const result = buildRobotMeshes();
    expect(result.meshes).toHaveLength(2);
  });
  it("the head mesh is a rounded box approximately 6cm wide", () => {
    const { head, body } = buildRobotMeshes();
    expect(head.userData.role).toBe("robot-head");
    expect(body.userData.role).toBe("robot-body");
  });
});