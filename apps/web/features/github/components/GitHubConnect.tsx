import { useEffect } from 'react'
import { useGitHubStore } from '../hooks/useGitHub'
import { Button } from '@/components/ui/button'

export function GitHubConnect({ workspaceId }: { workspaceId: string }) {
  const { installation, isLoading, fetchInstallation, connect, disconnect } = useGitHubStore()

  useEffect(() => {
    fetchInstallation(workspaceId)
  }, [workspaceId])

  if (isLoading) return <div className="text-muted-foreground">Loading...</div>

  if (!installation) {
    return (
      <div className="border rounded p-4">
        <h3 className="font-semibold mb-2">Connect GitHub</h3>
        <p className="text-muted-foreground text-sm mb-4">
          Connect your GitHub account to enable PR status sync and automatic commits.
        </p>
        <Button onClick={() => window.location.href = '/api/github/oauth'}>
          Connect GitHub App
        </Button>
      </div>
    )
  }

  return (
    <div className="border rounded p-4">
      <div className="flex justify-between items-center">
        <div>
          <span className="font-medium">{installation.account_login}</span>
          <span className="text-muted-foreground text-sm ml-2">
            ({installation.account_type})
          </span>
        </div>
        <Button variant="destructive" onClick={() => disconnect(workspaceId)}>
          Disconnect
        </Button>
      </div>
    </div>
  )
}