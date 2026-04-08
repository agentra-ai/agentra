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
  context.fillStyle = "rgba(7, 20, 39, 0.96)";
  context.strokeStyle = "rgba(94, 231, 255, 0.58)";
  context.lineWidth = 4;
  drawRoundedRect(context, 10, 14, 236, 100, 30);
  context.fill();
  context.stroke();

  context.fillStyle = "#def8ff";
  context.font = "700 48px Inter, system-ui, sans-serif";
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.fillText(label, canvas.width / 2, canvas.height / 2 + 1);

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  return texture;
}

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

    const ambientLight = new THREE.AmbientLight(0xf0fbff, 1);
    const cyanLight = new THREE.PointLight(0x22d3ee, 2.2, 12, 2);
    cyanLight.position.set(-2.8, 0.8, 3);
    const emeraldLight = new THREE.PointLight(0x34d399, 1.8, 12, 2);
    emeraldLight.position.set(3, -0.5, 3);
    scene.add(ambientLight, cyanLight, emeraldLight);

    const backgroundPlane = new THREE.Mesh(
      new THREE.PlaneGeometry(11, 5.8),
      new THREE.MeshBasicMaterial({
        color: 0x050c18,
      }),
    );
    backgroundPlane.position.z = -2;
    scene.add(backgroundPlane);

    const gridGroup = new THREE.Group();
    for (let index = 0; index < 9; index += 1) {
      const vertical = new THREE.Mesh(
        new THREE.PlaneGeometry(0.012, 4.2),
        new THREE.MeshBasicMaterial({
          color: 0x1e3d62,
          transparent: true,
          opacity: 0.18,
        }),
      );
      vertical.position.set(-4 + index, 0, -1);
      gridGroup.add(vertical);
    }
    for (let index = 0; index < 5; index += 1) {
      const horizontal = new THREE.Mesh(
        new THREE.PlaneGeometry(8.6, 0.012),
        new THREE.MeshBasicMaterial({
          color: 0x1e3d62,
          transparent: true,
          opacity: 0.14,
        }),
      );
      horizontal.position.set(0, -1.4 + index * 0.7, -1);
      gridGroup.add(horizontal);
    }
    scene.add(gridGroup);

    const laneShadow = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.26, 24, false),
      new THREE.MeshBasicMaterial({
        color: 0x0d2239,
        transparent: true,
        opacity: 0.98,
      }),
    );
    laneShadow.position.z = -0.3;
    scene.add(laneShadow);

    const laneGlowMaterial = new THREE.MeshBasicMaterial({
      color: 0x8ee9ff,
      transparent: true,
      opacity: 0.18,
    });
    const laneGlow = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.18, 24, false),
      laneGlowMaterial,
    );
    laneGlow.position.z = -0.12;
    scene.add(laneGlow);

    const laneCoreMaterial = new THREE.MeshBasicMaterial({
      color: 0xe6fbff,
      transparent: true,
      opacity: 0.78,
    });
    const laneCore = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.088, 20, false),
      laneCoreMaterial,
    );
    laneCore.position.z = 0;
    scene.add(laneCore);

    const laneSheenMaterial = new THREE.MeshBasicMaterial({
      color: 0xd6f9ff,
      transparent: true,
      opacity: 0.14,
    });
    const laneSheen = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.044, 18, false),
      laneSheenMaterial,
    );
    laneSheen.position.z = 0.04;
    scene.add(laneSheen);

    const arrowMarkers = Array.from({ length: 4 }, () => {
      const arrow = new THREE.Mesh(
        new THREE.ConeGeometry(0.11, 0.26, 3),
        new THREE.MeshBasicMaterial({
          color: 0xe6fbff,
          transparent: true,
          opacity: 0.46,
        }),
      );
      scene.add(arrow);
      return arrow;
    });

    const badgeTextures: Array<{ dispose: () => void }> = [];
    const stationLabels = ["01", "02", "03", "04", "05"];
    const stations = STATION_POINTS.map((point, index) => {
      const group = new THREE.Group();
      group.position.copy(point);
      scene.add(group);

      const pulseMaterial = new THREE.MeshBasicMaterial({
        color: 0x22d3ee,
        transparent: true,
        opacity: 0.14,
      });
      const pulse = new THREE.Mesh(
        new THREE.RingGeometry(0.42, 0.78, 48),
        pulseMaterial,
      );
      group.add(pulse);

      const coreRingMaterial = new THREE.MeshBasicMaterial({
        color: 0xdaf7ff,
        transparent: true,
        opacity: 0.5,
      });
      const coreRing = new THREE.Mesh(
        new THREE.RingGeometry(0.18, 0.3, 40),
        coreRingMaterial,
      );
      group.add(coreRing);

      const nodeCoreMaterial = new THREE.MeshBasicMaterial({
        color: 0xf8fdff,
        transparent: true,
        opacity: 0.94,
      });
      const nodeCore = new THREE.Mesh(
        new THREE.CircleGeometry(0.14, 36),
        nodeCoreMaterial,
      );
      nodeCore.position.z = 0.02;
      group.add(nodeCore);

      const badgeTexture = createBadgeTexture(stationLabels[index] ?? "00");
      const badgeMaterial = new THREE.SpriteMaterial({
        map: badgeTexture ?? undefined,
        transparent: true,
        depthTest: false,
      });
      if (badgeTexture) {
        badgeTextures.push(badgeTexture);
      }
      const badge = new THREE.Sprite(badgeMaterial);
      badge.scale.set(0.92, 0.46, 1);
      badge.position.set(0, 0.9, 0);
      group.add(badge);

      return {
        group,
        pulse,
        pulseMaterial,
        coreRingMaterial,
        nodeCoreMaterial,
        badge,
      };
    });

    const packetGroup = new THREE.Group();
    const packetMaterial = new THREE.MeshBasicMaterial({
      color: 0xffffff,
      transparent: true,
      opacity: 1,
    });
    const packetBody = new THREE.Mesh(
      new THREE.PlaneGeometry(0.42, 0.18),
      packetMaterial,
    );
    packetGroup.add(packetBody);

    const packetAccent = new THREE.Mesh(
      new THREE.PlaneGeometry(0.16, 0.035),
      new THREE.MeshBasicMaterial({
        color: 0x7dd3fc,
        transparent: true,
        opacity: 0.88,
      }),
    );
    packetAccent.position.y = 0.06;
    packetAccent.position.z = 0.02;
    packetGroup.add(packetAccent);

    const packetHaloMaterial = new THREE.MeshBasicMaterial({
      color: 0xc9f6ff,
      transparent: true,
      opacity: 0.14,
    });
    const packetHalo = new THREE.Mesh(
      new THREE.RingGeometry(0.24, 0.4, 48),
      packetHaloMaterial,
    );
    packetGroup.add(packetHalo);
    scene.add(packetGroup);

    const trailDots = Array.from({ length: 9 }, (_, index) => {
      const material = new THREE.MeshBasicMaterial({
        color: index < 4 ? 0xb2f2ff : 0x7dd3fc,
        transparent: true,
        opacity: 0.22 - index * 0.018,
      });
      const dot = new THREE.Mesh(
        new THREE.CircleGeometry(Math.max(0.08 - index * 0.004, 0.038), 24),
        material,
      );
      scene.add(dot);
      return { dot, material };
    });

    const accentBlobs = [
      { x: -2.2, y: -0.18, color: 0x7dd3fc, opacity: 0.09, scale: [2.1, 2.3] },
      { x: 2.9, y: -0.08, color: 0xc4b5fd, opacity: 0.07, scale: [1.9, 2.1] },
    ] as const;
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

    const clock = new THREE.Clock();
    let frameId = 0;

    const animate = () => {
      const elapsed = clock.getElapsedTime();
      const targetProgress =
        STATION_POINTS.length > 1
          ? activeIndexRef.current / (STATION_POINTS.length - 1)
          : 0;

      progressRef.current = THREE.MathUtils.lerp(
        progressRef.current,
        targetProgress,
        targetProgress < progressRef.current ? 0.12 : 0.06,
      );

      const packetPoint = FLOW_CURVE.getPointAt(progressRef.current);
      const packetTangent = FLOW_CURVE.getTangentAt(progressRef.current);
      packetGroup.position.set(packetPoint.x, packetPoint.y, 0.16);
      packetGroup.rotation.z = Math.atan2(packetTangent.y, packetTangent.x);
      packetGroup.scale.setScalar(1 + Math.sin(elapsed * 5) * 0.03);
      packetHalo.rotation.z = elapsed * 1.6;
      packetHaloMaterial.opacity = 0.18 + Math.sin(elapsed * 4.2) * 0.05;

      trailDots.forEach(({ dot, material }, index) => {
        const offset = Math.max(progressRef.current - index * 0.045, 0);
        const point = FLOW_CURVE.getPointAt(offset);
        dot.position.set(point.x, point.y, 0.08);
        const visibility = THREE.MathUtils.clamp(
          (progressRef.current - index * 0.04) * 7,
          0,
          1,
        );
        material.opacity = (0.34 - index * 0.028) * visibility;
      });

      arrowMarkers.forEach((arrow, index) => {
        const progress = 0.16 + index * 0.2;
        const point = FLOW_CURVE.getPointAt(progress);
        const tangent = FLOW_CURVE.getTangentAt(progress);
        arrow.position.set(point.x, point.y, 0.04);
        arrow.rotation.z = Math.atan2(tangent.y, tangent.x) - Math.PI / 2;
      });

      stations.forEach((station, index) => {
        const isActive = index === activeIndexRef.current;
        const emphasis = isActive ? 1 : 0.24;

        station.group.scale.setScalar(isActive ? 1.08 : 1);
        station.pulse.scale.setScalar(1 + Math.sin(elapsed * 2.4 + index) * 0.05);
        station.pulseMaterial.opacity = 0.06 + emphasis * 0.18;
        station.coreRingMaterial.opacity = 0.22 + emphasis * 0.28;
        station.nodeCoreMaterial.opacity = 0.72 + emphasis * 0.16;
        station.badge.scale.setScalar(isActive ? 1.06 : 0.96);
      });

      laneGlowMaterial.opacity = 0.16 + Math.sin(elapsed * 1.8) * 0.02;
      laneCoreMaterial.opacity = 0.72 + Math.sin(elapsed * 2.1) * 0.03;
      laneSheenMaterial.opacity = 0.1 + Math.sin(elapsed * 2.8) * 0.02;
      sweepBeam.position.x = -3.8 + ((elapsed * 0.9) % 8);

      renderer.render(scene, camera);
      frameId = window.requestAnimationFrame(animate);
    };

    animate();

    return () => {
      window.cancelAnimationFrame(frameId);
      resizeObserver.disconnect();
      badgeTextures.forEach((texture) => texture.dispose());

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
        <div className="h-full w-full bg-[radial-gradient(circle_at_18%_42%,rgba(56,189,248,0.22),transparent_20%),radial-gradient(circle_at_78%_44%,rgba(52,211,153,0.18),transparent_22%),linear-gradient(180deg,rgba(7,12,24,0.92),rgba(3,7,18,0.98))]" />
      ) : null}
    </div>
  );
}
