import { create } from 'zustand'

interface GitHubInstallation {
  id: string
  account_login: string
  account_type: string
  repositories: string[]
}

interface GitHubState {
  installation: GitHubInstallation | null
  isLoading: boolean
  error: string | null
  fetchInstallation: (workspaceId: string) => Promise<void>
  connect: (workspaceId: string, installationId: number) => Promise<void>
  disconnect: (workspaceId: string) => Promise<void>
}

export const useGitHubStore = create<GitHubState>((set) => ({
  installation: null,
  isLoading: false,
  error: null,

  fetchInstallation: async (workspaceId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await fetch(`/api/workspaces/${workspaceId}/github/installations`)
      const data = await res.json()
      set({ installation: data || null, isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  connect: async (workspaceId: string, installationId: number) => {
    const res = await fetch(`/api/workspaces/${workspaceId}/github/connect`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ installation_id: installationId }),
    })
    const data = await res.json()
    set({ installation: data })
  },

  disconnect: async (workspaceId: string) => {
    await fetch(`/api/workspaces/${workspaceId}/github/disconnect`, { method: 'DELETE' })
    set({ installation: null })
  },
}))