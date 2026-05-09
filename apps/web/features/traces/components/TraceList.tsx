import { useEffect } from 'react'
import { useTraceStore } from '../hooks/useTraces'

export function TraceList({ agentId }: { agentId: string }) {
  const { runs, isLoading, error, fetchTraces } = useTraceStore()

  useEffect(() => {
    fetchTraces(agentId)
  }, [agentId])

  if (isLoading) return <div className="text-muted-foreground">Loading traces...</div>
  if (error) return <div className="text-destructive">Error: {error}</div>
  if (runs.length === 0) return <div className="text-muted-foreground">No traces yet.</div>

  return (
    <div className="space-y-2">
      {runs.map((run) => (
        <div key={run.id} className="border rounded p-3">
          <div className="flex justify-between">
            <span className="font-medium">{run.status}</span>
            <span className="text-muted-foreground text-sm">{run.duration_ms}ms</span>
          </div>
          <div className="text-sm text-muted-foreground">
            {run.total_steps} steps, {run.total_tokens} tokens, ${run.total_cost.toFixed(4)}
          </div>
        </div>
      ))}
    </div>
  )
}