# Landing Theater — Monument Valley Visual Redesign

**Date**: 2026-06-30
**Status**: Design (awaiting review)
**Goal**: Replace the current "Lego robot walking across 5 desks" 3D scene with a Monument-Valley-inspired Apple-style premium visual, while applying the A/B/C technical optimizations (geometry halving, adaptive frame rate, GPU shader) the user already approved.

**Architecture**: The 3D scene in `apps/web/features/landing/components/landing-proof-scene.tsx` (913 lines today) is re-imagined as a calm impossible-geometry tableau. The surrounding "left column" / footer / header components in `landing-theater.tsx` stay. Only the scene module is rewritten.

**Tech Stack**: Next.js 16, React 19, Three.js r183, WebGL 2, GLSL ES 3.0, next-intl, TypeScript.

---

## Background — why this exists

The current scene (5 stations + pixel-style robot walking between them) reads as a 2010-era SaaS demo: literal stations, literal agent on a literal path. Code review and self-audit both flagged the visual as "amateurish" (not technically broken — no z-fighting, no clipping — but design-wise dated). Three technical optimizations (geometry halving, adaptive frame rate, GPU shader) were already approved but didn't address the design problem on their own. This spec combines visual redesign with the three optimizations.

User's chosen direction: keep the robot character (it's already part of the product narrative in the agent platform) but redesign everything else around a Monument Valley aesthetic with Apple-style premium finishes.

---

## Section 1 — Visual concept

### Robot (kept, restyled)
- **Head**: a single rounded cube (~6 cm in scene units), pure white, no facial features except two small embedded spheres for eyes
- **Body**: a rounded cylinder pedestal in soft apricot (`#f3c5a0`)
- **No limbs**: drop the current pixel-Lego legs, arms, antenna, scan beam. Apple-style minimalism — one head, one body.
- Material: `MeshStandardMaterial` (replace current `MeshBasicMaterial`), roughness `0.4`, metalness `0`. No environment map.
- Eye pulse: a slow blink (every 4-6s, randomized per eye) — done via vertex shader injection on `onBeforeCompile`, **not** CPU `Math.sin` per frame.

### 5 step waypoints (rebuilt)
- Not "desks" anymore. Each is a single geometric primitive in Monument Valley's vocabulary:
  - **Step 1 (Capture)**: rounded cube, white
  - **Step 2 (Route)**: octahedron + small cylinder, teal (`#3c7a89`)
  - **Step 3 (Execute)**: cylinder tower, white
  - **Step 4 (Review)**: inverted cone, soft coral (`#e89c7d`)
  - **Step 5 (Reuse)**: sphere + ring, teal
