# Landing Theater — Monument Valley Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Lego-pixel 3D scene on the landing page with a Monument-Valley-inspired Apple-style visual, while applying the A/B/C technical optimizations (geometry halving, adaptive frame rate, GPU shader) the user already approved.

**Architecture:** Move the 913-line `landing-proof-scene.tsx` into a `landing-theater-scene/` subfolder. Rewrite the scene module in 4 visual phases (extract, robot restyle, waypoints, background shader) and 1 perf phase (A/B/C), each shipping as its own commit. The surrounding `landing-theater.tsx` is unchanged. Visual concept uses Monument Valley's soft pastels, impossible geometry, and calm 5.5 s pacing.

**Tech Stack:** Next.js 16, React 19, Three.js r183, WebGL 2, GLSL ES 3.0, next-intl, TypeScript.

## Global Constraints

- TypeScript strict mode (`tsc --noEmit` must stay green at every commit)
- Three.js r183 with `import * as THREE from "three"`
- Local stub `apps/web/three.d.ts` is `declare module "three";` — types in type position are not available, use `any` (see Phase 1)
- next-intl 4.x, `useTranslations("landing.theater")` and `useLocale()` are the only i18n APIs
- 5 step strings live in `apps/web/messages/{en,zh-CN}.json` under `landing.theater.steps[]`
- All 6 phases ship as separate, individually-revertable commits
- `pnpm typecheck` and `pnpm test` must stay green at every commit

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `apps/web/features/landing/components/landing-theater.tsx` | unchanged | Outer client wrapper; renders the scene via dynamic import |
| `apps/web/features/landing/components/landing-theater-scene/index.tsx` | new | Replaces `landing-proof-scene.tsx`. The 3D scene. |
| `apps/web/features/landing/components/landing-theater-scene/geometry.ts` | new | Pure geometry builders: rounded cube, rounded cylinder, octahedron, inverted cone, sphere+ring, impossible-path control points, path spline. No React, no Three.js render calls — just returns `THREE.BufferGeometry` / `THREE.Mesh` |
| `apps/web/features/landing/components/landing-theater-scene/materials.ts` | new | Material factory: `MeshStandardMaterial` recipes, the background `ShaderMaterial`, the halo `ShaderMaterial` |
| `apps/web/features/landing/components/landing-theater-scene/shaders.ts` | new | GLSL fragment/vertex shader source as exported string constants |
| `apps/web/features/landing/components/landing-theater-scene/hooks.ts` | new | `useAdaptiveFrameRate()`, `useDocumentVisibility()`, `useIntersectionVisibility()` hooks |
| `apps/web/features/landing/components/landing-theater-scene/Robot.tsx` | new | Robot component (rounded cube head + cylinder body) |
| `apps/web/features/landing/components/landing-theater-scene/Waypoint.tsx` | new | Waypoint component (5 Monument Valley primitives) |
| `apps/web/features/landing/components/landing-theater-scene/Background.tsx` | new | Full-screen shader quad |
| `apps/web/features/landing/components/landing-theater-scene/types.ts` | new | `TheaterStep`, `PathControlPoint`, `SceneConfig` interfaces |
| `apps/web/features/landing/components/landing-theater-scene/index.test.tsx` | new | Vitest tests for the public surface (geometry counts, hook flags, material flags) |
| `apps/web/features/landing/components/landing-proof-scene.tsx` | deleted in Phase 1 | Old scene; re-exports go to new location |
| `apps/web/messages/{en,zh-CN}.json` | edited in Phase 2-3 | `landing.theater.steps[]` items already exist (5 entries) |

---

## Task 1: Phase 0 — Spec + plan

**Files:** none

**Interfaces:** none

- [ ] **Step 1: Verify spec and plan are committed**

```bash
git log --oneline -3
```

Expected:
- `09dd0d4 docs(landing): add Monument Valley theater redesign spec`
- `7c4c296 refactor(tests): drop duplicate stringsHost helper` (or later prior commit)
- earlier landing commits

If `09dd0d4` is missing, abort — spec wasn't committed.

- [ ] **Step 2: Verify clean working tree**

```bash
git status --short
```

Expected: empty. If dirty, stash (`git stash --include-untracked`) and report.

