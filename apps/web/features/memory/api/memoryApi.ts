import { api } from '@/shared/api'
import type { AgentMemory, TeamMemory } from '@/shared/types'

export const memoryApi = {
  listTeamMemories: (workspaceId: string) =>
    api.get<TeamMemory[]>(`/workspaces/${workspaceId}/memories`),

  listAgentMemories: (agentId: string) =>
    api.get<AgentMemory[]>(`/agents/${agentId}/memories`),

  storeMemory: (data: {
    workspace_id: string
    agent_id?: string
    memory_type: string
    content: string
    is_private?: boolean
  }) => api.post('/workspaces/' + data.workspace_id + '/memories', data),

  deleteMemory: (id: string, agentId?: string) =>
    api.delete(`/memories/${id}?agent_id=${agentId || ''}`),

  searchMemories: (workspaceId: string, query: string, includeTeam = true) =>
    api.get('/memories/search', { params: { workspace_id: workspaceId, q: query, include_team: includeTeam } }),
}