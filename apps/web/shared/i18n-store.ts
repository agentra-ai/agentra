/* eslint-disable @typescript-eslint/no-explicit-any */
/**
 * Minimal translator for zustand stores and other non-React code.
 *
 * React components should still prefer `useTranslations()` so their output
 * updates on locale change. Stores only fire toasts on async failures and
 * re-mount when the user navigates, so a snapshot read of the active
 * locale at call-time is sufficient.
 *
 * Reads the `agentra-locale` cookie (fallback `en`), then walks the dot-path
 * against the statically-imported message dictionary. Falls back to `en`,
 * then to the raw key, so a missing translation never throws and still
 * produces readable output.
 */
import { messages } from "@/i18n";

const LOCALE_COOKIE = "agentra-locale";

function readLocale(): string {
  if (typeof document === "undefined") return "en";
  const match = document.cookie.match(
    new RegExp(`(?:^|;\\s*)${LOCALE_COOKIE}=([^;]+)`),
  );
  const v = match?.[1];
  return v === "zh-CN" || v === "en" ? v : "en";
}

function walk(dict: any, path: string): string | undefined {
  let cur = dict;
  for (const part of path.split(".")) {
    if (cur == null || typeof cur !== "object") return undefined;
    cur = cur[part];
  }
  return typeof cur === "string" ? cur : undefined;
}

/**
 * Look up a single translation key for the active locale.
 * Usage in a store action: `toast.error(storeT("issues.store.loadFailed"))`.
 */
export function storeT(key: string): string {
  const locale = readLocale();
  return (
    walk((messages as any)[locale], key) ??
    walk(messages.en, key) ??
    key
  );
}