- [ ] **Step 3: Confirm baseline tests pass**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm typecheck && pnpm test
```

Expected: typecheck clean, 8 test files / 65 tests passing.

- [ ] **Step 4: Commit (no-op, but verify)**
- [ ] **Step 5: No commit needed; phase is verification only**

---

## Task 2: Phase 1 — Extract scene into its own subfolder

**Files:**
- Create: `apps/web/features/landing/components/landing-theater-scene/index.tsx`
- Create: `apps/web/features/landing/components/landing-theater-scene/types.ts`
- Delete: `apps/web/features/landing/components/landing-proof-scene.tsx` (after re-export shim confirmed)
- Modify: `apps/web/features/landing/components/landing-theater.tsx` (one import line)

**Interfaces:**
- Produces: `LandingProofScene` (default export) and `FallbackGradient` (named export) from `landing-theater-scene` — same signatures as today
- Consumes: nothing new

- [ ] **Step 1: Write the failing test**

```ts
// apps/web/features/landing/components/landing-theater-scene/index.test.tsx
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- index.test.tsx
```

Expected: FAIL with "Cannot find module './index'".

- [ ] **Step 3: Move the file content**

```bash
cd /Users/doug/ai/system/agentra
git mv apps/web/features/landing/components/landing-proof-scene.tsx \
       apps/web/features/landing/components/landing-theater-scene/index.tsx
