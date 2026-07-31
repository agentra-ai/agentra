import { create } from "zustand";
import {
  memoryApi,
  type MemoryEntry,
  type StoreMemoryRequest,
} from "../api/memoryApi";

interface MemoryState {
  memories: MemoryEntry[];
  isLoading: boolean;
  error: string | null;
  fetchAgentMemories: (agentId: string) => Promise<void>;
  fetchTeamMemories: (workspaceId: string) => Promise<void>;
  storeMemory: (data: StoreMemoryRequest) => Promise<void>;
  deleteMemory: (memory: MemoryEntry) => Promise<void>;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown memory error";
}

export const useMemoryStore = create<MemoryState>((set) => ({
  memories: [],
  isLoading: false,
  error: null,

  fetchAgentMemories: async (agentId) => {
    set({ isLoading: true, error: null });
    try {
      const memories = await memoryApi.listAgentMemories(agentId);
      set({ memories: memories ?? [], isLoading: false });
    } catch (error) {
      set({ error: errorMessage(error), isLoading: false });
    }
  },

  fetchTeamMemories: async (workspaceId) => {
    set({ isLoading: true, error: null });
    try {
      const memories = await memoryApi.listTeamMemories(workspaceId);
      set({ memories: memories ?? [], isLoading: false });
    } catch (error) {
      set({ error: errorMessage(error), isLoading: false });
    }
  },

  storeMemory: async (data) => {
    try {
      const memory = await memoryApi.storeMemory(data);
      set((state) => ({ memories: [memory, ...state.memories], error: null }));
    } catch (error) {
      set({ error: errorMessage(error) });
      throw error;
    }
  },

  deleteMemory: async (memory) => {
    try {
      await memoryApi.deleteMemory(memory);
      set((state) => ({
        memories: state.memories.filter((item) => item.id !== memory.id),
        error: null,
      }));
    } catch (error) {
      set({ error: errorMessage(error) });
      throw error;
    }
  },
}));