- Each waypoint floats in 3D space; connecting them are **impossible paths** (Z-shaped vertical loops reminiscent of Monument Valley's staircases — visually a 2D M.C. Escher path even though 3D).
- Each waypoint is **1-2 meshes**, not 4+ as today. The ring and inverted cone are simple primitive shapes.
- The robot rides the path between waypoints; the path uses a Catmull-Rom spline through 4 control points per segment so the motion is smooth.

### Background
- **Full-screen quad** with a custom fragment shader (not CSS gradient — we need 3D-composited light, not a 2D background).
- Top color: `#f3c5a0` (warm peach)
- Bottom color: `#2c4a5a` (cool teal)
- 4 large soft radial light spots that drift slowly — driven by `uTime` in the shader, zero CPU per frame.
- The 3D scene composites over this quad at depth `1.0`; the waypoints and robot sit at depth `0.0-0.4`.

### Camera
- `PerspectiveCamera`, fov 35, position `(0, 1.4, 4.2)`, looking at `(0, 0.4, 0)`
- **Auto-orbit**: 0.05 rad/s (a full loop every ~125s)
- No user drag — the scene is a "film", not a model viewer. Touch on a waypoint jumps to it.

### State visualization
- **Active waypoint**: scale `1.12`, top emits a soft white glow (additive plane)
- **Completed waypoints**: scale `0.85`, top has a tiny check mark
- **Pending waypoints**: scale `0.92`, color washed toward gray
- Transitions between states: `300ms` ease-out

---

## Section 2 — Interaction

### Auto-advance
- Interval: **5.5 s** per step (slower than the current 4.8 s, more contemplative)
- Detected `prefers-reduced-motion: reduce`: **auto-advance disabled**, scene just shows current state
- Visible-only: when `IntersectionObserver` ratio < 0.05, auto-advance paused

### Manual control
- Click a waypoint: snap to that step (the robot flies along the path to it, ~450 ms ease-in-out spline)
- Click the currently-active waypoint: no-op (it's already there)
- Hover a waypoint: cursor `pointer`, waypoint floats up `0.05` units, label fades in below it ("Step 1: Capture" — next-intl `landing.theater.steps[0].label`)

### Path
- Catmull-Rom spline through 4 control points per segment (3D positions chosen so the visual silhouette is "impossible" — e.g. goes up then down then up)
- Robot position on path: linear `t` between active and target, with a 1.0 ± 0.05 unit gentle bob from `sin(elapsed * 4.2)`

### Robot halo
- A soft additive sphere billboarded to camera, slightly larger than the robot, color matching the warm peach of the body
- Implemented as a single fragment-shader quad (no actual sphere mesh) — saves mesh + draw call

---

## Section 3 — Technical optimizations (A / B / C)

### A — Geometry halving
- Current: ~200 meshes (5 stations × ~6 meshes each + ~20 step-rail planes + robot's 10+ sub-parts + 4 path overlay planes)
- Target: **~40 meshes** total (5 waypoints × 1-2 primitives + 1 robot × 2 parts + 4 background light spots × 0 mesh each + 1 halo quad + 1 path mesh + 1 background full-screen quad)
- `RingGeometry` segments: `32 → 12`
- `PlaneGeometry` segments: stays at `1` (already minimal)
- No tube or extrusion geometries (current scene uses `TubeGeometry` for the path; replace with a custom flat ribbon or LineSegments)

### B — Adaptive frame rate
- IntersectionObserver on the scene container, threshold `[0, 0.05, 0.5, 1]`
  - `ratio < 0.05`: pause the render loop entirely
  - `0.05 ≤ ratio < 0.5`: run RAF at ~30 fps (skip every other frame)
  - `ratio ≥ 0.5`: full 60 fps
- `document.visibilitychange`: stop the loop on hidden, re-render one frame on visible
- Reduced motion: loop runs at 30 fps only when in-view; auto-advance disabled

### C — GPU shader work
- **Background**: 1 full-screen plane with custom `ShaderMaterial`. Vertex shader is a pass-through. Fragment shader computes the warm→cool gradient + 4 drifting soft spots from `uTime`. One draw call for the entire background.
- **Robot eye pulse**: inject into `MeshStandardMaterial.onBeforeCompile`. Add `uniform float uTime;` to the common chunk, and modify the emissive output in the fragment to add a slow blink. No CPU per-frame work.
- **Robot halo**: 1 fragment-shader quad, billboarded to camera via vertex shader, additive blending, soft radial gradient.
- **Path light-points**: 8 small additive billboards distributed along the spline. Updated via vertex shader from a `uActiveStep` uniform + `uTime`. No CPU work for movement.
- **Impossible-path ambient**: the spline path itself is a thin custom-geometry line (1 mesh, ~30 vertices) drawn with a soft transparent shader; not a tube.

### Combined effect
- CPU per frame (idle): `0` (loop paused) or `< 1 ms` (state update only — no matrix recomputation because shaders do it)
- GPU per frame: stable 60 fps target on integrated graphics
- Bundle: still lazy-loaded via `next/dynamic` (`ssr: false`), 3D scene chunk ~600 KB, only fetched when the scene mounts

### Degradation paths
- No WebGL 2: fall back to a static SVG illustration of the same composition
- GPU < integrated threshold: drop fragment shaders, use `MeshBasicMaterial` + a baked gradient
- Mobile: keep one waypoint visible, hide the rest; disable the orbit

---

## Section 4 — Implementation plan, testing, rollback

### Phases (each ships as its own commit; commits 1-3 are reversible without behavior change)

1. **Phase 0 — plan**: spec + implementation plan (this document + writing-plans output)
2. **Phase 1 — extract**: move the 913-line `landing-proof-scene.tsx` into a `landing-theater-scene/` subfolder. No logic change. Theaters still renders the same scene. Establishes the canvas for the rewrite.
3. **Phase 2 — robot restyle**: replace the 10+ Lego-style robot meshes with the new minimal 2-mesh robot (rounded cube head + rounded cylinder body). Eye blink moved to vertex-shader injection. Commit is self-contained; if reverted the old robot comes back.
4. **Phase 3 — waypoints**: replace the 5 desk+instrument stations with the 5 geometric primitives. Path becomes Catmull-Rom spline with 4 control points per segment. Active-step scale/glow + completed/pending states wired up.
5. **Phase 4 — background shader**: replace the 5 absolute-positioned gradient divs with a full-screen quad + custom `ShaderMaterial`. Light spots inside the shader via `uTime`. CPU per-frame cost for background drops to zero.
6. **Phase 5 — A/B/C wire-up**: IntersectionObserver + `document.visibilitychange` + frame-skipping. `RobotEyeBlink` material hook. Halo billboard. No geometry change — pure runtime wiring.
7. **Phase 6 — visual verification**: run `pnpm dev` against the prod build, walk through 5 steps manually, check Chrome DevTools Performance for sustained 60 fps, run `pnpm typecheck` and `pnpm test`.

### Testing per phase
- `pnpm typecheck` + `pnpm test`: must stay green at every commit
- `pnpm dev`: visual smoke test
- Chrome DevTools Performance tab on the prod build: confirm ≥ 55 fps average (target: 60) over a 5-second capture

### Rollback
- Phases 1-3 (visual restructure): reverts to the Lego robot + desks. Behavior is the same; only the visual is older.
- Phases 4-5 (shader + perf): reverts to gradient divs and the current 4.8 s loop. Functionality preserved; performance regressions are acceptable as a temporary state.
- Phase 6 (verification only): no rollback needed.

### Out of scope
- Mobile-specific scene (use a static SVG fallback path described in Section 3; no separate mobile scene)
- WebXR
- Dynamic spline path generation from 3D Bezier handles (the path is hand-authored keyframes)
- Per-locale scene variation (scene uses next-intl for labels; the 3D primitives stay constant)
- Splitting `landing-theater.tsx` itself into smaller components (separate PR)
- Replacing Three.js with a different library (out of scope; we're optimizing the current stack)

---

## Open questions resolved in this spec

| Question | Answer |
|---|---|
| Keep or drop the robot character? | **Keep** — restyle to Apple-style minimal geometry |
| 5 step waypoints shape? | 5 Monument-Valley-style geometric primitives, one per step |
| Background? | Custom fragment shader, warm peach → cool teal |
| Auto-advance? | 5.5 s interval, disabled on reduce-motion, paused off-screen |
| Visual style reference? | Monument Valley (game) — soft pastels, impossible geometry, calm |
| Tech approach? | A+B+C (geometry halving + adaptive frame + GPU shader) |

---

## Verification (post-implementation)

```bash
pnpm typecheck
pnpm test
pnpm dev   # visual check + Chrome DevTools Performance capture
```

Pass criteria:
- 5-step interaction works (auto-advance + click)
- ≥ 55 fps sustained over 5 s
- `pnpm typecheck` and `pnpm test` clean
- Visual review: confirms Apple-style finish (no more Lego-pixel look)
