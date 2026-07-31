"use client";

import { useState, useCallback, useEffect } from "react";
import { useWSEvent, useWSReconnect } from "@/features/realtime/hooks";
import { api } from "@/shared/api";
import type { TaskMessagePayload } from "@/shared/types/events";

/**
 * Cap on the number of log lines kept in memory for a single task.
 * Long-running agents can stream thousands of lines per minute; without
 * a cap, the browser tab's heap will grow without bound and the page
 * will eventually crash. 5,000 lines is roughly 1-2 MB of text — plenty
 * for a human to scroll through, small enough to never OOM.
 */
const MAX_LOG_LINES = 5000;

interface LogEntry {
  key: string;
  seq: number;
  order: number;
  text: string;
}

export function taskMessagesToLogEntries(messages: TaskMessagePayload[]): LogEntry[] {
  const entries: LogEntry[] = [];
  for (const message of messages) {
    if (message.content) {
      entries.push({ key: `${message.seq}:content`, seq: message.seq, order: 0, text: message.content });
    }
    if (message.output) {
      entries.push({ key: `${message.seq}:output`, seq: message.seq, order: 1, text: message.output });
    }
  }
  return entries;
}

export function mergeLogEntries(current: LogEntry[], incoming: LogEntry[], limit = MAX_LOG_LINES): LogEntry[] {
  const byKey = new Map<string, LogEntry>();
  for (const entry of current) byKey.set(entry.key, entry);
  for (const entry of incoming) byKey.set(entry.key, entry);
  return [...byKey.values()]
    .sort((a, b) => a.seq - b.seq || a.order - b.order)
    .slice(-limit);
}

/**
 * Hook that subscribes to streaming logs for a specific task.
 * Aggregates log lines from task:message events, keeping at most
 * MAX_LOG_LINES entries in memory.
 */
export function useStreamingLogs(taskId: string | null) {
  const [entries, setEntries] = useState<LogEntry[]>([]);

  useEffect(() => {
    let cancelled = false;
    setEntries([]);
    if (!taskId) return () => { cancelled = true; };

    api.listTaskMessages(taskId, { limit: MAX_LOG_LINES }).then((messages) => {
      if (!cancelled) setEntries(taskMessagesToLogEntries(messages).slice(-MAX_LOG_LINES));
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [taskId]);

  useWSEvent("task:message", useCallback((payload: unknown) => {
    const p = payload as TaskMessagePayload;
    if (p.task_id !== taskId) return;

    setEntries((prev) => mergeLogEntries(prev, taskMessagesToLogEntries([p])));
  }, [taskId]));

  useWSReconnect(useCallback(() => {
    if (!taskId) return;
    api.listTaskMessages(taskId, { limit: MAX_LOG_LINES }).then((messages) => {
      setEntries((prev) => mergeLogEntries(prev, taskMessagesToLogEntries(messages)));
    }).catch(console.error);
  }, [taskId]));

  const clearLogs = useCallback(() => {
    setEntries([]);
  }, []);

  return { logLines: entries.map((entry) => entry.text), clearLogs };
}
