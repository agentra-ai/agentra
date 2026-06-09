import { api } from "@/shared/api";
import type { Loop, StartLoopRequest } from "@/shared/types/loop";

export async function listLoops(): Promise<Loop[]> {
  const res = await api.get<{ loops: Loop[] }>("/api/loops");
  return res.loops;
}

export async function getLoop(id: string): Promise<Loop> {
  return api.get<Loop>(`/api/loops/${id}`);
}

export async function startLoop(input: StartLoopRequest): Promise<Loop> {
  return api.post<Loop>("/api/loops", input);
}

export async function pauseLoop(id: string): Promise<Loop> {
  return api.post<Loop>(`/api/loops/${id}/pause`);
}

export async function resumeLoop(id: string): Promise<Loop> {
  return api.post<Loop>(`/api/loops/${id}/resume`);
}

export async function cancelLoop(id: string): Promise<Loop> {
  return api.post<Loop>(`/api/loops/${id}/cancel`);
}
