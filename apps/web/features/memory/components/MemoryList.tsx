import { useTranslations } from 'next-intl'
import { MemoryItem } from './MemoryItem'

interface MemoryListProps {
  memories: Array<{
    id: string
    memory_type: string
    content: string
    agent_id?: string
    created_at: string
  }>
}

export function MemoryList({ memories }: MemoryListProps) {
  const t = useTranslations('memory')
  if (memories.length === 0) {
    return <p className="text-muted-foreground text-sm">{t('empty')}</p>
  }
  return (
    <div className="flex flex-col gap-2">
      {memories.map(m => (
        <MemoryItem key={m.id} memory={m} />
      ))}
    </div>
  )
}