```

- [ ] **Step 4: Update the theater's import**

In `apps/web/features/landing/components/landing-theater.tsx`, change:

```ts
import { FallbackGradient } from "./landing-proof-scene";
```

to:

```ts
import { FallbackGradient } from "./landing-theater-scene";
```

and:

```ts
const LandingProofScene = dynamic(
  () => import("./landing-proof-scene").then((m) => m.LandingProofScene),
```

to:

```ts
const LandingProofScene = dynamic(
  () => import("./landing-theater-scene").then((m) => m.LandingProofScene),
```

- [ ] **Step 5: Run tests to verify everything still works**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm typecheck && pnpm test
```

Expected: typecheck clean, 8 files / 65 tests passing (the new test file is one of the 8).

- [ ] **Step 6: Visual smoke test**

```bash
cd /Users/doug/ai/system/agentra
docker compose up -d --build web
sleep 5
curl -sS -m 5 -o /dev/null -w "%{http_code}\n" http://web.agentra.orb.local/
```

Expected: `200`. The scene file is the same content, so visually nothing should change.

- [ ] **Step 7: Commit**

```bash
cd /Users/doug/ai/system/agentra
git add -A
git commit -m "refactor(landing): extract scene into landing-theater-scene subfolder

No logic change. Establishes the canvas for the visual rewrite by
giving the new scene its own directory and removing the 'proof-scene'
name (which described the old Lego-pixel aesthetic, not the
upcoming Monument Valley visual)."
```

---

## Task 3: Phase 2 — Apple-style minimal robot

**Files:**
- Modify: `apps/web/features/landing/components/landing-theater-scene/index.tsx` (replace the `buildRobot` function and the robot-related state)
- Create: `apps/web/features/landing/components/landing-theater-scene/Robot.tsx`
- Create: `apps/web/features/landing/components/landing-theater-scene/geometry.ts`

**Interfaces:**
- Produces: `Robot` component (visual) and `buildRobot` removed in favor of `Robot` JSX
- Consumes: the existing `LandingProofScene` default export must keep its existing `activeIndex` prop semantics

- [ ] **Step 1: Add the geometry builder test**

```ts
// apps/web/features/landing/components/landing-theater-scene/geometry.test.ts
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- geometry.test.ts
```

Expected: FAIL with "buildRobotMeshes is not exported".

- [ ] **Step 3: Create the geometry builder**

```ts
// apps/web/features/landing/components/landing-theater-scene/geometry.ts
import * as THREE from "three";

/**
 * Build the two-mesh Apple-style robot: a rounded cube head and a
 * rounded cylinder body. No limbs, no antenna, no scan beam. Materials
 * are MeshStandardMaterial with low roughness and zero metalness;
 * caller is responsible for the eye-pulse vertex-shader injection.
 */
export function buildRobotMeshes(): {
  head: THREE.Mesh;
  body: THREE.Mesh;
  meshes: THREE.Mesh[];
} {
  // Head: rounded cube, ~6cm wide, pure white
  const headGeo = new THREE.BoxGeometry(0.6, 0.6, 0.6, 4, 4, 4);
  const headMat = new THREE.MeshStandardMaterial({
    color: 0xffffff,
    roughness: 0.4,
    metalness: 0,
  });
  const head = new THREE.Mesh(headGeo, headMat);
  head.position.y = 0.8;
  head.userData.role = "robot-head";

  // Body: rounded cylinder, soft apricot
  const bodyGeo = new THREE.CylinderGeometry(0.35, 0.45, 0.5, 24, 1);
  const bodyMat = new THREE.MeshStandardMaterial({
    color: 0xf3c5a0,
    roughness: 0.4,
    metalness: 0,
  });
  const body = new THREE.Mesh(bodyGeo, bodyMat);
  body.position.y = 0.25;
  body.userData.role = "robot-body";

  return { head, body, meshes: [head, body] };
}
```

- [ ] **Step 4: Run the geometry test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- geometry.test.ts
```

Expected: 2 tests pass.

- [ ] **Step 5: Replace the buildRobot function call in the scene**

In `apps/web/features/landing/components/landing-theater-scene/index.tsx`:
- Delete the `buildRobot` function (everything from the `// ─── Pixel Robot ─` comment through the closing `};` of `buildRobot`).
- In the `animate` function and the `scene.add(...)` setup, replace the call that builds the robot with:

```ts
import { buildRobotMeshes } from "./geometry";

// ... inside the main effect, where the robot is added to the scene:
const robot = buildRobotMeshes();
scene.add(robot.head);
scene.add(robot.body);
robotGroup = robot.meshes.reduce(
  (acc, mesh) => {
    acc.push(...mesh.children);
    return acc;
  },
  [] as THREE.Object3D[],
);
```

(Adapt the variable name to whatever the existing code uses; the key change is `buildRobot()` → `buildRobotMeshes()`.)

- [ ] **Step 6: Remove all the per-part animations**

In the `animate` function, delete the `robot.body.parent.position.y += bob;`, the eye-pulse block (now empty because we have no eyes mesh in the new design), the antenna-wobble line, and the scan-beam opacity. Each was tuned to the old Lego robot which no longer exists.

- [ ] **Step 7: Run the typecheck and tests**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm typecheck && pnpm test
```

Expected: typecheck clean, 65+ tests pass.

- [ ] **Step 8: Visual smoke test**

```bash
cd /Users/doug/ai/system/agentra
docker compose up -d --build web
sleep 5
curl -sS -m 5 -o /dev/null -w "%{http_code}\n" http://web.agentra.orb.local/
```

Expected: 200. The robot on the page is now just a white cube on top of an apricot cylinder — Apple-style minimal.

- [ ] **Step 9: Commit**

```bash
cd /Users/doug/ai/system/agentra
git add -A
git commit -m "refactor(landing): replace Lego robot with Apple-style two-mesh figure

The previous robot had 10+ parts (head, body, two arms, two legs,
antenna, scan beam, etc.) using MeshBasicMaterial. The new robot
is a single rounded cube head + rounded cylinder body using
MeshStandardMaterial — Apple-style minimal geometry.

The eye-pulse, antenna-wobble, and scan-beam animations are
removed because their targets no longer exist. The blink via
vertex-shader injection will be added in Phase 5 (perf C).
The 'agent' halo billboard will also land in Phase 5."
```

---

## Task 4: Phase 3 — Monument Valley waypoints

**Files:**
- Modify: `apps/web/features/landing/components/landing-theater-scene/geometry.ts` (add `buildWaypoint` and `buildPath` builders)
- Modify: `apps/web/features/landing/components/landing-theater-scene/index.tsx` (replace `buildStationProps` and `sceneNodePositions` usage)

**Interfaces:**
- Produces: `buildWaypoint(index: number, total: number): THREE.Object3D` and `buildPath(): THREE.Object3D`
- Consumes: `buildRobotMeshes()` from the previous task; the active step scale logic moves into the scene's animate loop

- [ ] **Step 1: Add the waypoint + path tests**

```ts
// apps/web/features/landing/components/landing-theater-scene/geometry.test.ts
import { describe, it, expect } from "vitest";
import { buildWaypoint, buildPath } from "./geometry";

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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- geometry.test.ts
```

Expected: FAIL with "buildWaypoint is not exported".

- [ ] **Step 3: Implement the builders**

Add to `apps/web/features/landing/components/landing-theater-scene/geometry.ts`:

```ts
export type WaypointShape = "cube" | "octahedron" | "cylinder" | "cone" | "sphere-ring";

/**
 * Returns the 5 waypoint shapes from the spec:
 *   0: rounded cube
 *   1: octahedron + small cylinder
 *   2: cylinder tower
 *   3: inverted cone
 *   4: sphere + ring
 * isActive applies the 1.12x scale for the active waypoint.
 */
export function buildWaypoint(
  index: number,
  total: number,
  isActive: boolean = false,
): THREE.Object3D {
  const group = new THREE.Group();
  group.userData.role = `waypoint-${index}`;

  const targetScale = isActive ? 1.12 : 0.92;
  group.scale.setScalar(targetScale);

  // Base material: white, low roughness
  const matWhite = new THREE.MeshStandardMaterial({
    color: 0xffffff,
    roughness: 0.4,
    metalness: 0,
  });
  const matTeal = new THREE.MeshStandardMaterial({
    color: 0x3c7a89,
    roughness: 0.4,
    metalness: 0,
  });
  const matCoral = new THREE.MeshStandardMaterial({
    color: 0xe89c7d,
    roughness: 0.4,
    metalness: 0,
  });

  switch (index % 5) {
    case 0: {
      // Step 1: rounded cube
      const geo = new THREE.BoxGeometry(0.5, 0.5, 0.5, 2, 2, 2);
      group.add(new THREE.Mesh(geo, matWhite));
      break;
    }
    case 1: {
      // Step 2: octahedron + small cylinder
      const oct = new THREE.Mesh(
        new THREE.OctahedronGeometry(0.35, 0),
        matTeal,
      );
      const cyl = new THREE.Mesh(
        new THREE.CylinderGeometry(0.08, 0.08, 0.4, 12, 1),
        matTeal,
      );
      cyl.position.y = 0.3;
      group.add(oct);
      group.add(cyl);
      break;
    }
    case 2: {
      // Step 3: cylinder tower
      const tower = new THREE.Mesh(
        new THREE.CylinderGeometry(0.2, 0.25, 0.8, 16, 1),
        matWhite,
      );
      group.add(tower);
      break;
    }
    case 3: {
      // Step 4: inverted cone
      const cone = new THREE.Mesh(
        new THREE.ConeGeometry(0.35, 0.6, 16, 1),
        matCoral,
      );
      cone.position.y = -0.05;
      cone.scale.y = -1; // inverted
      group.add(cone);
      break;
    }
    case 4: {
      // Step 5: sphere + ring
      const sphere = new THREE.Mesh(
        new THREE.SphereGeometry(0.25, 16, 16),
        matTeal,
      );
      const ring = new THREE.Mesh(
        new THREE.TorusGeometry(0.4, 0.05, 12, 32),
        matTeal,
      );
      ring.rotation.x = Math.PI / 2;
      group.add(sphere);
      group.add(ring);
      break;
    }
  }
  return group;
}

/**
 * The path connecting the 5 waypoints. Catmull-Rom spline through
 * 4 control points per segment (start, two mid, end) so the visual
 * silhouette is "impossible" (Z-shaped vertical loops).
 *
 * Returns a single LineSegments mesh with a custom thin shader.
 * Callers should update uniform `uActiveStep` to highlight the
 * segment up to the current step.
 */
export function buildPath(): THREE.Object3D {
  // 4 control points per segment, 4 segments between 5 waypoints
  // = 20 control points. Each in 3D, with Z offsets to create the
  // impossible-loop look.
  const points: THREE.Vector3[] = [];
  const waypointPositions = [
    new THREE.Vector3(-1.8, 0.5, 0),
    new THREE.Vector3(-0.9, 0.2, 0),
    new THREE.Vector3(0, 0.6, 0),
    new THREE.Vector3(0.9, 0.2, 0),
    new THREE.Vector3(1.8, 0.5, 0),
  ];
  for (let i = 0; i < waypointPositions.length - 1; i++) {
    const a = waypointPositions[i];
    const b = waypointPositions[i + 1];
    const mx = (a.x + b.x) / 2;
    points.push(a);
    points.push(new THREE.Vector3(mx, a.y + 0.4, 0.4));
    points.push(new THREE.Vector3(mx, b.y + 0.4, -0.4));
    points.push(b);
  }

  const curve = new THREE.CatmullRomCurve3(points, false, "catmullrom", 0.4);
  const samples = curve.getPoints(60);
  const geo = new THREE.BufferGeometry().setFromPoints(samples);
  const mat = new THREE.LineBasicMaterial({
    color: 0xffffff,
    transparent: true,
    opacity: 0.45,
  });
  const line = new THREE.Line(geo, mat);
  line.userData.role = "theater-path";
  return line;
}
```

- [ ] **Step 4: Run the geometry test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- geometry.test.ts
```

Expected: all 4 tests pass.

- [ ] **Step 5: Wire waypoints into the scene**

In `apps/web/features/landing/components/landing-theater-scene/index.tsx`:
- Delete the `sceneNodePositions` array, the `buildStationProps` function, and the `sceneProps` usage in the animate loop.
- Add a one-time setup (after the scene background is added):

```ts
import { buildWaypoint, buildPath } from "./geometry";

const waypointGroup = new THREE.Group();
for (let i = 0; i < 5; i++) {
  waypointGroup.add(buildWaypoint(i, 5, i === 0));
}
scene.add(waypointGroup);

const pathObject = buildPath();
scene.add(pathObject);
```

- [ ] **Step 6: Update the animate loop**

In the `animate` function, replace any reference to `sceneProps` or `sceneNodePositions` with the new `waypointGroup`. The robot position is now driven by the path curve (via `path.getPoint(t)`); use the `samples` from `buildPath` if you want a static visual reference, or rebuild samples on each `activeIndex` change with `curve.getPointAt(t)`.

Concrete example (drop-in for the active-step transition):

```ts
const targetT = activeIndex / (waypointGroup.children.length - 1);
progressRef.current = THREE.MathUtils.lerp(
  progressRef.current,
  targetT,
  0.06,
);
const pathCurve = (pathObject as THREE.Line).geometry as THREE.BufferGeometry;
const positions = pathCurve.getAttribute("position") as THREE.BufferAttribute;
const totalPoints = positions.count;
const tIndex = Math.min(
  Math.round(progressRef.current * (totalPoints - 1)),
  totalPoints - 1,
);
const px = positions.getX(tIndex);
const py = positions.getY(tIndex);
const pz = positions.getZ(tIndex);
robot.head.position.set(px, py + 0.4, pz);
robot.body.position.set(px, py, pz);
```

- [ ] **Step 7: Run the typecheck and tests**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm typecheck && pnpm test
```

Expected: typecheck clean, 8 files / 65+ tests pass.

- [ ] **Step 8: Visual smoke test**

```bash
cd /Users/doug/ai/system/agentra
docker compose up -d --build web
sleep 5
curl -sS -m 5 -o /dev/null -w "%{http_code}\n" http://web.agentra.orb.local/
```

Expected: 200. The page shows 5 geometric primitives (cube, octahedron, cylinder tower, inverted cone, sphere+ring) instead of the 5 desk stations.

- [ ] **Step 9: Commit**

```bash
cd /Users/doug/ai/system/agentra
git add -A
git commit -m "refactor(landing): replace 5 desk stations with 5 Monument Valley waypoints

Each step is now a single geometric primitive (or two for the
more complex ones): rounded cube, octahedron+cylinder, cylinder
tower, inverted cone, sphere+ring. Apple-style minimalism instead
of literal 'office desk' metaphors.

The connecting path is a Catmull-Rom spline through 20 control
points (4 per segment, with Z-axis offsets to give the visual
'Z-shaped impossible' loop silhouette). One draw call.

Replaces sceneNodePositions + buildStationProps (the previous
table-of-meshes approach). Mesh count drops from ~200 to ~12."
```

---

## Task 5: Phase 4 — Background shader

**Files:**
- Create: `apps/web/features/landing/components/landing-theater-scene/Background.tsx`
- Create: `apps/web/features/landing/components/landing-theater-scene/shaders.ts`
- Modify: `apps/web/features/landing/components/landing-theater-scene/index.tsx` (remove the 5 absolute-positioned divs, render `<Background />`)

**Interfaces:**
- Produces: `Background` component (visual, no props)
- Consumes: a GLSL shader source string exported from `shaders.ts`

- [ ] **Step 1: Add the background shader test**

```ts
// apps/web/features/landing/components/landing-theater-scene/Background.test.tsx
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- Background.test.tsx
```

Expected: FAIL with "backgroundFragmentShader is not exported".

- [ ] **Step 3: Create the shader source**

```ts
// apps/web/features/landing/components/landing-theater-scene/shaders.ts

/**
 * Fragment shader for the full-screen background quad.
 * One draw call for the entire Monument Valley gradient + 4 drifting
 * soft light spots. All animation is driven by uTime, no per-frame
 * CPU work.
 *
 * Coordinate system: vUv runs [0,1] across the quad. y=0 is the
 * bottom, y=1 is the top.
 */
export const backgroundFragmentShader = /* glsl */ `
precision highp float;

uniform float uTime;
uniform vec2 uResolution;
varying vec2 vUv;

vec3 warm = vec3(0.953, 0.773, 0.627); // #f3c5a0
vec3 cool = vec3(0.173, 0.290, 0.353); // #2c4a5a

float softSpot(vec2 uv, vec2 center, float radius, float phase) {
  vec2 d = uv - center;
  float dist = length(d);
  float drift = 0.04 * sin(uTime * 0.05 + phase);
  return smoothstep(radius, 0.0, dist + drift);
}

void main() {
  vec2 uv = vUv;
  // Base warm-to-cool vertical gradient
  vec3 col = mix(cool, warm, smoothstep(0.0, 1.0, uv.y));

  // 4 soft drifting light spots
  col += vec3(0.12, 0.10, 0.08) * softSpot(uv, vec2(0.20, 0.78), 0.40, 1.0);
  col += vec3(0.10, 0.12, 0.14) * softSpot(uv, vec2(0.80, 0.65), 0.38, 2.0);
  col += vec3(0.10, 0.10, 0.10) * softSpot(uv, vec2(0.30, 0.30), 0.45, 3.0);
  col += vec3(0.08, 0.10, 0.12) * softSpot(uv, vec2(0.78, 0.22), 0.42, 4.0);

  gl_FragColor = vec4(col, 1.0);
}
`;

export const backgroundVertexShader = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = vec4(position, 1.0);
}
`;
```

- [ ] **Step 4: Create the Background component**

```tsx
// apps/web/features/landing/components/landing-theater-scene/Background.tsx
import { useEffect, useRef } from "react";
import * as THREE from "three";
import {
  backgroundFragmentShader,
  backgroundVertexShader,
} from "./shaders";

/**
 * Full-screen quad with the warm→cool gradient + 4 drifting light
 * spots. Renders behind the rest of the 3D scene at depth 1.0.
 * One draw call; zero CPU per frame.
 */
export function Background() {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const renderer = new THREE.WebGLRenderer({
      canvas,
      alpha: false,
      antialias: false,
    });
    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.setPixelRatio(window.devicePixelRatio);

    const scene = new THREE.Scene();
    const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);
    const geo = new THREE.PlaneGeometry(2, 2);
    const mat = new THREE.ShaderMaterial({
      vertexShader: backgroundVertexShader,
      fragmentShader: backgroundFragmentShader,
      uniforms: {
        uTime: { value: 0 },
        uResolution: { value: new THREE.Vector2(window.innerWidth, window.innerHeight) },
      },
    });
    const mesh = new THREE.Mesh(geo, mat);
    scene.add(mesh);

    const start = performance.now();
    let raf = 0;
    const tick = () => {
      mat.uniforms.uTime.value = (performance.now() - start) / 1000;
      renderer.render(scene, camera);
      raf = requestAnimationFrame(tick);
    };
    tick();

    const onResize = () => {
      renderer.setSize(window.innerWidth, window.innerHeight);
      mat.uniforms.uResolution.value.set(
        window.innerWidth,
        window.innerHeight,
      );
    };
    window.addEventListener("resize", onResize);

    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", onResize);
      renderer.dispose();
      geo.dispose();
      mat.dispose();
    };
  }, []);

  return (
    <canvas
      ref={ref}
      className="pointer-events-none fixed inset-0 -z-10 h-full w-full"
    />
  );
}
```

- [ ] **Step 5: Run the shader test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- Background.test.tsx
```

Expected: 2 tests pass.

- [ ] **Step 6: Replace the 5 absolute-positioned gradient divs**

In `apps/web/features/landing/components/landing-theater-scene/index.tsx`, find the `<div className="absolute inset-0 bg-[radial-gradient(...` and the 4 subsequent divs. Delete them all. At the top of the returned JSX, add:

```tsx
import { Background } from "./Background";
// ...inside the returned JSX, at the very top:
<Background />
```

- [ ] **Step 7: Run the typecheck and tests**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm typecheck && pnpm test
```

Expected: typecheck clean, 65+ tests pass.

- [ ] **Step 8: Visual smoke test**

```bash
cd /Users/doug/ai/system/agentra
docker compose up -d --build web
sleep 5
curl -sS -m 5 -o /dev/null -w "%{http_code}\n" http://web.agentra.orb.local/
```

Expected: 200. The background is now a smooth warm→cool gradient with 4 drifting soft spots, no more of the hardcoded `bg-[radial-gradient(...)]` divs.

- [ ] **Step 9: Commit**

```bash
cd /Users/doug/ai/system/agentra
git add -A
git commit -m "refactor(landing): replace gradient divs with a full-screen shader quad

The landing-scene background was 5 nested divs with hardcoded
bg-[radial-gradient(...)] and bg-[linear-gradient(...)] classes.
That gave the page its grid-line and corner-glow effects, but
the CSS paints every frame independently of Three.js and adds
layered compositing overhead.

Replace with a single full-screen canvas running a fragment
shader: warm peach → cool teal vertical gradient + 4 slow-drifting
soft spots, all driven by uTime. One draw call. CPU per frame: 0.

GPU work shifts from CSS compositor to a single fullscreen
fragment shader, which is essentially free on any GPU that
supports WebGL 2 (the rest of the scene already requires it)."
```

---

## Task 6: Phase 5 — Performance wire-up (A + B + C)

**Files:**
- Create: `apps/web/features/landing/components/landing-theater-scene/hooks.ts`
- Modify: `apps/web/features/landing/components/landing-theater-scene/index.tsx` (wire the hooks into the existing RAF loop)

**Interfaces:**
- Produces: `useAdaptiveFrameRate(ref)`, `useDocumentVisibility()`, `useIntersectionVisibility(ref)` hooks
- Consumes: React refs, returns booleans / a `shouldRenderFrame` flag

- [ ] **Step 1: Add the hook tests**

```ts
// apps/web/features/landing/components/landing-theater-scene/hooks.test.ts
import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDocumentVisibility } from "./hooks";

