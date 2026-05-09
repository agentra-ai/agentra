export interface GitHubInstallation {
  id: string
  workspace_id: string
  installation_id: number
  account_login: string
  account_type: string
  access_token: string
  repositories: string[]
  created_at: string
  updated_at: string
}

export const githubApi = {
  getInstallation: async (workspaceId: string): Promise<GitHubInstallation | null> => {
    try {
      const res = await fetch(`/api/workspaces/${workspaceId}/github/installations`)
      if (!res.ok) return null
      return res.json()
    } catch {
      return null
    }
  },

  connect: async (workspaceId: string, data: {
    installation_id: number
    account_login: string
    account_type: string
    access_token: string
  }): Promise<GitHubInstallation> => {
    const res = await fetch(`/api/workspaces/${workspaceId}/github/connect`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    })
    return res.json()
  },

  disconnect: async (workspaceId: string): Promise<void> => {
    await fetch(`/api/workspaces/${workspaceId}/github/disconnect`, { method: 'DELETE' })
  },
}