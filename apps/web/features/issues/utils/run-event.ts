/**
 * A missing identity is tolerated for legacy/non-attempt events. Once both
 * sides identify a Run, only the active Run may mutate live UI state.
 */
export function isCurrentRunEvent(
  activeRunID: string | null,
  eventRunID: string | null | undefined,
): boolean {
  return activeRunID === null || eventRunID == null || activeRunID === eventRunID;
}

/**
 * An async snapshot is fresh when no newer dispatch arrived while it was in
 * flight, or when it already describes that newer Run.
 */
export function isFreshRunSnapshot(
  runAtRequest: string | null,
  currentRunID: string | null,
  snapshotRunID: string | null,
): boolean {
  return currentRunID === runAtRequest || currentRunID === snapshotRunID;
}
