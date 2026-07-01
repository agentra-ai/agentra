"use client";

import { useEffect, useState, type RefObject } from "react";

/**
 * Returns whether `document` is currently visible. False when the
 * tab is in the background or the user has switched away.
 */
export function useDocumentVisibility(): boolean {
  const [visible, setVisible] = useState(
    typeof document !== "undefined" ? !document.hidden : true,
  );
  useEffect(() => {
    const onChange = () => setVisible(!document.hidden);
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);
  return visible;
}

/**
 * Returns the IntersectionObserverEntry.intersectionRatio of the
 * given ref. Used to scale the scene frame rate or pause it
 * entirely when off-screen.
 */
export function useIntersectionVisibility(
  ref: RefObject<Element | null>,
): number {
  const [ratio, setRatio] = useState(1);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const io = new IntersectionObserver(
      ([entry]) => setRatio(entry?.intersectionRatio ?? 0),
      { threshold: [0, 0.05, 0.5, 1] },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [ref]);
  return ratio;
}
