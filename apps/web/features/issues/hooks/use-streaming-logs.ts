"use client";

import { useState, useCallback } from "react";
import { useWSEvent } from "@/features/realtime/hooks";
import type { TaskMessagePayload } from "@/shared/types/events";

/**
 * Hook that subscribes to streaming logs for a specific task.
 * Aggregates log lines from task:message events.
 */
export function useStreamingLogs(taskId: string | null) {
  const [logLines, setLogLines] = useState<string[]>([]);

  useWSEvent("task:message", useCallback((payload: unknown) => {
    const p = payload as TaskMessagePayload;
    if (p.task_id !== taskId) return;

    // Add content as a log line
    if (p.content) {
      const content = p.content;
      setLogLines((prev) => [...prev, content]);
    }

    // Also add tool results as log lines
    if (p.output) {
      const output = p.output;
      setLogLines((prev) => [...prev, output]);
    }
  }, [taskId]));

  const clearLogs = useCallback(() => {
    setLogLines([]);
  }, []);

  return { logLines, clearLogs };
}
