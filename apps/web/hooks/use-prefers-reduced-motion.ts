"use client";

import { useEffect, useState } from "react";

/**
 * Returns true when the user has set "prefers-reduced-motion: reduce"
 * in their OS or browser. Theatrical animations (3D scenes, auto-
 * advancing slideshows) should use this to skip non-essential motion
 * and stay accessible.
 *
 * SSR-safe: starts as false on the server and during hydration, then
 * updates to the actual value after mount.
 */
export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }
    const mql = window.matchMedia("(prefers-reduced-motion: reduce)");
    // Set the initial value post-mount so SSR markup and the first
    // client render agree (both false).
    setReduced(mql.matches);
    const onChange = (event: MediaQueryListEvent) => setReduced(event.matches);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  return reduced;
}