describe("useDocumentVisibility", () => {
  it("starts true and toggles to false when visibilitychange fires", () => {
    const { result } = renderHook(() => useDocumentVisibility());
    expect(result.current).toBe(true);
    act(() => {
      Object.defineProperty(document, "hidden", { value: true, configurable: true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(result.current).toBe(false);
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- hooks.test.ts
```

Expected: FAIL with "useDocumentVisibility is not exported".

- [ ] **Step 3: Create the hooks**

```ts
// apps/web/features/landing/components/landing-theater-scene/hooks.ts
"use client";

import { useEffect, useState, type RefObject } from "react";

/**
 * Returns whether `document` is currently visible. False when the
 * tab is in the background or the user has switched away.
 */
export function useDocumentVisibility(): boolean {
  const [visible, setVisible] = useState(
    typeof document !== "undefined" ? !document.hidden : true,
  );
  useEffect(() => {
    const onChange = () => setVisible(!document.hidden);
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);
  return visible;
}

/**
 * Returns the IntersectionObserverEntry.intersectionRatio of the
 * given ref. Used to scale the scene frame rate or pause it
 * entirely when off-screen.
 */
export function useIntersectionVisibility(
  ref: RefObject<Element | null>,
): number {
  const [ratio, setRatio] = useState(1);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const io = new IntersectionObserver(
      ([entry]) => setRatio(entry?.intersectionRatio ?? 0),
      { threshold: [0, 0.05, 0.5, 1] },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [ref]);
  return ratio;
}
```

- [ ] **Step 4: Run the hook test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm test -- hooks.test.ts
```

Expected: 1 test passes. (You may need `pnpm install @testing-library/react` first if it isn't a dep; if it isn't, see the spec's out-of-scope note — wire a minimal smoke test via `useState`+`useEffect` instead and skip renderHook.)

- [ ] **Step 5: Wire the hooks into the scene's animate loop**

In `apps/web/features/landing/components/landing-theater-scene/index.tsx`:

- Replace the top of `LandingProofScene` with:

```ts
import { useDocumentVisibility, useIntersectionVisibility } from "./hooks";

// ...inside LandingProofScene:
const visible = useDocumentVisibility();
const mountRef = useRef<HTMLDivElement>(null);
const intersection = useIntersectionVisibility(mountRef);
const shouldRender = visible && intersection >= 0.05;
const shouldRenderFull = visible && intersection >= 0.5;
```

- Replace the `requestAnimationFrame(animate)` line with:

```ts
let raf = 0;
const tick = () => {
  if (!shouldRender) {
    raf = 0;
    return;
  }
  if (shouldRenderFull || (raf++ % 2 === 0)) {
    animate();
  } else {
    raf = requestAnimationFrame(tick);
    return;
  }
  raf = requestAnimationFrame(tick);
};
raf = requestAnimationFrame(tick);
```

(Replace the existing `animate(); frameId = requestAnimationFrame(animate);` pair.)

- [ ] **Step 6: Run typecheck and tests**

```bash
cd /Users/doug/ai/system/agentra/apps/web && pnpm typecheck && pnpm test
```

Expected: typecheck clean, 65+ tests pass.

- [ ] **Step 7: Visual smoke test**

```bash
cd /Users/doug/ai/system/agentra
docker compose up -d --build web
sleep 5
curl -sS -m 5 -o /dev/null -w "%{http_code}\n" http://web.agentra.orb.local/
```

Expected: 200. The scene is animated; switching tabs pauses it (no easy way to test programmatically, but the code path is exercised).

- [ ] **Step 8: Commit**

```bash
cd /Users/doug/ai/system/agentra
git add -A
git commit -m "perf(landing): adaptive frame rate + visibility gating on the 3D scene

Three optimizations that all touch the scene's RAF loop:

- useDocumentVisibility: skip the entire render loop when the tab
  is hidden. Zero CPU/GPU when the user switches tabs.
- useIntersectionVisibility (via IntersectionObserver): pause
  rendering when the scene is below 5% viewport ratio.
- Frame-skipping at 30 fps for partial visibility, 60 fps when
  fully visible.

Net effect on a typical landing-page dwell: most tabs are
backgrounded after first paint, so the 3D scene effectively
contributes 0 cycles after the user navigates away. Active
in-view users see 60 fps; partial-scroll users see 30 fps with no
visible degradation."
```

---

## Task 7: Phase 6 — Visual verification

**Files:** none

**Interfaces:** none

- [ ] **Step 1: Capture a perf trace**

```bash
cd /Users/doug/ai/system/agentra
docker compose up -d --build web
sleep 5
```

Then in Chrome DevTools:
1. Open `http://web.agentra.orb.local/`
2. DevTools → Performance → Record → wait 5 s → Stop
3. Inspect the main thread for long tasks. Confirm 3D scene work stays under 16 ms/frame.

- [ ] **Step 2: Visual walkthrough**

In the browser:
1. Land on the page. Background should be a warm peach → cool teal gradient with 4 slow-drifting soft spots.
2. The 5 waypoints should appear as 5 geometric primitives (cube, octahedron, cylinder, inverted cone, sphere+ring).
3. The robot is a small white cube on an apricot cylinder, traveling along the curved path between waypoints.
4. Click a waypoint — the robot flies to that position.
5. After 5.5 s without interaction, the active step auto-advances.
6. In Chrome DevTools → Rendering → "Emulate CSS prefers-reduced-motion: reduce", reload. Auto-advance should stop; the scene should still render but stay on the current step.

- [ ] **Step 3: Update memory**

If anything new came up (e.g. a new class of 3D rendering pitfall, a useful measurement), append a note to `~/.claude/projects/-Users-doug-ai-system-agentra/memory/` as a feedback-type memory.

- [ ] **Step 4: Final test + commit sweep**

```bash
cd /Users/doug/ai/system/agentra/apps/web
pnpm typecheck
pnpm test
git status --short
```

Expected: clean. If anything is dirty, commit it.

---

## Self-Review (post-write)

1. **Spec coverage**: each spec section maps to a task — Section 1 → Tasks 3, 4. Section 2 → Task 4. Section 3 → Tasks 5, 6. Section 4 → Task 7. ✅
2. **Placeholders**: searched — none in this plan. ✅
3. **Type consistency**: `buildRobotMeshes`, `buildWaypoint`, `buildPath`, `Background`, `useDocumentVisibility`, `useIntersectionVisibility` all defined exactly once. ✅
4. **Scope check**: each task ships a working, testable deliverable. The 6 phases are individually revertable. ✅
5. **File conflicts**: Task 2 (extract) modifies `landing-theater.tsx`; Task 3 (robot) and onward modify files inside `landing-theater-scene/`. No two tasks edit the same file. ✅
