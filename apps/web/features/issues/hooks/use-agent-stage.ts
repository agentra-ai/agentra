"use client";

import { useState, useCallback, useRef } from "react";
import { useWSEvent } from "@/features/realtime/hooks";
import type { AgentStage, AgentStagePayload, TaskDispatchPayload, TaskMessagePayload } from "@/shared/types/events";
import { isCurrentRunEvent } from "../utils/run-event";

/**
 * Hook that tracks the agent stage for a specific task.
 * Also infers stage from task:message events as a fallback.
 */
export function useAgentStage(taskId: string | null) {
  const [stageState, setStageState] = useState<{
    taskId: string | null;
    stage: AgentStage;
    lastUpdate: Date | null;
  }>({ taskId: null, stage: "idle", lastUpdate: null });
  const activeRun = useRef<{ taskId: string | null; runId: string | null }>({
    taskId: null,
    runId: null,
  });

  const currentRunID = useCallback(() => (
    activeRun.current.taskId === taskId ? activeRun.current.runId : null
  ), [taskId]);

  const setCurrentRunID = useCallback((runId: string | null) => {
    activeRun.current = { taskId, runId };
  }, [taskId]);

  useWSEvent("task:dispatch", useCallback((payload: unknown) => {
    const p = payload as TaskDispatchPayload;
    if (p.task_id !== taskId) return;
    setCurrentRunID(p.run_id);
    setStageState({ taskId, stage: "idle", lastUpdate: null });
  }, [setCurrentRunID, taskId]));

  // Subscribe to agent:stage events
  useWSEvent("agent:stage", useCallback((payload: unknown) => {
    const p = payload as AgentStagePayload;
    if (p.task_id === taskId) {
      const runID = currentRunID();
      if (!isCurrentRunEvent(runID, p.run_id)) return;
      if (runID === null) setCurrentRunID(p.run_id);
      setStageState({ taskId, stage: p.stage, lastUpdate: new Date() });
    }
  }, [currentRunID, setCurrentRunID, taskId]));

  // Infer stage from task:message events as fallback
  useWSEvent("task:message", useCallback((payload: unknown) => {
    const p = payload as TaskMessagePayload;
    if (p.task_id !== taskId || !p.content) return;
    const runID = currentRunID();
    if (!isCurrentRunEvent(runID, p.run_id)) return;
    if (runID === null) setCurrentRunID(p.run_id);

    const content = p.content.toLowerCase();
    const stage = stageState.taskId === taskId ? stageState.stage : "idle";

    // Don't override explicit stage events
    if (stage !== "idle") return;

    if (content.includes("reading") || content.includes("loading") || content.includes("fetching")) {
      setStageState({ taskId, stage: "reading", lastUpdate: new Date() });
    } else if (content.includes("implementing") || content.includes("writing") || content.includes("creating") || content.includes("modifying")) {
      setStageState({ taskId, stage: "implementing", lastUpdate: new Date() });
    } else if (content.includes("running") || content.includes("testing")) {
      setStageState({ taskId, stage: "testing", lastUpdate: new Date() });
    } else if (content.includes("committing") || content.includes("git commit") || content.includes("pushing")) {
      setStageState({ taskId, stage: "committing", lastUpdate: new Date() });
    }
  }, [currentRunID, setCurrentRunID, stageState, taskId]));

  const resetStage = useCallback(() => {
    setCurrentRunID(null);
    setStageState({ taskId, stage: "idle", lastUpdate: null });
  }, [setCurrentRunID, taskId]);

  const stage = stageState.taskId === taskId ? stageState.stage : "idle";
  const lastUpdate = stageState.taskId === taskId ? stageState.lastUpdate : null;

  return { stage, lastUpdate, resetStage };
}
