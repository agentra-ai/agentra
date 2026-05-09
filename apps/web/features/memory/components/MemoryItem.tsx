import { useState } from 'react'
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

export function MemoryItem({ memory }: MemoryItemProps) {
  const { deleteMemory } = useMemoryStore()
  const [confirmDelete, setConfirmDelete] = useState(false)

  return (
    <div className="border rounded-lg p-3 hover:bg-muted/50 transition-colors">
      <div className="flex items-center gap-2 mb-2">
        <Badge className={TYPE_COLORS[memory.memory_type] || 'bg-gray-100'}>
          {memory.memory_type}
        </Badge>
        <span className="text-xs text-muted-foreground">
          {new Date(memory.created_at).toLocaleDateString()}
        </span>
      </div>
      <p className="text-sm whitespace-pre-wrap">{memory.content}</p>
      <div className="flex justify-end mt-2">
        {confirmDelete ? (
          <div className="flex gap-2">
            <Button size="xs" variant="destructive" onClick={() => deleteMemory(memory.id, memory.agent_id)}>
              Confirm
            </Button>
            <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(false)}>
              Cancel
            </Button>
          </div>
        ) : (
          <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(true)}>
            Delete
          </Button>
        )}
      </div>
    </div>
  )
}
