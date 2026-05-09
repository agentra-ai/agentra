export const memoryApi = {
  listTeamMemories: async (workspaceId: string) => {
    const res = await fetch(`/api/workspaces/${workspaceId}/memories`)
    return res.json()
  },

  listAgentMemories: async (agentId: string) => {
    const res = await fetch(`/api/agents/${agentId}/memories`)
    return res.json()
  },

  storeMemory: async (data: {
    workspace_id: string
    agent_id?: string
    memory_type: string
    content: string
    is_private?: boolean
  }) => {
    const res = await fetch(`/api/workspaces/${data.workspace_id}/memories`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    })
    return res.json()
  },

  deleteMemory: async (id: string, agentId?: string) => {
    await fetch(`/api/memories/${id}?agent_id=${agentId || ''}`, { method: 'DELETE' })
  },

  searchMemories: async (workspaceId: string, query: string, includeTeam = true) => {
    const params = new URLSearchParams({ workspace_id: workspaceId, q: query, include_team: String(includeTeam) })
    const res = await fetch(`/api/memories/search?${params}`)
    return res.json()
  },
}