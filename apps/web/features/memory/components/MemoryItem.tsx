'use client'

import { useState } from 'react'
import { useTranslations } from 'next-intl'
import { useDateFormatter } from '@/shared/hooks/use-date-formatter'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const TYPE_COLORS: Record<string, string> = {
  learning: 'bg-yellow-100 text-yellow-800',
  task_result: 'bg-green-100 text-green-800',
  context: 'bg-blue-100 text-blue-800',
  pattern: 'bg-purple-100 text-purple-800',
}

interface MemoryItemProps {
  memory: {
    id: string
    memory_type: string
    content: string
    agent_id?: string
    created_at: string
  }
}

const TYPE_LABEL_KEYS: Record<string, 'learnings' | 'results' | 'context' | 'patterns'> = {
  learning: 'learnings',
  task_result: 'results',
  context: 'context',
  pattern: 'patterns',
}

export function MemoryItem({ memory }: MemoryItemProps) {
  const t = useTranslations('memory')
  const tc = useTranslations('common')
  const f = useDateFormatter()
  const { deleteMemory } = useMemoryStore()
  const [confirmDelete, setConfirmDelete] = useState(false)

  return (
    <div className="border rounded-lg p-3 hover:bg-muted/50 transition-colors">
      <div className="flex items-center gap-2 mb-2">
        <Badge className={TYPE_COLORS[memory.memory_type] || 'bg-gray-100'}>
          {t(`tabs.${TYPE_LABEL_KEYS[memory.memory_type] ?? 'all'}`)}
        </Badge>
        <span className="text-xs text-muted-foreground">
          {f.dateTime(new Date(memory.created_at), { dateStyle: 'short' })}
        </span>
      </div>
      <p className="text-sm whitespace-pre-wrap">{memory.content}</p>
      <div className="flex justify-end mt-2">
        {confirmDelete ? (
          <div className="flex gap-2">
            <Button size="xs" variant="destructive" onClick={() => deleteMemory(memory.id, memory.agent_id)}>
              {tc('confirm')}
            </Button>
            <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(false)}>
              {tc('cancel')}
            </Button>
          </div>
        ) : (
          <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(true)}>
            {tc('delete')}
          </Button>
        )}
      </div>
    </div>
  )
}
