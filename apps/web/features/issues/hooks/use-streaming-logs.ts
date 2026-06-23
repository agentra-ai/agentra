"use client";

import { useState, useCallback } from "react";
import { useWSEvent } from "@/features/realtime/hooks";
import type { TaskMessagePayload } from "@/shared/types/events";
import { appendCapped } from "../utils/ring-buffer";

/**
 * Cap on the number of log lines kept in memory for a single task.
 * Long-running agents can stream thousands of lines per minute; without
 * a cap, the browser tab's heap will grow without bound and the page
 * will eventually crash. 5,000 lines is roughly 1-2 MB of text — plenty
 * for a human to scroll through, small enough to never OOM.
 */
const MAX_LOG_LINES = 5000;

/**
 * Hook that subscribes to streaming logs for a specific task.
 * Aggregates log lines from task:message events, keeping at most
 * MAX_LOG_LINES entries in memory.
 */
export function useStreamingLogs(taskId: string | null) {
  const [logLines, setLogLines] = useState<string[]>([]);

  useWSEvent("task:message", useCallback((payload: unknown) => {
    const p = payload as TaskMessagePayload;
    if (p.task_id !== taskId) return;

    if (p.content) {
      const content = p.content;
      setLogLines((prev) => appendCapped(prev, content, MAX_LOG_LINES));
    }

    if (p.output) {
      const output = p.output;
      setLogLines((prev) => appendCapped(prev, output, MAX_LOG_LINES));
    }
  }, [taskId]));

  const clearLogs = useCallback(() => {
    setLogLines([]);
  }, []);

  return { logLines, clearLogs };
}
