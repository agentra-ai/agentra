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
    mountNode.appendChild(renderer.domElement);
    setRenderMode("webgl");

    const camera = new THREE.OrthographicCamera(-4.8, 4.8, 2.3, -2.3, 0.1, 20);
    camera.position.set(0, 0, 8);
    camera.lookAt(0, 0, 0);

    // ── Lights ──────────────────────────────────────────────────────────────
    const ambientLight = new THREE.AmbientLight(0xffffff, 0.92);
    const keyLight = new THREE.PointLight(0xffffff, 1.35, 12, 2);
    keyLight.position.set(-2.6, 0.7, 3);
    const rimLight = new THREE.PointLight(0xd1d5db, 0.75, 12, 2);
    rimLight.position.set(3, -0.4, 3);
    scene.add(ambientLight, keyLight, rimLight);

    // ── Background plane ────────────────────────────────────────────────────
    const backgroundPlane = new THREE.Mesh(
      new THREE.PlaneGeometry(11, 5.8),
      new THREE.MeshBasicMaterial({ color: 0x07090d }),
    );
    backgroundPlane.position.z = -2;
    scene.add(backgroundPlane);

    // ── Grid ───────────────────────────────────────────────────────────────
    const gridGroup = new THREE.Group();
    for (let index = 0; index < 9; index += 1) {
      const vertical = new THREE.Mesh(
        new THREE.PlaneGeometry(0.012, 4.2),
        new THREE.MeshBasicMaterial({
          color: 0x5d6775,
          transparent: true,
          opacity: 0.12,
        }),
      );
      vertical.position.set(-4 + index, 0, -1);
      gridGroup.add(vertical);
    }
    for (let index = 0; index < 5; index += 1) {
      const horizontal = new THREE.Mesh(
        new THREE.PlaneGeometry(8.6, 0.012),
        new THREE.MeshBasicMaterial({
          color: 0x5d6775,
          transparent: true,
          opacity: 0.1,
        }),
      );
      horizontal.position.set(0, -1.4 + index * 0.7, -1);
      gridGroup.add(horizontal);
    }
    scene.add(gridGroup);

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
    const animate = () => {
      if (!shouldRender) {
        return;
      }
      if (shouldRenderFull || frameCount++ % 2 === 0) {
        const elapsed = clock.getElapsedTime();
        const targetT = activeIndexRef.current / 4;

        progressRef.current = THREE.MathUtils.lerp(
          progressRef.current,
          targetT,
          targetT < progressRef.current ? 0.14 : 0.055,
        );

        // Robot position along curve
        const robotPoint = FLOW_CURVE.getPointAt(progressRef.current);
        const robotTangent = FLOW_CURVE.getTangentAt(progressRef.current);
        robot.body.parent!.position.set(robotPoint.x, robotPoint.y, 0.22);
        robot.body.parent!.rotation.z = Math.atan2(robotTangent.y, robotTangent.x);

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
