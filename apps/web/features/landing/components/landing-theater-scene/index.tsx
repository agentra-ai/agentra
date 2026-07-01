"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { usePrefersReducedMotion } from "@/hooks/use-prefers-reduced-motion";
import { buildRobotMeshes, buildWaypoint, buildPath } from "./geometry";
import { Background } from "./Background";
import { useDocumentVisibility, useIntersectionVisibility } from "./hooks";

const STATION_POINTS = [
  new THREE.Vector3(-3.8, 0.56, 0),
  new THREE.Vector3(-2.05, -0.62, 0),
  new THREE.Vector3(-0.05, 0.08, 0),
  new THREE.Vector3(1.92, -0.5, 0),
  new THREE.Vector3(3.78, 0.44, 0),
];

const FLOW_CURVE = new THREE.CatmullRomCurve3(
  STATION_POINTS.map(
    (point) => new THREE.Vector3(point.x, point.y, point.z),
  ),
  false,
  "catmullrom",
  0.14,
);

function drawRoundedRect(
  context: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
) {
  context.beginPath();
  context.moveTo(x + radius, y);
  context.lineTo(x + width - radius, y);
  context.quadraticCurveTo(x + width, y, x + width, y + radius);
  context.lineTo(x + width, y + height - radius);
  context.quadraticCurveTo(
    x + width,
    y + height,
    x + width - radius,
    y + height,
  );
  context.lineTo(x + radius, y + height);
  context.quadraticCurveTo(x, y + height, x, y + height - radius);
  context.lineTo(x, y + radius);
  context.quadraticCurveTo(x, y, x + radius, y);
  context.closePath();
}

function createBadgeTexture(label: string) {
  const canvas = document.createElement("canvas");
  canvas.width = 256;
  canvas.height = 128;

  const context = canvas.getContext("2d");
  if (!context) {
    return null;
  }

  context.clearRect(0, 0, canvas.width, canvas.height);
  context.fillStyle = "rgba(20, 24, 31, 0.94)";
  context.strokeStyle = "rgba(255, 255, 255, 0.18)";
  context.lineWidth = 4;
  drawRoundedRect(context, 10, 14, 236, 100, 30);
  context.fill();
  context.stroke();

  context.fillStyle = "rgba(255, 255, 255, 0.9)";
  context.font = "700 48px Inter, system-ui, sans-serif";
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.fillText(label, canvas.width / 2, canvas.height / 2 + 1);

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  return texture;
}

/**
 * Dispose a material and any textures it references. Skips the empty
 * base Material case (no .map, no .dispose weirdness).
 */
function disposeMaterial(material: any): void {
  const mat = material;
  if (mat && mat.map) {
    mat.map.dispose();
  }
  if (mat && typeof mat.dispose === "function") {
    mat.dispose();
  }
}


// ─── Main Component ───────────────────────────────────────────────────────────

