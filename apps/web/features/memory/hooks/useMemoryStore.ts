import { create } from 'zustand'
import { api } from '@/shared/api'
import type { AgentMemory, TeamMemory } from '@/shared/types'

interface MemoryEntry {
  id: string
  memory_type: 'learning' | 'task_result' | 'context' | 'pattern'
  content: string
  agent_id?: string
  score?: number
  created_at: string
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

export const useMemoryStore = create<MemoryState>((set, get) => ({
  memories: [],
  isLoading: false,
  error: null,

  fetchAgentMemories: async (agentId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await api.get(`/agents/${agentId}/memories`)
      set({ memories: res.data, isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  fetchTeamMemories: async (workspaceId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await api.get(`/workspaces/${workspaceId}/memories`)
      set({ memories: res.data, isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  storeMemory: async (data) => {
    const workspaceId = data.workspace_id
    const res = await api.post(`/workspaces/${workspaceId}/memories`, data)
    set(state => ({ memories: [res.data, ...state.memories] }))
  },

  deleteMemory: async (id: string, agentId?: string) => {
    await api.delete(`/memories/${id}?agent_id=${agentId || ''}`)
    set(state => ({ memories: state.memories.filter(m => m.id !== id) }))
  },
}))