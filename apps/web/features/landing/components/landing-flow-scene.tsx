"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";

const STEP_POINTS = [
  new THREE.Vector3(-3.4, 0.34, -0.95),
  new THREE.Vector3(-1.65, 0.34, 1.05),
  new THREE.Vector3(0, 0.34, -0.2),
  new THREE.Vector3(1.8, 0.34, 1.2),
  new THREE.Vector3(3.45, 0.34, -0.72),
];

const FLOW_CURVE = new THREE.CatmullRomCurve3(
  STEP_POINTS.map(
    (point) => new THREE.Vector3(point.x, point.y + 0.12, point.z),
  ),
);

export function LandingFlowScene({
  activeIndex,
}: {
  activeIndex: number;
}) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const activeIndexRef = useRef(activeIndex);
  const [renderMode, setRenderMode] = useState<"webgl" | "fallback">("webgl");

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
    scene.fog = new THREE.Fog(0x04070c, 10, 22);

    let renderer;
    try {
      renderer = new THREE.WebGLRenderer({
        antialias: true,
        alpha: true,
        powerPreference: "high-performance",
      });
    } catch (error) {
      console.warn("LandingFlowScene fallback:", error);
      setRenderMode("fallback");
      return;
    }

    setRenderMode("webgl");
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    mountNode.appendChild(renderer.domElement);

    const camera = new THREE.PerspectiveCamera(34, 1, 0.1, 100);
    camera.position.set(0, 4.8, 12.6);
    camera.lookAt(0, 0.1, 0);

    const ambientLight = new THREE.AmbientLight(0xdbeafe, 0.75);
    const keyLight = new THREE.DirectionalLight(0xffffff, 1.35);
    keyLight.position.set(4.5, 7.5, 8);
    const fillLight = new THREE.PointLight(0x67e8f9, 2.5, 18, 2);
    fillLight.position.set(-4, 3.8, 4.6);
    const rimLight = new THREE.PointLight(0xfbbf24, 2.2, 16, 2);
    rimLight.position.set(5.8, 3.5, -4.2);
    scene.add(ambientLight, keyLight, fillLight, rimLight);

    const boardGroup = new THREE.Group();
    boardGroup.rotation.x = -0.56;
    boardGroup.rotation.z = -0.14;
    scene.add(boardGroup);

    const board = new THREE.Mesh(
      new THREE.BoxGeometry(9.6, 0.36, 5.9),
      new THREE.MeshStandardMaterial({
        color: 0x0c1422,
        metalness: 0.72,
        roughness: 0.34,
      }),
    );
    board.receiveShadow = true;
    boardGroup.add(board);

    const boardTop = new THREE.Mesh(
      new THREE.BoxGeometry(9.1, 0.04, 5.35),
      new THREE.MeshStandardMaterial({
        color: 0x132034,
        metalness: 0.1,
        roughness: 0.65,
        transparent: true,
        opacity: 0.96,
      }),
    );
    boardTop.position.y = 0.2;
    boardGroup.add(boardTop);

    const gridGroup = new THREE.Group();
    boardGroup.add(gridGroup);
    for (let index = 0; index < 9; index += 1) {
      const vertical = new THREE.Mesh(
        new THREE.BoxGeometry(0.018, 0.02, 5.18),
        new THREE.MeshBasicMaterial({
          color: 0x1e3a5f,
          transparent: true,
          opacity: 0.28,
        }),
      );
      vertical.position.set(-4 + index, 0.23, 0);
      gridGroup.add(vertical);
    }
    for (let index = 0; index < 5; index += 1) {
      const horizontal = new THREE.Mesh(
        new THREE.BoxGeometry(8.85, 0.02, 0.018),
        new THREE.MeshBasicMaterial({
          color: 0x1e3a5f,
          transparent: true,
          opacity: 0.2,
        }),
      );
      horizontal.position.set(0, 0.23, -2 + index);
      gridGroup.add(horizontal);
    }

    const path = new THREE.Mesh(
      new THREE.TubeGeometry(FLOW_CURVE, 120, 0.045, 12, false),
      new THREE.MeshStandardMaterial({
        color: 0x164e63,
        emissive: 0x0ea5e9,
        emissiveIntensity: 0.18,
        metalness: 0.42,
        roughness: 0.3,
      }),
    );
    boardGroup.add(path);

    const pulse = new THREE.Mesh(
      new THREE.SphereGeometry(0.14, 20, 20),
      new THREE.MeshStandardMaterial({
        color: 0xf0f9ff,
        emissive: 0x67e8f9,
        emissiveIntensity: 2.4,
      }),
    );
    boardGroup.add(pulse);

    const taskCard = new THREE.Mesh(
      new THREE.BoxGeometry(0.94, 0.08, 0.62),
      new THREE.MeshStandardMaterial({
        color: 0xf8fafc,
        emissive: 0xf59e0b,
        emissiveIntensity: 0.2,
        metalness: 0.22,
        roughness: 0.62,
      }),
    );
    taskCard.rotation.y = 0.2;
    boardGroup.add(taskCard);

    const nodeShellGeometry = new THREE.SphereGeometry(0.29, 28, 28);
    const nodeCoreGeometry = new THREE.SphereGeometry(0.12, 20, 20);
    const ringGeometry = new THREE.TorusGeometry(0.42, 0.022, 16, 64);

    const nodes = STEP_POINTS.map((point) => {
      const group = new THREE.Group();
      group.position.copy(point);

      const shell = new THREE.Mesh(
        nodeShellGeometry,
        new THREE.MeshStandardMaterial({
          color: 0x17324d,
          emissive: 0x0f172a,
          emissiveIntensity: 0.35,
          metalness: 0.3,
          roughness: 0.25,
        }),
      );

      const core = new THREE.Mesh(
        nodeCoreGeometry,
        new THREE.MeshStandardMaterial({
          color: 0xe0f2fe,
          emissive: 0x38bdf8,
          emissiveIntensity: 1.2,
        }),
      );

      const ring = new THREE.Mesh(
        ringGeometry,
        new THREE.MeshStandardMaterial({
          color: 0x7dd3fc,
          emissive: 0x0ea5e9,
          emissiveIntensity: 0.3,
          transparent: true,
          opacity: 0.85,
        }),
      );
      ring.rotation.x = Math.PI / 2;

      group.add(shell, core, ring);
      boardGroup.add(group);

      return { group, shell, core, ring };
    });

    const particleGeometry = new THREE.BufferGeometry();
    const particleCount = 48;
    const particlePositions = new Float32Array(particleCount * 3);
    for (let index = 0; index < particleCount; index += 1) {
      particlePositions[index * 3] = (Math.random() - 0.5) * 10;
      particlePositions[index * 3 + 1] = Math.random() * 2.6 + 0.8;
      particlePositions[index * 3 + 2] = (Math.random() - 0.5) * 7;
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
        opacity: 0.5,
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
      const active = activeIndexRef.current;
      const progress = (elapsed * 0.12) % 1;

      pulse.position.copy(FLOW_CURVE.getPointAt(progress));
      pulse.position.y += 0.08;

      const taskProgress = (progress + 0.12) % 1;
      taskCard.position.copy(FLOW_CURVE.getPointAt(taskProgress));
      taskCard.position.y += 0.38;
      taskCard.rotation.z = Math.sin(elapsed * 1.3) * 0.08;

      nodes.forEach((node, index) => {
        const isActive = index === active;
        const emphasis = isActive ? 1 : 0.22;
        const pulseScale = isActive ? 1 + Math.sin(elapsed * 3.2) * 0.08 : 1;

        node.group.scale.setScalar(pulseScale);
        node.shell.material.emissiveIntensity = 0.2 + emphasis * 0.95;
        node.core.material.emissiveIntensity = 0.65 + emphasis * 2.8;
        node.ring.material.emissiveIntensity = 0.15 + emphasis * 1.45;
        node.ring.scale.setScalar(isActive ? 1.08 + Math.sin(elapsed * 2.4) * 0.06 : 0.92);
      });

      path.material.emissiveIntensity = 0.18 + Math.sin(elapsed * 2.2) * 0.04;
      particles.rotation.y = elapsed * 0.04;
      particles.position.y = Math.sin(elapsed * 0.45) * 0.08;

      camera.position.x = Math.sin(elapsed * 0.22) * 0.65;
      camera.position.y = 4.8 + Math.cos(elapsed * 0.24) * 0.14;
      camera.lookAt(0, 0.2, 0);

      renderer.render(scene, camera);
      frameId = window.requestAnimationFrame(animate);
    };

    animate();

    return () => {
      window.cancelAnimationFrame(frameId);
      resizeObserver.disconnect();
      mountNode.removeChild(renderer.domElement);

      scene.traverse((object: {
        geometry?: { dispose: () => void };
        material?:
          | { dispose: () => void }
          | Array<{ dispose: () => void }>;
      }) => {
        const mesh = object;
        if (mesh.geometry) {
          mesh.geometry.dispose();
        }

        const material = mesh.material;
        if (Array.isArray(material)) {
          material.forEach((value) => value.dispose());
        } else if (material) {
          material.dispose();
        }
      });

      particleGeometry.dispose();
      renderer.dispose();
    };
  }, []);

  return (
    <div ref={mountRef} className="relative size-full" aria-hidden="true">
      {renderMode === "fallback" ? (
        <div className="absolute inset-0 overflow-hidden rounded-[24px] bg-[radial-gradient(circle_at_20%_20%,rgba(56,189,248,0.18),transparent_28%),radial-gradient(circle_at_78%_30%,rgba(245,158,11,0.16),transparent_24%),linear-gradient(180deg,rgba(8,16,25,0.92),rgba(3,7,18,0.96))]">
          <div className="absolute inset-x-[10%] top-[14%] h-[68%] rounded-[28px] border border-white/10 bg-[linear-gradient(180deg,rgba(255,255,255,0.07),rgba(255,255,255,0.02))] shadow-[0_28px_90px_rgba(2,6,23,0.55)] [transform:perspective(1200px)_rotateX(60deg)_rotateZ(-12deg)]" />

          <div className="absolute inset-x-[13%] top-[20%] h-[54%] rounded-[24px] border border-cyan-200/8">
            <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(125,211,252,0.08)_1px,transparent_1px),linear-gradient(180deg,rgba(125,211,252,0.06)_1px,transparent_1px)] bg-[size:20%_25%]" />
          </div>

          {[
            { left: "16%", top: "48%" },
            { left: "31%", top: "31%" },
            { left: "49%", top: "44%" },
            { left: "66%", top: "28%" },
            { left: "82%", top: "46%" },
          ].map((point, index) => {
            const isActive = index === activeIndex;
            return (
              <div
                key={`${point.left}-${point.top}`}
                className="absolute"
                style={{ left: point.left, top: point.top }}
              >
                <div
                  className={
                    isActive
                      ? "absolute left-1/2 top-1/2 size-12 -translate-x-1/2 -translate-y-1/2 rounded-full border border-cyan-200/25 bg-cyan-300/12 animate-ping"
                      : "absolute left-1/2 top-1/2 size-10 -translate-x-1/2 -translate-y-1/2 rounded-full border border-white/8 bg-white/[0.04]"
                  }
                />
                <div
                  className={
                    isActive
                      ? "relative size-4 rounded-full bg-cyan-200 shadow-[0_0_28px_rgba(103,232,249,0.9)]"
                      : "relative size-3.5 rounded-full bg-white/65"
                  }
                />
              </div>
            );
          })}

          <div className="absolute left-[18%] top-[47%] h-[2px] w-[64%] origin-left bg-[linear-gradient(90deg,rgba(34,211,238,0.55),rgba(245,158,11,0.45))] [transform:rotate(-9deg)]" />
          <div className="absolute left-[30%] top-[33%] h-[2px] w-[40%] origin-left bg-[linear-gradient(90deg,rgba(34,211,238,0.35),rgba(125,211,252,0.22))] [transform:rotate(12deg)]" />

          <div
            className="absolute size-14 rounded-[16px] border border-amber-200/25 bg-white/90 shadow-[0_18px_50px_rgba(245,158,11,0.24)] transition-all duration-700"
            style={{
              left: ["14%", "28%", "46%", "63%", "78%"][activeIndex] ?? "14%",
              top: ["54%", "38%", "49%", "35%", "52%"][activeIndex] ?? "54%",
              transform: "rotate(-12deg)",
            }}
          >
            <div className="mx-auto mt-3 h-1.5 w-8 rounded-full bg-slate-300" />
            <div className="mx-auto mt-2 h-1.5 w-6 rounded-full bg-slate-200" />
          </div>
        </div>
      ) : null}
    </div>
  );
}
