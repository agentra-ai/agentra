"use client";

import { useEffect } from 'react'
import { useTranslations } from 'next-intl'
import { useTraceStore } from '../hooks/useTraces'

export function TraceList({ agentId }: { agentId: string }) {
  const t = useTranslations('traces')
  const { runs, isLoading, error, fetchTraces } = useTraceStore()

  useEffect(() => {
    fetchTraces(agentId)
  }, [agentId])

  if (isLoading) return <div className="text-muted-foreground">{t('status.loading')}...</div>
  if (error) return <div className="text-destructive">{t('error', { detail: error })}</div>
  if (runs.length === 0) return <div className="text-muted-foreground">{t('status.empty')}</div>

  return (
    <div className="space-y-2">
      {runs.map((run) => (
        <div key={run.id} className="border rounded p-3">
          <div className="flex justify-between">
            <span className="font-medium">{run.status}</span>
            <span className="text-muted-foreground text-sm">{run.duration_ms}ms</span>
          </div>
          <div className="text-sm text-muted-foreground">
            {t('summary', {
              steps: run.total_steps,
              tokens: run.total_tokens,
              cost: run.total_cost.toFixed(4),
            })}
          </div>
        </div>
      ))}
    </div>
  )
}