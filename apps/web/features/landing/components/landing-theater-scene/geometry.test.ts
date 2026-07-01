import { describe, it, expect } from "vitest";
import { buildRobotMeshes, buildWaypoint, buildPath } from "./geometry";

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

describe("buildWaypoint", () => {
  it("returns an Object3D for each of 5 indices", () => {
    for (let i = 0; i < 5; i++) {
      const wp = buildWaypoint(i, 5);
      expect(wp.userData.role).toBe(`waypoint-${i}`);
    }
  });
  it("active=true marks the waypoint as 1.12 scale", () => {
    const wp = buildWaypoint(0, 5, true);
    expect(wp.scale.x).toBeCloseTo(1.12);
  });
});

describe("buildPath", () => {
  it("returns a single line/ribbon Object3D", () => {
    const path = buildPath();
    expect(path.userData.role).toBe("theater-path");
  });
});
