import type { TaskRun, TraceStep } from "../types"

export const tracesApi = {
  listAgentTraces: async (agentId: string): Promise<TaskRun[]> => {
    const res = await fetch(`/api/agents/${agentId}/traces`)
    return res.json()
  },

  getTaskTrace: async (taskId: string, runId?: string): Promise<{ task_id: string; run_id: string; steps: TraceStep[] }> => {
    const url = runId ? `/api/tasks/${taskId}/trace?run_id=${runId}` : `/api/tasks/${taskId}/trace`
    const res = await fetch(url)
    return res.json()
  },

  getTraceSummary: async (taskId: string, runId: string): Promise<any> => {
    const res = await fetch(`/api/tasks/${taskId}/trace/summary?run_id=${runId}`)
    return res.json()
  },

  getTraceAnalytics: async (agentId: string, period: string): Promise<any> => {
    const res = await fetch(`/api/traces/analytics?agent_id=${agentId}&period=${period}`)
    return res.json()
  },
}