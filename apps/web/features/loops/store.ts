"use client";

import { create } from "zustand";
import type { Loop } from "@/shared/types/loop";

interface LoopState {
  loops: Record<string, Loop>;
  loopIds: string[];
  loading: boolean;
  error: string | null;
  setLoops: (loops: Loop[]) => void;
  upsertLoop: (loop: Loop) => void;
  removeLoop: (id: string) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
}

export const useLoopStore = create<LoopState>((set) => ({
  loops: {},
  loopIds: [],
  loading: false,
  error: null,

  setLoops: (loops) =>
    set(() => {
      const next: Record<string, Loop> = {};
      const ids: string[] = [];
      for (const loop of loops) {
        next[loop.id] = loop;
        ids.push(loop.id);
      }
      return { loops: next, loopIds: ids };
    }),

  upsertLoop: (loop) =>
    set((s) => {
      const exists = Object.prototype.hasOwnProperty.call(s.loops, loop.id);
      return {
        loops: { ...s.loops, [loop.id]: loop },
        loopIds: exists ? s.loopIds : [...s.loopIds, loop.id],
      };
    }),

  removeLoop: (id) =>
    set((s) => {
      if (!Object.prototype.hasOwnProperty.call(s.loops, id)) return s;
      const { [id]: _removed, ...rest } = s.loops;
      return { loops: rest, loopIds: s.loopIds.filter((loopId) => loopId !== id) };
    }),

  setLoading: (loading) => set({ loading }),
  setError: (error) => set({ error }),
}));
