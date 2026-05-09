import { api } from "@/shared/api"

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
      const res = await api.get<GitHubInstallation>(`/workspaces/${workspaceId}/github/installations`)
      return res
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
    return api.post<GitHubInstallation, typeof data>(`/workspaces/${workspaceId}/github/connect`, data)
  },

  disconnect: async (workspaceId: string): Promise<void> => {
    await api.delete(`/workspaces/${workspaceId}/github/disconnect`)
  },
}