export interface AgentMetricSummary {
  provider: string;
  total: number;
  successes: number;
  success_rate_pct: string;
  median_duration_ms: number;
  total_cost_usd: number;
}

export interface AgentMetricSummaryResponse {
  days: number;
  providers: AgentMetricSummary[];
}
