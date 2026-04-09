"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";

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

// t-value for each station along the curve (uniform spacing)
const STATION_T = STATION_POINTS.map(
  (_, i, arr) => i / Math.max(arr.length - 1, 1),
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

// ─── Pixel Robot ─────────────────────────────────────────────────────────────
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const buildRobot = (scene: any): any => {
  const robotGroup = new THREE.Group();

  // Head — boxy pixel style
  const headGeo = new THREE.BoxGeometry(0.32, 0.28, 0.18);
  const headMat = new THREE.MeshBasicMaterial({ color: 0xe8edf5 });
  const head = new THREE.Mesh(headGeo, headMat);
  head.position.y = 0.22;
  robotGroup.add(head);

  // Eyes — two small dark boxes with slight glow
  const eyeGeo = new THREE.BoxGeometry(0.07, 0.06, 0.04);
  const eyeMat = new THREE.MeshBasicMaterial({ color: 0x00d4ff });
  const eyeL = new THREE.Mesh(eyeGeo, eyeMat);
  eyeL.position.set(-0.09, 0.24, 0.1);
  robotGroup.add(eyeL);
  const eyeR = new THREE.Mesh(eyeGeo, eyeMat.clone());
  eyeR.position.set(0.09, 0.24, 0.1);
  robotGroup.add(eyeR);

  // Body
  const bodyGeo = new THREE.BoxGeometry(0.28, 0.32, 0.16);
  const bodyMat = new THREE.MeshBasicMaterial({ color: 0xc8d4e8 });
  const body = new THREE.Mesh(bodyGeo, bodyMat);
  body.position.y = -0.06;
  robotGroup.add(body);

  // Arms — small horizontal bars
  const armGeo = new THREE.BoxGeometry(0.08, 0.22, 0.1);
  const armMat = new THREE.MeshBasicMaterial({ color: 0xb8c8de });
  const armL = new THREE.Mesh(armGeo, armMat);
  armL.position.set(-0.22, -0.04, 0);
  robotGroup.add(armL);
  const armR = new THREE.Mesh(armGeo, armMat.clone());
  armR.position.set(0.22, -0.04, 0);
  robotGroup.add(armR);

  // Legs
  const legGeo = new THREE.BoxGeometry(0.08, 0.18, 0.1);
  const legMat = new THREE.MeshBasicMaterial({ color: 0xa8b8cc });
  const legL = new THREE.Mesh(legGeo, legMat);
  legL.position.set(-0.1, -0.28, 0);
  robotGroup.add(legL);
  const legR = new THREE.Mesh(legGeo, legMat.clone());
  legR.position.set(0.1, -0.28, 0);
  robotGroup.add(legR);

  // Antenna
  const antennaGeo = new THREE.BoxGeometry(0.03, 0.14, 0.03);
  const antennaMat = new THREE.MeshBasicMaterial({ color: 0x00d4ff });
  const antenna = new THREE.Mesh(antennaGeo, antennaMat);
  antenna.position.y = 0.41;
  robotGroup.add(antenna);

  // Antenna tip glow dot
  const tipGeo = new THREE.CircleGeometry(0.04, 8);
  const tipMat = new THREE.MeshBasicMaterial({ color: 0x00d4ff });
  const tip = new THREE.Mesh(tipGeo, tipMat);
  tip.position.y = 0.5;
  robotGroup.add(tip);

  // Scan beam — vertical cyan line (hidden by default)
  const beamGeo = new THREE.PlaneGeometry(0.015, 0.7);
  const beamMat = new THREE.MeshBasicMaterial({
    color: 0x00d4ff,
    transparent: true,
    opacity: 0,
  });
  const scanBeam = new THREE.Mesh(beamGeo, beamMat);
  scanBeam.position.set(0, -0.1, 0.05);
  robotGroup.add(scanBeam);

  robotGroup.position.z = 0.2;
  scene.add(robotGroup);

  return { body, head, eyeL, eyeR, antenna, armL, armR, legL, legR, scanBeam };
};

// ─── Station Props ────────────────────────────────────────────────────────────
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const buildStationProps = (scene: any, stationPoints: any[]): any[] => {
  const stationLabels = ["01", "02", "03", "04", "05"];

  return stationPoints.map((point, index) => {
    const group = new THREE.Group();
    group.position.copy(point);
    scene.add(group);

    const props = {
      group,
      mat0: new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.18 }),
      mat1: undefined as any,
      mat2: undefined as any,
    };

    if (index === 0) {
      // ── 01 接单面板 ─────────────────────────────────────────────────────
      // Vertical panel
      const panelGeo = new THREE.PlaneGeometry(0.22, 0.65);
      const panel = new THREE.Mesh(panelGeo, props.mat0);
      panel.position.y = 0.05;
      group.add(panel);

      // Scan line (moves up/down)
      const scanGeo = new THREE.PlaneGeometry(0.18, 0.025);
      const scanMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.9,
      });
      props.mat1 = scanMat;
      const scanLine = new THREE.Mesh(scanGeo, scanMat);
      scanLine.position.y = -0.25;
      group.add(scanLine);

      // Panel frame lines (horizontal)
      const frameMat = new THREE.MeshBasicMaterial({
        color: 0xffffff,
        transparent: true,
        opacity: 0.22,
      });
      for (let fi = 0; fi < 4; fi++) {
        const lineGeo = new THREE.PlaneGeometry(0.22, 0.012);
        const line = new THREE.Mesh(lineGeo, frameMat.clone());
        line.position.y = -0.15 + fi * 0.15;
        group.add(line);
      }
    } else if (index === 1) {
      // ── 02 路由台 ───────────────────────────────────────────────────────
      // Outer ring
      const ringGeo = new THREE.RingGeometry(0.22, 0.32, 32);
      const ring = new THREE.Mesh(ringGeo, props.mat0);
      group.add(ring);

      // Inner rotating cross
      const crossMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.85,
      });
      props.mat1 = crossMat;
      const hGeo = new THREE.PlaneGeometry(0.38, 0.04);
      const vGeo = new THREE.PlaneGeometry(0.04, 0.38);
      const crossH = new THREE.Mesh(hGeo, crossMat);
      const crossV = new THREE.Mesh(vGeo, crossMat.clone());
      props.mat2 = crossV.material as any;
      group.add(crossH);
      group.add(crossV);

      // Center dot
      const dotGeo = new THREE.CircleGeometry(0.05, 16);
      const dotMat = new THREE.MeshBasicMaterial({ color: 0xffffff });
      const dot = new THREE.Mesh(dotGeo, dotMat);
      group.add(dot);
    } else if (index === 2) {
      // ── 03 执行终端 ─────────────────────────────────────────────────────
      // 3 horizontal stripes
      const stripes: any[] = [];
      for (let si = 0; si < 3; si++) {
        const sGeo = new THREE.PlaneGeometry(0.55, 0.06);
        const sMat = new THREE.MeshBasicMaterial({
          color: si === 1 ? 0x00d4ff : 0xffffff,
          transparent: true,
          opacity: 0.28,
        });
        const stripe = new THREE.Mesh(sGeo, sMat);
        stripe.position.y = 0.18 - si * 0.18;
        group.add(stripe);
        stripes.push(stripe);
        if (si === 1) props.mat1 = sMat;
      }

      // Pulse dots (blink)
      const dotGeo = new THREE.CircleGeometry(0.045, 12);
      const dotMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0,
      });
      props.mat2 = dotMat;
      for (let di = 0; di < 2; di++) {
        const dot = new THREE.Mesh(dotGeo, dotMat.clone());
        dot.position.set(-0.3 + di * 0.6, 0, 0.02);
        group.add(dot);
      }
    } else if (index === 3) {
      // ── 04 Review 扫描架 ────────────────────────────────────────────────
      // Magnifier circle
      const ringGeo = new THREE.RingGeometry(0.18, 0.28, 32);
      const ring = new THREE.Mesh(ringGeo, props.mat0);
      group.add(ring);

      // Glass fill
      const glassMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.06,
      });
      props.mat1 = glassMat;
      const glassGeo = new THREE.CircleGeometry(0.18, 32);
      const glass = new THREE.Mesh(glassGeo, glassMat);
      group.add(glass);

      // Handle
      const handleGeo = new THREE.PlaneGeometry(0.05, 0.2);
      const handleMat = new THREE.MeshBasicMaterial({
        color: 0xffffff,
        transparent: true,
        opacity: 0.4,
      });
      const handle = new THREE.Mesh(handleGeo, handleMat);
      handle.rotation.z = Math.PI / 4;
      handle.position.set(0.18, -0.18, 0);
      group.add(handle);

      // Scan crosshair inside
      const crossMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.7,
      });
      props.mat2 = crossMat;
      const chGeo = new THREE.PlaneGeometry(0.22, 0.015);
      const cvGeo = new THREE.PlaneGeometry(0.015, 0.22);
      const ch = new THREE.Mesh(chGeo, crossMat);
      const cv = new THREE.Mesh(cvGeo, (crossMat.clone()));
      group.add(ch);
      group.add(cv);
    } else {
      // ── 05 Skill 归档柜 ────────────────────────────────────────────────
      // 3 stacked drawers
      const drawerColors = [0xffffff, 0xffffff, 0xffffff];
      for (let dr = 0; dr < 3; dr++) {
        const dGeo = new THREE.PlaneGeometry(0.48, 0.14);
        const dMat = new THREE.MeshBasicMaterial({
          color: drawerColors[dr],
          transparent: true,
          opacity: 0.2 - dr * 0.04,
        });
        const drawer = new THREE.Mesh(dGeo, dMat);
        drawer.position.y = 0.14 - dr * 0.18;
        group.add(drawer);
        if (dr === 0) props.mat1 = dMat;

        // Drawer handle
        const hGeo = new THREE.PlaneGeometry(0.1, 0.02);
        const hMat = new THREE.MeshBasicMaterial({
          color: 0x00d4ff,
          transparent: true,
          opacity: 0.6,
        });
        const hBar = new THREE.Mesh(hGeo, hMat);
        hBar.position.y = 0.14 - dr * 0.18;
        group.add(hBar);
        if (dr === 0) props.mat2 = hMat;
      }

      // Label tag on top
      const tagGeo = new THREE.PlaneGeometry(0.2, 0.06);
      const tagMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.7,
      });
      const tag = new THREE.Mesh(tagGeo, tagMat);
      tag.position.y = 0.3;
      group.add(tag);
    }

    // ── Badge sprite (keep from original) ──────────────────────────────────
    const badgeTexture = createBadgeTexture(stationLabels[index] ?? "00");
    const badgeMaterial = new THREE.SpriteMaterial({
      map: badgeTexture ?? undefined,
      transparent: true,
      depthTest: false,
    });
    if (badgeTexture) {
      const badge: any = new THREE.Sprite(badgeMaterial);
      badge.scale.set(0.92, 0.46, 1);
      badge.position.set(0, 0.9, 0);
      group.add(badge);
    }

    return props;
  });
};

