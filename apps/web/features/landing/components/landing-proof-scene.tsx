"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";

const STATION_POINTS = [
  new THREE.Vector3(-3.2, 0.2, -0.7),
  new THREE.Vector3(-0.9, 0.7, 1.1),
  new THREE.Vector3(1.8, 0.35, 1.4),
  new THREE.Vector3(3.1, 0.15, -0.5),
  new THREE.Vector3(0.2, -0.25, -1.45),
];

const FLOW_CURVE = new THREE.CatmullRomCurve3(
  [...STATION_POINTS, STATION_POINTS[0]].map(
    (point) => new THREE.Vector3(point.x, point.y, point.z),
  ),
  true,
  "catmullrom",
  0.2,
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

    const camera = new THREE.PerspectiveCamera(32, 1, 0.1, 100);
    camera.position.set(0, 4.1, 10.5);
    camera.lookAt(0, 0.1, 0);

    const ambientLight = new THREE.AmbientLight(0xe2f1ff, 0.9);
    const keyLight = new THREE.DirectionalLight(0xffffff, 1.2);
    keyLight.position.set(4, 6, 5);
    const cyanLight = new THREE.PointLight(0x22d3ee, 2.4, 18, 2);
    cyanLight.position.set(-3.4, 2.6, 3.8);
    const emeraldLight = new THREE.PointLight(0x34d399, 1.9, 16, 2);
    emeraldLight.position.set(3.8, 2.4, -3.2);
    scene.add(ambientLight, keyLight, cyanLight, emeraldLight);

    const deckGroup = new THREE.Group();
    deckGroup.rotation.x = -0.54;
    deckGroup.rotation.z = -0.1;
    scene.add(deckGroup);

    const deck = new THREE.Mesh(
      new THREE.BoxGeometry(9.4, 0.34, 5.4),
      new THREE.MeshStandardMaterial({
        color: 0x091120,
        metalness: 0.7,
        roughness: 0.28,
      }),
    );
    deckGroup.add(deck);

    const deckTop = new THREE.Mesh(
      new THREE.BoxGeometry(8.9, 0.04, 5.05),
      new THREE.MeshStandardMaterial({
        color: 0x102038,
        metalness: 0.08,
        roughness: 0.66,
        transparent: true,
        opacity: 0.96,
      }),
    );
    deckTop.position.y = 0.2;
    deckGroup.add(deckTop);

    const railMaterial = new THREE.MeshStandardMaterial({
      color: 0x0f2745,
      emissive: 0x0ea5e9,
      emissiveIntensity: 0.3,
      metalness: 0.45,
      roughness: 0.24,
    });
    const rail = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.05, 14, true),
      railMaterial,
    );
    deckGroup.add(rail);

    const laneGlowMaterial = new THREE.MeshBasicMaterial({
      color: 0x22d3ee,
      transparent: true,
      opacity: 0.08,
    });
    const laneGlow = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 220, 0.1, 14, true),
      laneGlowMaterial,
    );
    deckGroup.add(laneGlow);

    const packet = new THREE.Mesh(
      new THREE.SphereGeometry(0.16, 20, 20),
      new THREE.MeshStandardMaterial({
        color: 0xf0f9ff,
        emissive: 0x67e8f9,
        emissiveIntensity: 2.6,
      }),
    );
    deckGroup.add(packet);

    const cardGeometry = new THREE.BoxGeometry(1.2, 0.12, 0.78);
    const cardMaterial = new THREE.MeshStandardMaterial({
      color: 0xf8fafc,
      emissive: 0x38bdf8,
      emissiveIntensity: 0.08,
      metalness: 0.16,
      roughness: 0.68,
    });

    const floatingCards = [0.08, 0.41, 0.74].map((offset, index) => {
      const card = new THREE.Mesh(cardGeometry, cardMaterial.clone());
      const point = FLOW_CURVE.getPointAt(offset);
      card.position.copy(point);
      card.position.y += 0.32 + index * 0.02;
      card.rotation.y = -0.2 + index * 0.16;
      deckGroup.add(card);
      return card;
    });

    const nodeShellGeometry = new THREE.SphereGeometry(0.24, 24, 24);
    const nodeCoreGeometry = new THREE.SphereGeometry(0.1, 18, 18);
    const nodeRingGeometry = new THREE.TorusGeometry(0.34, 0.02, 16, 48);

    const nodes = STATION_POINTS.map((point) => {
      const group = new THREE.Group();
      group.position.copy(point);

      const shellMaterial = new THREE.MeshStandardMaterial({
        color: 0x12243f,
        emissive: 0x08111d,
        emissiveIntensity: 0.25,
        metalness: 0.24,
        roughness: 0.28,
      });
      const shell = new THREE.Mesh(
        nodeShellGeometry,
        shellMaterial,
      );
      const coreMaterial = new THREE.MeshStandardMaterial({
        color: 0xe0f2fe,
        emissive: 0x22d3ee,
        emissiveIntensity: 1.1,
      });
      const core = new THREE.Mesh(
        nodeCoreGeometry,
        coreMaterial,
      );
      const ringMaterial = new THREE.MeshStandardMaterial({
        color: 0x7dd3fc,
        emissive: 0x0ea5e9,
        emissiveIntensity: 0.22,
        transparent: true,
        opacity: 0.86,
      });
      const ring = new THREE.Mesh(
        nodeRingGeometry,
        ringMaterial,
      );
      ring.rotation.x = Math.PI / 2;

      group.add(shell, core, ring);
      deckGroup.add(group);

      return {
        group,
        shell: shellMaterial,
        core: coreMaterial,
        ring: ringMaterial,
      };
    });

    const particleGeometry = new THREE.BufferGeometry();
    const particleCount = 42;
    const particlePositions = new Float32Array(particleCount * 3);
    for (let index = 0; index < particleCount; index += 1) {
      particlePositions[index * 3] = (Math.random() - 0.5) * 9;
      particlePositions[index * 3 + 1] = Math.random() * 2.2 + 0.6;
      particlePositions[index * 3 + 2] = (Math.random() - 0.5) * 6.4;
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
        opacity: 0.45,
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
        ((activeIndexRef.current % STATION_POINTS.length) + 0.02) /
        STATION_POINTS.length;

      const currentProgress = progressRef.current;
      const directDelta = targetProgress - currentProgress;
      const wrappedDelta =
        directDelta > 0 ? directDelta - 1 : directDelta < 0 ? directDelta + 1 : 0;
      const delta =
        Math.abs(directDelta) <= Math.abs(wrappedDelta) ? directDelta : wrappedDelta;
      progressRef.current = (currentProgress + delta * 0.05 + 1) % 1;

      const packetPoint = FLOW_CURVE.getPointAt(progressRef.current);
      packet.position.copy(packetPoint);
      packet.position.y += 0.12;
      packet.scale.setScalar(1 + Math.sin(elapsed * 4.2) * 0.06);

      nodes.forEach((node, index) => {
        const isActive = index === activeIndexRef.current;
        const emphasis = isActive ? 1 : 0.28;
        const pulseScale = isActive ? 1.06 + Math.sin(elapsed * 3) * 0.05 : 0.96;

        node.group.scale.setScalar(pulseScale);
        node.shell.emissiveIntensity = 0.2 + emphasis * 0.8;
        node.core.emissiveIntensity = 0.5 + emphasis * 2.2;
        node.ring.emissiveIntensity = 0.12 + emphasis * 1.15;
      });

      floatingCards.forEach((card, index) => {
        const offset = (0.1 + index * 0.27 + elapsed * 0.03) % 1;
        const point = FLOW_CURVE.getPointAt(offset);
        card.position.x = point.x;
        card.position.z = point.z;
        card.position.y = 0.34 + Math.sin(elapsed * 1.4 + index) * 0.08;
        card.rotation.z = Math.sin(elapsed * 1.1 + index) * 0.08;
      });

      particles.rotation.y = elapsed * 0.05;
      particles.position.y = Math.sin(elapsed * 0.6) * 0.08;
      laneGlowMaterial.opacity = 0.08 + Math.sin(elapsed * 2.3) * 0.015;
      railMaterial.emissiveIntensity = 0.26 + Math.sin(elapsed * 2) * 0.04;

      camera.position.x = Math.sin(elapsed * 0.18) * 0.55;
      camera.position.y = 4.1 + Math.cos(elapsed * 0.22) * 0.12;
      camera.lookAt(0, 0.12, 0);

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
          const mesh = object as {
            geometry?: { dispose: () => void };
            material?:
              | { dispose: () => void }
              | Array<{ dispose: () => void }>;
          };
          if (mesh.geometry) {
            mesh.geometry.dispose();
          }

          const material = mesh.material;
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
        <div className="h-full w-full bg-[radial-gradient(circle_at_30%_30%,rgba(56,189,248,0.24),transparent_24%),radial-gradient(circle_at_68%_42%,rgba(52,211,153,0.18),transparent_22%),linear-gradient(180deg,rgba(8,15,28,0.9),rgba(4,8,18,0.98))]" />
      ) : null}
    </div>
  );
}
