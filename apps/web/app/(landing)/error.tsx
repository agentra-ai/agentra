"use client";

import { useEffect } from "react";

/**
 * Next.js error boundary for the landing route group. Catches any
 * unhandled exception that escapes the page (i.e. not already caught
 * by SceneErrorBoundary inside the 3D container) and renders a
 * static dark section so the user sees the page headline instead of
 * a white screen.
 */
export default function LandingError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Landing route error:", error);
  }, [error]);

  return (
    <section className="min-h-screen bg-landing-bg px-6 py-20 text-white">
      <div className="mx-auto max-w-2xl space-y-4">
        <h1 className="text-3xl font-semibold sm:text-4xl">
          Make coding agents the team they should be.
        </h1>
        <p className="text-sm text-white/72 sm:text-base">
          The interactive product preview hit an unexpected error. You can
          retry, or come back in a minute — the rest of the page is back to
          normal.
        </p>
        <button
          type="button"
          onClick={reset}
          className="rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm font-medium text-white transition hover:bg-white/10"
        >
          Retry
        </button>
      </div>
    </section>
  );
}