// ─── Main Component ───────────────────────────────────────────────────────────

export function LandingProofScene({
  activeIndex,
}: {
  activeIndex: number;
}) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const activeIndexRef = useRef(activeIndex);
  const progressRef = useRef(0);
  const [renderMode, setRenderMode] = useState<"webgl" | "fallback">(
    "fallback",
  );

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
    const robot = buildRobot(scene);

    // ── Station props ──────────────────────────────────────────────────────
    const stationProps = buildStationProps(scene, STATION_POINTS);

    // ── Station pulse rings (simplified from original) ────────────────────
    const stationRings = STATION_POINTS.map((point) => {
      const group = new THREE.Group();
      group.position.copy(point);
      scene.add(group);

      const pulseMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.1,
      });
      const pulse = new THREE.Mesh(new THREE.RingGeometry(0.42, 0.78, 48), pulseMat);
      group.add(pulse);

      const coreRingMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.28,
      });
      const coreRing = new THREE.Mesh(new THREE.RingGeometry(0.18, 0.3, 40), coreRingMat);
      group.add(coreRing);

      const nodeCoreMat = new THREE.MeshBasicMaterial({
        color: 0x00d4ff,
        transparent: true,
        opacity: 0.75,
      });
      const nodeCore = new THREE.Mesh(new THREE.CircleGeometry(0.1, 36), nodeCoreMat);
      nodeCore.position.z = 0.02;
      group.add(nodeCore);

      return { group, pulseMat, coreRingMat, nodeCoreMat };
    });

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

    const animate = () => {
      const elapsed = clock.getElapsedTime();
      const targetT = STATION_T[activeIndexRef.current] ?? 0;

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

      // Robot idle bob
      const bob = Math.sin(elapsed * 4.2) * 0.04;
      robot.body.parent!.position.y += bob;

      // Eye glow pulse
      const eyeIntensity = 0.7 + Math.sin(elapsed * 3.5) * 0.3;
      (robot.eyeL.material as any).color.setHex(
        eyeIntensity > 0.85 ? 0x00d4ff : 0x009dcc,
      );
      (robot.eyeR.material as any).color.setHex(
        eyeIntensity > 0.85 ? 0x00d4ff : 0x009dcc,
      );

      // Antenna wobble
      robot.antenna.rotation.z = Math.sin(elapsed * 5.5) * 0.18;

      // ── Per-station animations ─────────────────────────────────────────
      const currentStation = activeIndexRef.current;

      stationProps.forEach((prop, idx) => {
        const isActive = idx === currentStation;
        const emphasis = isActive ? 1 : 0.22;

        // Ring emphasis
        if (stationRings[idx]) {
          stationRings[idx].group.scale.setScalar(isActive ? 1.1 : 1);
          stationRings[idx].pulseMat.opacity = 0.03 + emphasis * 0.1;
          stationRings[idx].coreRingMat.opacity = 0.12 + emphasis * 0.18;
          stationRings[idx].nodeCoreMat.opacity = 0.5 + emphasis * 0.3;
        }

        if (idx === 0 && prop.mat1) {
          // 01 Scan line — oscillate y
          const scanY = -0.25 + Math.sin(elapsed * 3.2) * 0.28;
          const child = prop.group.children[1] as any;
          if (child) {
            child.position.y = scanY;
          }
          prop.mat1.opacity = isActive ? 0.7 + Math.sin(elapsed * 3.2) * 0.2 : 0.3;
        }

        if (idx === 1 && prop.mat1 && prop.mat2) {
          // 02 Router cross — rotate
          const rot = elapsed * 2.8;
          const crossH = prop.group.children[1] as any;
          const crossV = prop.group.children[2] as any;
          if (crossH) crossH.rotation.z = rot;
          if (crossV) crossV.rotation.z = -rot * 0.7;
          prop.mat1.opacity = isActive ? 0.85 : 0.4;
          prop.mat2.opacity = isActive ? 0.7 : 0.3;
        }

        if (idx === 2 && prop.mat1) {
          // 03 Terminal — stripe pulse
          prop.group.children.forEach((child: any, ci: number) => {
            if (ci < 3) {
              (child as any).material = prop.group.children[1].material;
            }
          });
          prop.mat1.opacity = isActive
            ? 0.5 + Math.sin(elapsed * 6) * 0.35
            : 0.22;
          // Blink dots
          prop.group.children.forEach((child: any, ci: number) => {
            if (ci === 4 || ci === 5) {
              (child as any).material.opacity =
                isActive && Math.sin(elapsed * 8) > 0 ? 0.9 : 0;
            }
          });
        }

        if (idx === 3 && prop.mat1 && prop.mat2) {
          // 04 Review — scan cross oscillates x
          const crossX = Math.sin(elapsed * 2.5) * 0.1;
          const ch = prop.group.children[3] as any;
          const cv = prop.group.children[4] as any;
          if (ch) ch.position.x = crossX;
          if (cv) cv.position.y = crossX * 0.5;
          prop.mat1.opacity = isActive ? 0.15 : 0.05;
          prop.mat2.opacity = isActive ? 0.75 : 0.35;
        }

        if (idx === 4 && prop.mat2) {
          // 05 Skill — top drawer handle pulses
          prop.mat2.opacity = isActive
            ? 0.5 + Math.sin(elapsed * 2.2) * 0.35
            : 0.45;
          // Slight vertical bounce on active
          if (isActive) {
            const bounce = Math.sin(elapsed * 3.8) * 0.02;
            prop.group.children.forEach((child: { position: { y: number } }) => {
              child.position.y += bounce;
            });
          }
        }
      });

      // Robot station-specific action
      const robotY = robot.body.parent!.position.y;

      // Scan beam only on station 0
      robot.scanBeam.material.opacity =
        currentStation === 0
          ? 0.15 + Math.abs(Math.sin(elapsed * 3.2)) * 0.5
          : 0;

      // ── Trail dots ────────────────────────────────────────────────────────
      trailDots.forEach(({ dot, material }, index) => {
        const offset = Math.max(progressRef.current - index * 0.045, 0);
        const point = FLOW_CURVE.getPointAt(offset);
        dot.position.set(point.x, point.y, 0.08);
        const visibility = THREE.MathUtils.clamp(
          (progressRef.current - index * 0.04) * 7,
          0,
          1,
        );
        material.opacity = (0.2 - index * 0.015) * visibility;
      });

      // ── Arrow markers ─────────────────────────────────────────────────────
      arrowMarkers.forEach((arrow, index) => {
        const progress = 0.16 + index * 0.2;
        const point = FLOW_CURVE.getPointAt(progress);
        const tangent = FLOW_CURVE.getTangentAt(progress);
        arrow.position.set(point.x, point.y, 0.04);
        arrow.rotation.z = Math.atan2(tangent.y, tangent.x) - Math.PI / 2;
      });

      // ── Lane glow pulse ───────────────────────────────────────────────────
      laneGlowMaterial.opacity = 0.08 + Math.sin(elapsed * 1.8) * 0.015;
      laneCoreMaterial.opacity = 0.68 + Math.sin(elapsed * 2.1) * 0.02;
      laneSheenMaterial.opacity = 0.06 + Math.sin(elapsed * 2.8) * 0.015;
      sweepBeam.position.x = -3.8 + ((elapsed * 0.9) % 8);

      renderer.render(scene, camera);
      frameId = window.requestAnimationFrame(animate);
    };

    animate();

    return () => {
      window.cancelAnimationFrame(frameId);
      resizeObserver.disconnect();

      if (mountNode.contains(renderer.domElement)) {
        mountNode.removeChild(renderer.domElement);
      }

      scene.traverse(
        (
          object: {
            geometry?: { dispose: () => void };
            material?:
              | { dispose: () => void }
              | Array<{ dispose: () => void }>;
          },
        ) => {
          if (object.geometry) {
            object.geometry.dispose();
          }
          const material = object.material;
          if (Array.isArray(material)) {
            material.forEach((value) => value.dispose());
          } else if (material) {
            material.dispose();
          }
        },
      );

      renderer.dispose();
    };
  }, []);

  return (
    <div className="absolute inset-0">
      <div
        ref={mountRef}
        className={renderMode === "webgl" ? "h-full w-full" : "hidden"}
      />
      {renderMode === "fallback" ? (
        <div className="h-full w-full bg-[radial-gradient(circle_at_18%_42%,rgba(0,212,255,0.08),transparent_20%),radial-gradient(circle_at_78%_44%,rgba(125,216,232,0.06),transparent_22%),linear-gradient(180deg,rgba(7,9,14,0.95),rgba(3,5,9,0.99))]" />
      ) : null}
    </div>
  );
}
