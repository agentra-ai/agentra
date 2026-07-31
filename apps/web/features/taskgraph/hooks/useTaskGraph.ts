import { create } from "zustand";
import {
  taskGraphApi,
  type TaskGraphEdge,
  type TaskGraphNode,
} from "../api/taskGraphApi";

interface AutoDecomposeOptions {
  provider?: string;
  model?: string;
  maxNodes?: number;
  additionalContext?: string;
}

interface TaskGraphState {
  nodes: TaskGraphNode[];
  edges: TaskGraphEdge[];
  isLoading: boolean;
  error: string | null;
  fetchGraph: (issueId: string) => Promise<void>;
  updateNode: (id: string, data: Partial<TaskGraphNode>) => Promise<void>;
  deleteNode: (id: string) => Promise<void>;
  decomposeGraph: (issueId: string, options?: AutoDecomposeOptions) => Promise<void>;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown task graph error";
}

export const useTaskGraph = create<TaskGraphState>((set) => ({
  nodes: [],
  edges: [],
  isLoading: false,
  error: null,

  fetchGraph: async (issueId) => {
    set({ isLoading: true, error: null });
    try {
      const graph = await taskGraphApi.getGraph(issueId);
      set({ nodes: graph.nodes ?? [], edges: graph.edges ?? [], isLoading: false });
    } catch (error) {
      set({ error: errorMessage(error), isLoading: false });
    }
  },

  updateNode: async (id, data) => {
    try {
      const updated = await taskGraphApi.updateNode(id, data);
      set((state) => ({
        nodes: state.nodes.map((node) => node.id === id ? updated : node),
        error: null,
      }));
    } catch (error) {
      set({ error: errorMessage(error) });
      throw error;
    }
  },

  deleteNode: async (id) => {
    try {
      await taskGraphApi.deleteNode(id);
      set((state) => ({
        nodes: state.nodes.filter((node) => node.id !== id),
        edges: state.edges.filter((edge) => edge.from_node_id !== id && edge.to_node_id !== id),
        error: null,
      }));
    } catch (error) {
      set({ error: errorMessage(error) });
      throw error;
    }
  },

  decomposeGraph: async (issueId, options) => {
    set({ isLoading: true, error: null });
    try {
      const graph = await taskGraphApi.autoDecompose(issueId, options);
      set({ nodes: graph.nodes ?? [], edges: graph.edges ?? [], isLoading: false });
    } catch (error) {
      set({ error: errorMessage(error), isLoading: false });
      throw error;
    }
  },
}));
