import { useEffect } from 'react'
import { useTraceStore } from '../hooks/useTraces'

export function TraceDetail({ taskId, runId }: { taskId: string; runId?: string }) {
  const { currentSteps, isLoading, error, fetchTraceDetail } = useTraceStore()

  useEffect(() => {
    fetchTraceDetail(taskId, runId)
  }, [taskId, runId])

  if (isLoading) return <div className="text-muted-foreground">Loading trace...</div>
  if (error) return <div className="text-destructive">Error: {error}</div>
  if (currentSteps.length === 0) return <div className="text-muted-foreground">No trace steps found.</div>

  return (
    <div className="space-y-2">
      {currentSteps.map((step, idx) => (
        <div key={step.id || idx} className="border rounded p-3">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground text-sm">#{step.step_number}</span>
            <span className="font-medium">{step.action}</span>
            {step.tool && (
              <span className="text-xs bg-muted px-2 py-0.5 rounded">{step.tool}</span>
            )}
          </div>
          {step.input_text && (
            <div className="mt-2 text-sm">
              <div className="text-muted-foreground">Input:</div>
              <pre className="bg-muted p-2 rounded text-xs overflow-x-auto">{step.input_text}</pre>
            </div>
          )}
          {step.output_text && (
            <div className="mt-2 text-sm">
              <div className="text-muted-foreground">Output:</div>
              <pre className="bg-muted p-2 rounded text-xs overflow-x-auto">{step.output_text}</pre>
            </div>
          )}
        </div>
      ))}
    </div>
  )
}