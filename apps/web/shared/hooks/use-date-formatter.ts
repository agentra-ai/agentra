"use client";

import { useFormatter } from "next-intl";

/**
 * Locale-aware `useFormatter()` from next-intl.
 *
 * Callers should pass `new Date(isoString)` as the value — the hook
 * itself mirrors the exact `useFormatter()` signature so all Intl types
 * flow through unchanged.
 */
export function useDateFormatter() {
  return useFormatter();
}
