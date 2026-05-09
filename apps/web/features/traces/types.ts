export interface TraceStep {
  id: string
  step_number: number
  action: string
  tool?: string
  input_text?: string
  output_text?: string
  timestamp: string
  tokens_used?: number
  duration_ms?: number
  metadata?: Record<string, any>
}

export interface TaskRun {
  id: string
  task_id: string
  agent_id: string
  status: string
  started_at?: string
  completed_at?: string
  exit_code?: number
  total_steps: number
  total_tokens: number
  total_cost: number
  duration_ms: number
  output?: string
  error?: string
  created_at: string
}

export interface TaskRunSummary {
  task_id: string
  run_id: string
  total_steps: number
  total_tokens: number
  total_cost: number
  duration_ms: number
  tool_usage: Record<string, number>
  key_actions: string[]
}

export interface TraceAnalytics {
  total_runs: number
  avg_duration: number
  avg_tokens: number
  avg_cost: number
  completed_count: number
  failed_count: number
}