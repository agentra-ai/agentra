import { create } from 'zustand'

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
      const res = await fetch(`/api/agents/${agentId}/memories`)
      const data = await res.json()
      set({ memories: Array.isArray(data) ? data : [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  fetchTeamMemories: async (workspaceId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await fetch(`/api/workspaces/${workspaceId}/memories`)
      const data = await res.json()
      set({ memories: Array.isArray(data) ? data : [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  storeMemory: async (data) => {
    const res = await fetch(`/api/workspaces/${data.workspace_id}/memories`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    })
    const newMem = await res.json()
    set(state => ({ memories: [newMem, ...state.memories] }))
  },

  deleteMemory: async (id: string, agentId?: string) => {
    await fetch(`/api/memories/${id}?agent_id=${agentId || ''}`, { method: 'DELETE' })
    set(state => ({ memories: state.memories.filter(m => m.id !== id) }))
  },
}))