import { create } from 'zustand'

interface TraceStep {
  id: string
  step_number: number
  action: string
  tool?: string
  input_text?: string
  output_text?: string
  timestamp: string
}

interface TaskRun {
  id: string
  task_id: string
  agent_id: string
  status: string
  total_steps: number
  total_tokens: number
  total_cost: number
  duration_ms: number
  created_at: string
}

interface TraceState {
  runs: TaskRun[]
  currentSteps: TraceStep[]
  isLoading: boolean
  error: string | null
  fetchTraces: (agentId: string) => Promise<void>
  fetchTraceDetail: (taskId: string, runId?: string) => Promise<void>
}

export const useTraceStore = create<TraceState>((set) => ({
  runs: [],
  currentSteps: [],
  isLoading: false,
  error: null,

  fetchTraces: async (agentId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await fetch(`/api/agents/${agentId}/traces`)
      const data = await res.json()
      set({ runs: Array.isArray(data) ? data : [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  fetchTraceDetail: async (taskId: string, runId?: string) => {
    set({ isLoading: true, error: null })
    try {
      const url = runId ? `/api/tasks/${taskId}/trace?run_id=${runId}` : `/api/tasks/${taskId}/trace`
      const res = await fetch(url)
      const data = await res.json()
      set({ currentSteps: data.steps || [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },
}))