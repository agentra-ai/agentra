import * as THREE from "three";

/**
 * Build the two-mesh Apple-style robot: a rounded cube head and a
 * rounded cylinder body. No limbs, no antenna, no scan beam. Materials
 * are MeshStandardMaterial with low roughness and zero metalness;
 * caller is responsible for the eye-pulse vertex-shader injection.
 */
export function buildRobotMeshes(): {
  head: any;
  body: any;
  meshes: any[];
} {
  // Head: rounded cube, ~6cm wide, pure white
  const headGeo = new THREE.BoxGeometry(0.6, 0.6, 0.6, 6, 6, 6);
  const headMat = new THREE.MeshStandardMaterial({
    color: 0xffffff,
    roughness: 0.35,
    metalness: 0.05,
  });
  const head = new THREE.Mesh(headGeo, headMat);
  head.position.y = 0.8;
  head.castShadow = true;
  head.receiveShadow = true;
  head.userData.role = "robot-head";

  // Body: rounded cylinder, soft apricot — slightly taller for proportion
  const bodyGeo = new THREE.CylinderGeometry(0.35, 0.45, 0.55, 32, 1);
  const bodyMat = new THREE.MeshStandardMaterial({
    color: 0xf3c5a0,
    roughness: 0.35,
    metalness: 0.05,
  });
  const body = new THREE.Mesh(bodyGeo, bodyMat);
  body.position.y = 0.25;
  body.castShadow = true;
  body.receiveShadow = true;
  body.userData.role = "robot-body";

  return { head, body, meshes: [head, body] };
}

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
): any {
  const group = new THREE.Group();
  group.userData.role = `waypoint-${index}`;

  const targetScale = isActive ? 1.12 : 0.92;
  group.scale.setScalar(targetScale);

  // Base materials: soft, slightly metallic for a premium feel under IBL
  const matWhite = new THREE.MeshStandardMaterial({
    color: 0xffffff,
    roughness: 0.35,
    metalness: 0.05,
  });
  const matTeal = new THREE.MeshStandardMaterial({
    color: 0x3c7a89,
    roughness: 0.35,
    metalness: 0.05,
  });
  const matCoral = new THREE.MeshStandardMaterial({
    color: 0xe89c7d,
    roughness: 0.35,
    metalness: 0.05,
  });

  let meshes: any[] = [];
  switch (index % 5) {
    case 0: {
      // Step 1: rounded cube
      const geo = new THREE.BoxGeometry(0.5, 0.5, 0.5, 2, 2, 2);
      meshes = [new THREE.Mesh(geo, matWhite)];
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
      meshes = [oct, cyl];
      break;
    }
    case 2: {
      // Step 3: cylinder tower
      meshes = [
        new THREE.Mesh(
          new THREE.CylinderGeometry(0.2, 0.25, 0.8, 16, 1),
          matWhite,
        ),
      ];
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
      meshes = [cone];
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
      meshes = [sphere, ring];
      break;
    }
  }
  for (const mesh of meshes) {
    mesh.castShadow = true;
    mesh.receiveShadow = true;
    group.add(mesh);
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
export function buildPath(): any {
  // 4 control points per segment, 4 segments between 5 waypoints
  // = 20 control points. Each in 3D, with Z offsets to create the
  // impossible-loop look.
  const points: any[] = [];
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