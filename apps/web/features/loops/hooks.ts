"use client";

import { useEffect, useCallback } from "react";
import * as loopsApi from "@/shared/api/loops";
import { useLoopStore } from "./store";
import { useWSEvent } from "@/features/realtime";
import type { Loop, StartLoopRequest } from "@/shared/types/loop";

function isLoop(value: unknown): value is Loop {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.id === "string" && typeof candidate.issue_id === "string";
}

export function useLoops(): Loop[] {
  const loopIds = useLoopStore((s) => s.loopIds);
  const loops = useLoopStore((s) => s.loops);
  const setLoops = useLoopStore((s) => s.setLoops);
  const upsertLoop = useLoopStore((s) => s.upsertLoop);

  useEffect(() => {
    let cancelled = false;
    loopsApi.listLoops()
      .then((items) => {
        if (!cancelled) setLoops(items);
      })
      .catch((err) => {
        console.error("failed to list loops", err);
      });
    return () => { cancelled = true; };
  }, [setLoops]);

  useWSEvent("loop:created", useCallback((payload: unknown) => {
    if (isLoop(payload)) upsertLoop(payload);
  }, [upsertLoop]));

  useWSEvent("loop:updated", useCallback((payload: unknown) => {
    if (isLoop(payload)) upsertLoop(payload);
  }, [upsertLoop]));

  return loopIds.map((id) => loops[id]).filter((loop): loop is Loop => loop !== undefined);
}

export function useLoop(id: string | null | undefined): Loop | null {
  const loop = useLoopStore((s) => (id ? s.loops[id] ?? null : null));
  const upsertLoop = useLoopStore((s) => s.upsertLoop);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    loopsApi.getLoop(id)
      .then((item) => {
        if (!cancelled) upsertLoop(item);
      })
      .catch((err) => {
        console.error("failed to get loop", err);
      });
    return () => { cancelled = true; };
  }, [id, upsertLoop]);

  useWSEvent("loop:updated", useCallback((payload: unknown) => {
    if (isLoop(payload) && payload.id === id) upsertLoop(payload);
  }, [id, upsertLoop]));

  useWSEvent("loop:stage_changed", useCallback((payload: unknown) => {
    if (isLoop(payload) && payload.id === id) upsertLoop(payload);
  }, [id, upsertLoop]));

  useWSEvent("loop:completed", useCallback((payload: unknown) => {
    if (isLoop(payload) && payload.id === id) upsertLoop(payload);
  }, [id, upsertLoop]));

  useWSEvent("loop:failed", useCallback((payload: unknown) => {
    if (isLoop(payload) && payload.id === id) upsertLoop(payload);
  }, [id, upsertLoop]));

  return loop;
}

export function useStartLoop() {
  const upsertLoop = useLoopStore((s) => s.upsertLoop);
  const setLoops = useLoopStore((s) => s.setLoops);

  return useCallback(async (input: StartLoopRequest): Promise<Loop> => {
    const loop = await loopsApi.startLoop(input);
    upsertLoop(loop);
    loopsApi.listLoops()
      .then(setLoops)
      .catch((err) => console.error("failed to refresh loops after start", err));
    return loop;
  }, [upsertLoop, setLoops]);
}

export function useLoopTransition(id: string) {
  const upsertLoop = useLoopStore((s) => s.upsertLoop);

  const pause = useCallback(async (): Promise<Loop> => {
    const loop = await loopsApi.pauseLoop(id);
    upsertLoop(loop);
    return loop;
  }, [id, upsertLoop]);

  const resume = useCallback(async (): Promise<Loop> => {
    const loop = await loopsApi.resumeLoop(id);
    upsertLoop(loop);
    return loop;
  }, [id, upsertLoop]);

  const cancel = useCallback(async (): Promise<Loop> => {
    const loop = await loopsApi.cancelLoop(id);
    upsertLoop(loop);
    return loop;
  }, [id, upsertLoop]);

  return { pause, resume, cancel };
}