export function LandingProofScene({
  activeIndex,
}: {
  activeIndex: number;
}) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const activeIndexRef = useRef(activeIndex);
  const progressRef = useRef(0);
  const visible = useDocumentVisibility();
  const intersection = useIntersectionVisibility(mountRef);
  const [renderMode, setRenderMode] = useState<"webgl" | "fallback">(
    "fallback",
  );
  const reduced = usePrefersReducedMotion();

  useEffect(() => {
    activeIndexRef.current = activeIndex;
  }, [activeIndex]);

  useEffect(() => {
    const mountNode = mountRef.current;
    if (!mountNode) {
      return;
    }

    const probeCanvas = document.createElement("canvas");
    const probeContext =
      probeCanvas.getContext("webgl2") ?? probeCanvas.getContext("webgl");
    if (!probeContext) {
      setRenderMode("fallback");
      return;
    }

    const createRenderer = () =>
      new THREE.WebGLRenderer({
        antialias: true,
        alpha: true,
        powerPreference: "high-performance",
      });

    let renderer: ReturnType<typeof createRenderer>;

    try {
      renderer = createRenderer();
    } catch (error) {
      console.warn("LandingProofScene fallback:", error);
      setRenderMode("fallback");
      return;
    }

    const scene = new THREE.Scene();

    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    mountNode.appendChild(renderer.domElement);
    setRenderMode("webgl");

    const camera = new THREE.OrthographicCamera(-4.8, 4.8, 2.3, -2.3, 0.1, 20);
    camera.position.set(0, 0, 8);
    camera.lookAt(0, 0, 0);

    // ── IBL environment (procedural) ─────────────────────────────────────
    // Room-like soft lighting without any external HDR asset. Build a
    // simple scene with a warm-to-cool gradient sky + emissive soft
    // boxes above; PMREMGenerator bakes it into a prefiltered
    // environment map. The result is realistic indirect lighting on
    // MeshStandardMaterial — the single biggest visual upgrade.
    const pmrem = new THREE.PMREMGenerator(renderer);
    pmrem.compileEquirectangularShader();
    const envScene = new THREE.Scene();
    // gradient background via vertex colors on a large box
    const skyGeo = new THREE.BoxGeometry(20, 20, 20);
    const skyMats = [
      // +X right: warm
      new THREE.MeshBasicMaterial({ color: 0xf3d9c0, side: THREE.BackSide }),
      // -X left: cool
      new THREE.MeshBasicMaterial({ color: 0x3a5566, side: THREE.BackSide }),
      // +Y top: warm highlight
      new THREE.MeshBasicMaterial({ color: 0xfff4e8, side: THREE.BackSide }),
      // -Y bottom: cool shadow
      new THREE.MeshBasicMaterial({ color: 0x1a2b33, side: THREE.BackSide }),
      // +Z front: mid
      new THREE.MeshBasicMaterial({ color: 0xa8b8c0, side: THREE.BackSide }),
      // -Z back: mid
      new THREE.MeshBasicMaterial({ color: 0x8899a4, side: THREE.BackSide }),
    ];
    envScene.add(new THREE.Mesh(skyGeo, skyMats));
    // 3 soft area-like emissive panels to create gentle highlights
    const panelDefs: Array<{ pos: [number, number, number]; color: number; intensity: number; size: [number, number] }> = [
      { pos: [-4, 5, -2], color: 0xfff0e0, intensity: 1.5, size: [6, 6] },
      { pos: [4, 3, -3], color: 0xc8dce8, intensity: 1.0, size: [8, 8] },
      { pos: [0, -4, -4], color: 0x2a3a44, intensity: 0.6, size: [12, 12] },
    ];
    for (const p of panelDefs) {
      const panel = new THREE.Mesh(
        new THREE.PlaneGeometry(p.size[0], p.size[1]),
        new THREE.MeshBasicMaterial({ color: p.color, side: THREE.DoubleSide }),
      );
      panel.position.set(...p.pos);
      panel.lookAt(0, 0, 0);
      envScene.add(panel);
    }
    const envMap = pmrem.fromScene(envScene, 0.04).texture;
    scene.environment = envMap;
    skyGeo.dispose();
    pmrem.dispose();

    // ── Key directional light + soft shadow ──────────────────────────────
    const keyLight = new THREE.DirectionalLight(0xfff4e8, 2.2);
    keyLight.position.set(-3.5, 4.5, 5);
    keyLight.castShadow = true;
    keyLight.shadow.mapSize.set(1024, 1024);
    keyLight.shadow.camera.near = 0.5;
    keyLight.shadow.camera.far = 20;
    keyLight.shadow.camera.left = -6;
    keyLight.shadow.camera.right = 6;
    keyLight.shadow.camera.top = 3;
    keyLight.shadow.camera.bottom = -2;
    keyLight.shadow.bias = -0.0005;
    keyLight.shadow.radius = 4;
    scene.add(keyLight);

    // ── Soft fill light ──────────────────────────────────────────────────
    const fillLight = new THREE.DirectionalLight(0xc8dce8, 0.7);
    fillLight.position.set(4, 1, 3);
    scene.add(fillLight);

    // ── Ground plane (contact shadow) ────────────────────────────────────
    const ground = new THREE.Mesh(
      new THREE.PlaneGeometry(20, 10),
      new THREE.ShadowMaterial({ opacity: 0.28 }),
    );
    ground.position.y = -1.2;
    ground.position.z = -0.5;
    ground.rotation.x = -Math.PI / 2;
    ground.receiveShadow = true;
    scene.add(ground);

    // ── Lane tubes ──────────────────────────────────────────────────────────
    const laneShadow = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.26, 24, false),
      new THREE.MeshBasicMaterial({
        color: 0x06080c,
        transparent: true,
        opacity: 0.98,
      }),
    );
    laneShadow.position.z = -0.3;
    scene.add(laneShadow);

    const laneGlowMaterial = new THREE.MeshBasicMaterial({
      color: 0xffffff,
      transparent: true,
      opacity: 0.1,
    });
    const laneGlow = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.18, 24, false),
      laneGlowMaterial,
    );
    laneGlow.position.z = -0.12;
    scene.add(laneGlow);

    const laneCoreMaterial = new THREE.MeshBasicMaterial({
      color: 0xf3f4f6,
      transparent: true,
      opacity: 0.74,
    });
    const laneCore = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.088, 20, false),
      laneCoreMaterial,
    );
    laneCore.position.z = 0;
    scene.add(laneCore);

    const laneSheenMaterial = new THREE.MeshBasicMaterial({
      color: 0xffffff,
      transparent: true,
      opacity: 0.08,
    });
    const laneSheen = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.044, 18, false),
      laneSheenMaterial,
    );
    laneSheen.position.z = 0.04;
    scene.add(laneSheen);

    // ── Arrow markers ─────────────────────────────────────────────────────
    const arrowMarkers = Array.from({ length: 4 }, () => {
      const arrow = new THREE.Mesh(
        new THREE.ConeGeometry(0.11, 0.26, 3),
        new THREE.MeshBasicMaterial({
          color: 0xf3f4f6,
          transparent: true,
          opacity: 0.32,
        }),
      );
      scene.add(arrow);
      return arrow;
    });

    // ── Trail dots ─────────────────────────────────────────────────────────
    const trailDots = Array.from({ length: 9 }, (_, index) => {
      const material = new THREE.MeshBasicMaterial({
        color: index < 4 ? 0x00d4ff : 0x7dd8e8,
        transparent: true,
        opacity: 0.18 - index * 0.016,
      });
      const dot = new THREE.Mesh(
        new THREE.CircleGeometry(Math.max(0.07 - index * 0.004, 0.035), 24),
        material,
      );
      scene.add(dot);
      return { dot, material };
    });

    // ── Accent blobs + sweep beam ──────────────────────────────────────────
    const accentBlobs = [
      { x: -2.2, y: -0.18, color: 0x00d4ff as number, opacity: 0.03, scale: [1.8, 2.0] as [number, number] },
      { x: 2.9, y: -0.08, color: 0x7dd8e8 as number, opacity: 0.025, scale: [1.6, 1.8] as [number, number] },
    ];
    accentBlobs.forEach((blob) => {
      const mesh = new THREE.Mesh(
        new THREE.CircleGeometry(0.9, 40),
        new THREE.MeshBasicMaterial({
          color: blob.color,
          transparent: true,
          opacity: blob.opacity,
        }),
      );
      mesh.position.set(blob.x, blob.y, -0.8);
      mesh.scale.set(blob.scale[0], blob.scale[1], 1);
      scene.add(mesh);
    });

    const sweepBeam = new THREE.Mesh(
      new THREE.PlaneGeometry(2.2, 3.8),
      new THREE.MeshBasicMaterial({
        color: 0xf8fdff,
        transparent: true,
        opacity: 0.05,
      }),
    );
    sweepBeam.position.z = -0.7;
    sweepBeam.rotation.z = 0.12;
    scene.add(sweepBeam);

    // ── Robot ───────────────────────────────────────────────────────────────
    const robot = buildRobotMeshes();
    const robotGroup = new THREE.Group();
    robotGroup.add(robot.head);
    robotGroup.add(robot.body);
    robotGroup.position.z = 0.2;
    scene.add(robotGroup);

    // ── Waypoints + path (Monument Valley) ──────────────────────
    const waypointGroup = new THREE.Group();
    for (let i = 0; i < 5; i++) {
      waypointGroup.add(buildWaypoint(i, 5, i === 0));
    }
    scene.add(waypointGroup);

    const pathObject = buildPath();
    scene.add(pathObject);

    // ── Resize ─────────────────────────────────────────────────────────────
    const resize = () => {
      const { clientWidth, clientHeight } = mountNode;
      renderer.setSize(clientWidth, clientHeight, false);
      const aspect = clientWidth / Math.max(clientHeight, 1);
      const viewHeight = 4.5;
      camera.left = (-viewHeight * aspect) / 2;
      camera.right = (viewHeight * aspect) / 2;
      camera.top = viewHeight / 2;
      camera.bottom = -viewHeight / 2;
      camera.updateProjectionMatrix();
    };

    const resizeObserver = new ResizeObserver(resize);
    resizeObserver.observe(mountNode);
    resize();

    // ── Animation loop ─────────────────────────────────────────────────────
    const clock = new THREE.Clock();
    let frameId = 0;

    const shouldRender = visible && intersection >= 0.05;
    const shouldRenderFull = visible && intersection >= 0.5;
    let frameCount = 0;
    // easeInOutQuad — smoother than linear lerp
    const easeInOut = (t: number) =>
      t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
    const animate = () => {
      if (!shouldRender) {
        return;
      }
      if (shouldRenderFull || frameCount++ % 2 === 0) {
        const elapsed = clock.getElapsedTime();
        const targetT = activeIndexRef.current / 4;
        const delta = targetT - progressRef.current;
        // Scale the step by delta size so small nudges settle fast and
        // large jumps take the full frame budget.
        const step = Math.abs(delta) * 0.06;
        progressRef.current += Math.sign(delta) * Math.max(step, 0.004);
        if (Math.abs(targetT - progressRef.current) < 0.001) {
          progressRef.current = targetT;
        }

        // Robot position along curve
        const robotPoint = FLOW_CURVE.getPointAt(progressRef.current);
        const robotTangent = FLOW_CURVE.getTangentAt(progressRef.current);
        robot.body.parent!.position.set(robotPoint.x, robotPoint.y, 0.22);
        robot.body.parent!.rotation.z = Math.atan2(robotTangent.y, robotTangent.x);

        // Gentle idle bob (sinusoidal, very subtle)
        const bob = Math.sin(elapsed * 4.2) * 0.02;
        robot.head.position.y = 0.8 + bob;

        renderer.render(scene, camera);
      }
      frameId = window.requestAnimationFrame(animate);
    };

    animate();

    // When the user requests reduced motion, render one static frame and
    // stop. The robot stays at the current station without any of the
    // per-part animations. The step still updates on click — the
    // theater's auto-advance is gated separately.
    if (reduced) {
      window.cancelAnimationFrame(frameId);
      renderer.render(scene, camera);
    }

    return () => {
      window.cancelAnimationFrame(frameId);
      resizeObserver.disconnect();

      if (mountNode.contains(renderer.domElement)) {
        mountNode.removeChild(renderer.domElement);
      }

      scene.traverse((object: any) => {
        const mesh = object;
        if (mesh.geometry) {
          mesh.geometry.dispose();
        }
        const material = mesh.material;
        if (Array.isArray(material)) {
          material.forEach((m) => disposeMaterial(m));
        } else if (material) {
          disposeMaterial(material);
        }
      });

      renderer.dispose();
    };
  }, []);

  return (
    <div className="absolute inset-0">
      <Background />
      <div
        ref={mountRef}
        className={renderMode === "webgl" ? "h-full w-full" : "hidden"}
      />
      {renderMode === "fallback" ? <FallbackGradient /> : null}
    </div>
  );
}

/**
 * Static radial gradient used as a fallback when WebGL is unavailable,
 * the 3D scene hasn't loaded yet, or it has thrown and the error
 * boundary caught it. Exported so dynamic-import loading and the
 * error boundary can both reuse the same visual.
 */
export function FallbackGradient() {
  return (
    <div className="h-full w-full bg-[radial-gradient(circle_at_18%_42%,rgba(0,212,255,0.08),transparent_20%),radial-gradient(circle_at_78%_44%,rgba(125,216,232,0.06),transparent_22%),linear-gradient(180deg,rgba(7,9,14,0.95),rgba(3,5,9,0.99))]" />
  );
}
