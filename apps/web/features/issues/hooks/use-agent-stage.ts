"use client";

import { useState, useEffect, useCallback } from "react";
import { useWSEvent } from "@/features/realtime/hooks";
import type { AgentStage, AgentStagePayload } from "@/shared/types/events";

/**
 * Hook that tracks the agent stage for a specific task.
 * Also infers stage from task:message events as a fallback.
 */
export function useAgentStage(taskId: string | null) {
  const [stage, setStage] = useState<AgentStage>("idle");
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  // Subscribe to agent:stage events
  useWSEvent("agent:stage", useCallback((payload: unknown) => {
    const p = payload as AgentStagePayload;
    if (p.task_id === taskId) {
      setStage(p.stage);
      setLastUpdate(new Date());
    }
  }, [taskId]));

  // Infer stage from task:message events as fallback
  useWSEvent("task:message", useCallback((payload: unknown) => {
    const p = payload as { task_id: string; content?: string };
    if (p.task_id !== taskId || !p.content) return;

    const content = p.content.toLowerCase();

    // Don't override explicit stage events
    if (stage !== "idle") return;

    if (content.includes("reading") || content.includes("loading") || content.includes("fetching")) {
      setStage("reading");
      setLastUpdate(new Date());
    } else if (content.includes("implementing") || content.includes("writing") || content.includes("creating") || content.includes("modifying")) {
      setStage("implementing");
      setLastUpdate(new Date());
    } else if (content.includes("running") || content.includes("testing")) {
      setStage("testing");
      setLastUpdate(new Date());
    } else if (content.includes("committing") || content.includes("git commit") || content.includes("pushing")) {
      setStage("committing");
      setLastUpdate(new Date());
    }
  }, [taskId, stage]));

  const resetStage = useCallback(() => {
    setStage("idle");
    setLastUpdate(null);
  }, []);

  return { stage, lastUpdate, resetStage };
}
