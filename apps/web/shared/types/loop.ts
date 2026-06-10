export type LoopStatus = "pending" | "running" | "paused" | "done" | "failed" | "cancelled";
export type LoopStage = "plan" | "develop" | "review" | "fix";

export interface Loop {
  id: string;
  issue_id: string;
  workspace_id: string;
  status: LoopStatus;
  current_stage?: LoopStage;
  iteration: number;
  max_iterations: number;
  pr_url?: string;
  pr_number?: number;
  branch_name?: string;
  agent_id?: string;
  failure_reason?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface StartLoopRequest {
  issue_id: string;
  agent_id: string;
  max_iterations?: number;
  stage_agents?: Partial<Record<LoopStage, string>>;
}
