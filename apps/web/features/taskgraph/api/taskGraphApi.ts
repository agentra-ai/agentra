import { api } from "@/shared/api";

export interface TaskGraphNode {
  id: string;
  workspace_id: string;
  issue_id: string;
  agent_id?: string;
  node_type: string;
  status: string;
  context: {
    description?: string;
    suggested_agent?: string;
    [key: string]: unknown;
  };
  result?: Record<string, unknown>;
  position_x: number;
  position_y: number;
  depth: number;
  created_at: string;
}

export interface TaskGraphEdge {
  id: string;
  from_node_id: string;
  to_node_id: string;
  edge_type: string;
  metadata?: Record<string, unknown>;
}

export interface TaskGraphResponse {
  nodes: TaskGraphNode[];
  edges: TaskGraphEdge[];
}

interface AutoDecomposeOptions {
  provider?: string;
  model?: string;
  maxNodes?: number;
  additionalContext?: string;
}

interface AutoDecomposeResponse extends TaskGraphResponse {
  plan: string;
  usage?: Record<string, unknown>;
}

export const taskGraphApi = {
  getGraph: (issueId: string): Promise<TaskGraphResponse> =>
    api.get<TaskGraphResponse>(`/api/issues/${issueId}/graph`),

  updateNode: (id: string, data: Partial<TaskGraphNode>): Promise<TaskGraphNode> =>
    api.patch<TaskGraphNode>(`/api/graph/nodes/${id}`, data),

  deleteNode: (id: string): Promise<void> =>
    api.delete(`/api/graph/nodes/${id}`),

  autoDecompose: (issueId: string, options: AutoDecomposeOptions = {}): Promise<AutoDecomposeResponse> =>
    api.post<AutoDecomposeResponse>(`/api/issues/${issueId}/auto-decompose`, {
      provider: options.provider,
      model: options.model,
      max_nodes: options.maxNodes,
      additional_context: options.additionalContext,
    }),
};
