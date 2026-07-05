import { create } from 'zustand'
import { api } from '@/shared/api'

interface MemoryEntry {
  id: string
  workspace_id: string
  agent_id?: string
  memory_type: 'learning' | 'task_result' | 'context' | 'pattern'
  content: string
  is_private: boolean
  embedding?: number[]
  created_at: string
  updated_at: string
}

interface MemoryState {
  memories: MemoryEntry[]
  isLoading: boolean
  error: string | null
  fetchAgentMemories: (agentId: string) => Promise<void>
  fetchTeamMemories: (workspaceId: string) => Promise<void>
  storeMemory: (data: { agent_id?: string; workspace_id: string; memory_type: string; content: string; is_private?: boolean }) => Promise<void>
  deleteMemory: (id: string, agentId?: string) => Promise<void>
}

export const useMemoryStore = create<MemoryState>((set) => ({
  memories: [],
  isLoading: false,
  error: null,

  fetchAgentMemories: async (agentId: string) => {
    set({ isLoading: true, error: null })
    try {
      const data = await api.get<MemoryEntry[]>(`/api/agents/${agentId}/memories`)
      set({ memories: Array.isArray(data) ? data : [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  fetchTeamMemories: async (workspaceId: string) => {
    set({ isLoading: true, error: null })
    try {
      const data = await api.get<MemoryEntry[]>(`/api/workspaces/${workspaceId}/memories`)
      set({ memories: Array.isArray(data) ? data : [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  storeMemory: async (data) => {
    const newMem = await api.post<MemoryEntry>(`/api/workspaces/${data.workspace_id}/memories`, data)
    set(state => ({ memories: [newMem, ...state.memories] }))
  },

  deleteMemory: async (id: string, agentId?: string) => {
    await api.delete(`/api/memories/${id}?agent_id=${agentId || ''}`)
    set(state => ({ memories: state.memories.filter(m => m.id !== id) }))
  },
}))