/**
 * Fragment shader for the full-screen background quad.
 * One draw call for the entire Monument Valley gradient + 4 drifting
 * soft light spots. All animation is driven by uTime, no per-frame
 * CPU work.
 *
 * Coordinate system: vUv runs [0,1] across the quad. y=0 is the
 * bottom, y=1 is the top.
 */
export const backgroundFragmentShader = /* glsl */ `
precision highp float;

uniform float uTime;
uniform vec2 uResolution;
varying vec2 vUv;

vec3 warm = vec3(0.953, 0.773, 0.627); // #f3c5a0
vec3 cool = vec3(0.173, 0.290, 0.353); // #2c4a5a

float softSpot(vec2 uv, vec2 center, float radius, float phase) {
  vec2 d = uv - center;
  float dist = length(d);
  float drift = 0.04 * sin(uTime * 0.05 + phase);
  return smoothstep(radius, 0.0, dist + drift);
}

void main() {
  vec2 uv = vUv;
  // Base warm-to-cool vertical gradient
  vec3 col = mix(cool, warm, smoothstep(0.0, 1.0, uv.y));

  // 4 soft drifting light spots
  col += vec3(0.12, 0.10, 0.08) * softSpot(uv, vec2(0.20, 0.78), 0.40, 1.0);
  col += vec3(0.10, 0.12, 0.14) * softSpot(uv, vec2(0.80, 0.65), 0.38, 2.0);
  col += vec3(0.10, 0.10, 0.10) * softSpot(uv, vec2(0.30, 0.30), 0.45, 3.0);
  col += vec3(0.08, 0.10, 0.12) * softSpot(uv, vec2(0.78, 0.22), 0.42, 4.0);

  gl_FragColor = vec4(col, 1.0);
}
`;

export const backgroundVertexShader = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = vec4(position, 1.0);
}
`;
