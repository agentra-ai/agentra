import { api } from "@/shared/api";

export type MemoryType = "learning" | "task_result" | "context" | "pattern";

export interface MemoryEntry {
  id: string;
  workspace_id: string;
  agent_id?: string;
  memory_type: MemoryType;
  content: string;
  is_private?: boolean;
  created_at: string;
}

export interface StoreMemoryRequest {
  workspace_id: string;
  agent_id?: string;
  memory_type: MemoryType;
  content: string;
  is_private?: boolean;
}

export const memoryApi = {
  listTeamMemories: (workspaceId: string): Promise<MemoryEntry[]> =>
    api.get<MemoryEntry[]>(`/api/workspaces/${workspaceId}/memories`),

  listAgentMemories: (agentId: string): Promise<MemoryEntry[]> =>
    api.get<MemoryEntry[]>(`/api/agents/${agentId}/memories`),

  storeMemory: (data: StoreMemoryRequest): Promise<MemoryEntry> => {
    if (data.agent_id) {
      return api.post<MemoryEntry>(`/api/agents/${data.agent_id}/memories`, {
        memory_type: data.memory_type,
        content: data.content,
        is_private: data.is_private,
      });
    }
    return api.post<MemoryEntry>(`/api/workspaces/${data.workspace_id}/memories`, {
      memory_type: data.memory_type,
      content: data.content,
    });
  },

  deleteMemory: (memory: MemoryEntry): Promise<void> => {
    if (memory.agent_id) {
      return api.delete(`/api/agents/${memory.agent_id}/memories/${memory.id}`);
    }
    return api.delete(`/api/workspaces/${memory.workspace_id}/memories/${memory.id}`);
  },

  searchMemories: (workspaceId: string, query: string): Promise<{ memories: MemoryEntry[] }> => {
    const params = new URLSearchParams({ q: query });
    return api.get<{ memories: MemoryEntry[] }>(
      `/api/workspaces/${workspaceId}/memories/search?${params}`,
    );
  },
};
