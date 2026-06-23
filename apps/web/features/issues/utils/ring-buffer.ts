/**
 * Ring-buffer helpers for capped in-memory lists. The streaming log
 * viewer can receive unbounded content from a long-running task;
 * without a cap, a browser tab will OOM after a few hours of agent
 * output. `appendCapped` keeps the most recent `max` items, dropping
 * the oldest when the cap is exceeded.
 */

/**
 * Returns a new array with `next` appended to `prev`, truncated to
 * the most recent `max` items. If `prev` already has at least `max`
 * items, the oldest entries are dropped from the front so the total
 * length never exceeds `max`.
 *
 * Pure: never mutates `prev`. Always returns a fresh array.
 */
export function appendCapped<T>(prev: readonly T[], next: T, max: number): T[] {
  if (!Number.isFinite(max) || max <= 0) {
    // Treat invalid max as "no cap" so callers can pass 0 to disable
    // capping in development. We still return a new array to keep
    // the function pure.
    return [...prev, next];
  }
  const out = prev.length >= max ? prev.slice(prev.length - max + 1) : prev.slice();
  out.push(next);
  return out;
}
