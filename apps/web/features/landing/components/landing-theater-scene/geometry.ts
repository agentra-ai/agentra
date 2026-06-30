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