import { create } from "zustand";

interface GraphNode {
  id: string;
  workspace_id: string;
  issue_id: string;
  agent_id?: string;
  node_type: string;
  status: string;
  context: Record<string, any>;
  result?: Record<string, any>;
  position_x?: number;
  position_y?: number;
  depth: number;
  created_at: string;
}

interface GraphEdge {
  id: string;
  from_node_id: string;
  to_node_id: string;
  edge_type: string;
  metadata?: Record<string, any>;
}

interface TaskGraphState {
  nodes: GraphNode[];
  edges: GraphEdge[];
  isLoading: boolean;
  error: string | null;
  fetchGraph: (issueId: string) => Promise<void>;
  updateNode: (id: string, data: Partial<GraphNode>) => Promise<void>;
  deleteNode: (id: string) => Promise<void>;
}

export const useTaskGraph = create<TaskGraphState>((set) => ({
  nodes: [],
  edges: [],
  isLoading: false,
  error: null,

  fetchGraph: async (issueId: string) => {
    set({ isLoading: true, error: null });
    try {
      const res = await fetch(`/api/issues/${issueId}/graph`);
      const data = await res.json();
      set({ nodes: data.nodes || [], edges: data.edges || [], isLoading: false });
    } catch (e: any) {
      set({ error: e.message, isLoading: false });
    }
  },

  updateNode: async (id: string, data: Partial<GraphNode>) => {
    try {
      await fetch(`/api/graph/nodes/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      set((state) => ({
        nodes: state.nodes.map((n) => (n.id === id ? { ...n, ...data } : n)),
      }));
    } catch (e: any) {
      set({ error: e.message });
    }
  },

  deleteNode: async (id: string) => {
    try {
      await fetch(`/api/graph/nodes/${id}`, { method: "DELETE" });
      set((state) => ({ nodes: state.nodes.filter((n) => n.id !== id) }));
    } catch (e: any) {
      set({ error: e.message });
    }
  },
}));
