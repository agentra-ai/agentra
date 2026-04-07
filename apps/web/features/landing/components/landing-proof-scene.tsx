"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";

const STATION_POINTS = [
  new THREE.Vector3(-3.1, 1.02, -0.38),
  new THREE.Vector3(2.95, 0.92, -0.08),
  new THREE.Vector3(3.18, 0.42, 1.02),
  new THREE.Vector3(0.52, 0.1, 1.38),
  new THREE.Vector3(-2.72, 0.34, 0.92),
];

const FLOW_CURVE = new THREE.CatmullRomCurve3(
  [...STATION_POINTS, STATION_POINTS[0]].map(
    (point) => new THREE.Vector3(point.x, point.y, point.z),
  ),
  true,
  "catmullrom",
  0.12,
);

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

    const scene = new THREE.Scene();
    scene.fog = new THREE.Fog(0x040816, 9, 15);

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

    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    mountNode.appendChild(renderer.domElement);
    setRenderMode("webgl");

    const camera = new THREE.PerspectiveCamera(30, 1, 0.1, 100);
    camera.position.set(0.05, 3.8, 8.35);
    camera.lookAt(0, 0.82, 0.38);

    const ambientLight = new THREE.AmbientLight(0xe5f3ff, 0.96);
    const keyLight = new THREE.DirectionalLight(0xffffff, 1.16);
    keyLight.position.set(4.6, 7.8, 4.2);
    const cyanLight = new THREE.PointLight(0x22d3ee, 3.1, 20, 2);
    cyanLight.position.set(-3.8, 2.8, 3.5);
    const emeraldLight = new THREE.PointLight(0x34d399, 2.3, 18, 2);
    emeraldLight.position.set(3.3, 2.4, -2.8);
    const rimLight = new THREE.PointLight(0x93c5fd, 1.7, 16, 2);
    rimLight.position.set(0.8, 3.2, 4.8);
    scene.add(ambientLight, keyLight, cyanLight, emeraldLight, rimLight);

    const boardGroup = new THREE.Group();
    boardGroup.rotation.x = -0.16;
    boardGroup.rotation.z = -0.04;
    boardGroup.position.y = 0.38;
    scene.add(boardGroup);

    const board = new THREE.Mesh(
      new THREE.BoxGeometry(9.55, 0.3, 5.5),
      new THREE.MeshStandardMaterial({
        color: 0x08101d,
        metalness: 0.74,
        roughness: 0.24,
      }),
    );
    boardGroup.add(board);

    const boardTop = new THREE.Mesh(
      new THREE.BoxGeometry(9.02, 0.045, 5.08),
      new THREE.MeshStandardMaterial({
        color: 0x10223d,
        metalness: 0.1,
        roughness: 0.6,
        transparent: true,
        opacity: 0.98,
      }),
    );
    boardTop.position.y = 0.18;
    boardGroup.add(boardTop);

    const gridGroup = new THREE.Group();
    for (let index = 0; index < 7; index += 1) {
      const vertical = new THREE.Mesh(
        new THREE.BoxGeometry(0.018, 0.02, 4.92),
        new THREE.MeshBasicMaterial({
          color: 0x1b3658,
          transparent: true,
          opacity: 0.18,
        }),
      );
      vertical.position.set(-3 + index, 0.2, 0);
      gridGroup.add(vertical);
    }
    for (let index = 0; index < 4; index += 1) {
      const horizontal = new THREE.Mesh(
        new THREE.BoxGeometry(8.72, 0.02, 0.018),
        new THREE.MeshBasicMaterial({
          color: 0x1b3658,
          transparent: true,
          opacity: 0.14,
        }),
      );
      horizontal.position.set(0, 0.2, -1.5 + index);
      gridGroup.add(horizontal);
    }
    boardGroup.add(gridGroup);

    const railMaterial = new THREE.MeshStandardMaterial({
      color: 0x163457,
      emissive: 0x0ea5e9,
      emissiveIntensity: 0.34,
      metalness: 0.48,
      roughness: 0.22,
    });
    const rail = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 240, 0.06, 16, true),
      railMaterial,
    );
    rail.position.y = 0.22;
    boardGroup.add(rail);

    const railGlowMaterial = new THREE.MeshBasicMaterial({
      color: 0x22d3ee,
      transparent: true,
      opacity: 0.14,
    });
    const railGlow = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 240, 0.13, 16, true),
      railGlowMaterial,
    );
    railGlow.position.y = 0.22;
    boardGroup.add(railGlow);

    const stations = STATION_POINTS.map((point) => {
      const stationGroup = new THREE.Group();
      stationGroup.position.copy(point);
      stationGroup.position.y += 0.26;

      const padMaterial = new THREE.MeshBasicMaterial({
        color: 0x22d3ee,
        transparent: true,
        opacity: 0.18,
      });
      const pad = new THREE.Mesh(
        new THREE.CircleGeometry(0.52, 36),
        padMaterial,
      );
      pad.rotation.x = -Math.PI / 2;
      stationGroup.add(pad);

      const plinth = new THREE.Mesh(
        new THREE.CylinderGeometry(0.38, 0.46, 0.16, 28),
        new THREE.MeshStandardMaterial({
          color: 0x102744,
          emissive: 0x08111d,
          emissiveIntensity: 0.18,
          metalness: 0.34,
          roughness: 0.3,
        }),
      );
      plinth.position.y = 0.08;
      stationGroup.add(plinth);

      const orbMaterial = new THREE.MeshStandardMaterial({
        color: 0xe2f2ff,
        emissive: 0x22d3ee,
        emissiveIntensity: 0.9,
      });
      const orb = new THREE.Mesh(
        new THREE.SphereGeometry(0.1, 18, 18),
        orbMaterial,
      );
      orb.position.y = 0.24;
      stationGroup.add(orb);

      const cardGroup = new THREE.Group();
      cardGroup.position.y = 0.62;
      stationGroup.add(cardGroup);

      const card = new THREE.Mesh(
        new THREE.BoxGeometry(1.06, 0.08, 0.7),
        new THREE.MeshStandardMaterial({
          color: 0xf2f8ff,
          emissive: 0x38bdf8,
          emissiveIntensity: 0.08,
          metalness: 0.12,
          roughness: 0.56,
          transparent: true,
          opacity: 0.94,
        }),
      );
      cardGroup.add(card);

      const lineMaterial = new THREE.MeshBasicMaterial({
        color: 0x38bdf8,
        transparent: true,
        opacity: 0.42,
      });
      const lineA = new THREE.Mesh(
        new THREE.BoxGeometry(0.62, 0.015, 0.055),
        lineMaterial,
      );
      lineA.position.set(0, 0.055, -0.16);
      cardGroup.add(lineA);

      const lineB = new THREE.Mesh(
        new THREE.BoxGeometry(0.42, 0.015, 0.05),
        lineMaterial.clone(),
      );
      lineB.position.set(-0.08, 0.055, 0.04);
      cardGroup.add(lineB);

      const lineDotMaterial = new THREE.MeshBasicMaterial({
        color: 0x34d399,
        transparent: true,
        opacity: 0.7,
      });
      const lineDot = new THREE.Mesh(
        new THREE.BoxGeometry(0.14, 0.015, 0.05),
        lineDotMaterial,
      );
      lineDot.position.set(0.27, 0.055, 0.19);
      cardGroup.add(lineDot);

      boardGroup.add(stationGroup);

      return {
        group: stationGroup,
        cardGroup,
        padMaterial,
        orbMaterial,
        lineDotMaterial,
      };
    });

    const packetGroup = new THREE.Group();
    const packetBody = new THREE.Mesh(
      new THREE.BoxGeometry(0.56, 0.11, 0.34),
      new THREE.MeshStandardMaterial({
        color: 0xffffff,
        emissive: 0x38bdf8,
        emissiveIntensity: 0.22,
        metalness: 0.14,
        roughness: 0.34,
      }),
    );
    packetGroup.add(packetBody);

    const packetAccent = new THREE.Mesh(
      new THREE.BoxGeometry(0.3, 0.02, 0.06),
      new THREE.MeshBasicMaterial({
        color: 0x22d3ee,
        transparent: true,
        opacity: 0.85,
      }),
    );
    packetAccent.position.set(0, 0.065, -0.08);
    packetGroup.add(packetAccent);

    const packetHaloMaterial = new THREE.MeshBasicMaterial({
      color: 0x22d3ee,
      transparent: true,
      opacity: 0.22,
    });
    const packetHalo = new THREE.Mesh(
      new THREE.TorusGeometry(0.26, 0.03, 14, 40),
      packetHaloMaterial,
    );
    packetHalo.rotation.x = Math.PI / 2;
    packetGroup.add(packetHalo);
    packetGroup.position.y = 0.54;
    boardGroup.add(packetGroup);

    const trailMaterials = Array.from({ length: 4 }, (_, index) =>
      new THREE.MeshBasicMaterial({
        color: index % 2 === 0 ? 0x22d3ee : 0x34d399,
        transparent: true,
        opacity: 0.22 - index * 0.04,
      }),
    );
    const trailPackets = trailMaterials.map((material, index) => {
      const ghost = new THREE.Mesh(
        new THREE.BoxGeometry(0.42 - index * 0.04, 0.08, 0.24 - index * 0.02),
        material,
      );
      ghost.position.y = 0.5;
      boardGroup.add(ghost);
      return ghost;
    });

    const particleGeometry = new THREE.BufferGeometry();
    const particleCount = 48;
    const particlePositions = new Float32Array(particleCount * 3);
    for (let index = 0; index < particleCount; index += 1) {
      particlePositions[index * 3] = (Math.random() - 0.5) * 8.4;
      particlePositions[index * 3 + 1] = Math.random() * 1.8 + 0.7;
      particlePositions[index * 3 + 2] = (Math.random() - 0.5) * 5.8;
    }
    particleGeometry.setAttribute(
      "position",
      new THREE.BufferAttribute(particlePositions, 3),
    );
    const particles = new THREE.Points(
      particleGeometry,
      new THREE.PointsMaterial({
        color: 0x93c5fd,
        size: 0.05,
        transparent: true,
        opacity: 0.55,
      }),
    );
    scene.add(particles);

    const resize = () => {
      const { clientWidth, clientHeight } = mountNode;
      renderer.setSize(clientWidth, clientHeight, false);
      camera.aspect = clientWidth / Math.max(clientHeight, 1);
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
        ((activeIndexRef.current % STATION_POINTS.length) + 0.01) /
        STATION_POINTS.length;

      const currentProgress = progressRef.current;
      const directDelta = targetProgress - currentProgress;
      const wrappedDelta =
        directDelta > 0 ? directDelta - 1 : directDelta < 0 ? directDelta + 1 : 0;
      const delta =
        Math.abs(directDelta) <= Math.abs(wrappedDelta)
          ? directDelta
          : wrappedDelta;
      progressRef.current = (currentProgress + delta * 0.06 + 1) % 1;

      const packetPoint = FLOW_CURVE.getPointAt(progressRef.current);
      packetGroup.position.set(packetPoint.x, packetPoint.y + 0.52, packetPoint.z);
      packetGroup.rotation.z = Math.sin(elapsed * 3.5) * 0.06;
      packetGroup.rotation.x = Math.cos(elapsed * 2.4) * 0.04;
      packetHalo.rotation.z = elapsed * 1.4;
      packetHaloMaterial.opacity = 0.18 + Math.sin(elapsed * 4.2) * 0.05;

      trailPackets.forEach((ghost, index) => {
        const offset = (progressRef.current - (index + 1) * 0.03 + 1) % 1;
        const point = FLOW_CURVE.getPointAt(offset);
        ghost.position.set(point.x, point.y + 0.48, point.z);
        ghost.rotation.z = Math.sin(elapsed * 2.6 + index) * 0.04;
      });

      stations.forEach((station, index) => {
        const isActive = index === activeIndexRef.current;
        const emphasis = isActive ? 1 : 0.28;

        station.group.position.y =
          STATION_POINTS[index].y +
          0.26 +
          Math.sin(elapsed * 1.6 + index * 0.7) * 0.04;
        station.cardGroup.rotation.z =
          Math.sin(elapsed * 1.2 + index * 0.9) * 0.035;
        station.cardGroup.scale.setScalar(isActive ? 1.05 : 0.98);
        station.padMaterial.opacity = 0.14 + emphasis * 0.2;
        station.orbMaterial.emissiveIntensity = 0.55 + emphasis * 1.5;
        station.lineDotMaterial.opacity = 0.4 + emphasis * 0.4;
      });

      railMaterial.emissiveIntensity = 0.3 + Math.sin(elapsed * 2.1) * 0.05;
      railGlowMaterial.opacity = 0.12 + Math.sin(elapsed * 2.4) * 0.025;
      particles.rotation.y = elapsed * 0.035;
      particles.position.y = Math.sin(elapsed * 0.55) * 0.04;

      camera.position.x = 0.05 + Math.sin(elapsed * 0.18) * 0.22;
      camera.position.y = 3.8 + Math.cos(elapsed * 0.2) * 0.06;
      camera.lookAt(0, 0.82, 0.38);

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

      particleGeometry.dispose();
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
        <div className="h-full w-full bg-[radial-gradient(circle_at_28%_24%,rgba(56,189,248,0.22),transparent_24%),radial-gradient(circle_at_72%_48%,rgba(52,211,153,0.18),transparent_22%),linear-gradient(180deg,rgba(8,15,28,0.9),rgba(4,8,18,0.98))]" />
      ) : null}
    </div>
  );
}
