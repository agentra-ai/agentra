import { useState } from 'react'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { useWorkspaceStore } from '@/features/workspace'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

interface MemoryEditorProps {
  open: boolean
  onClose: () => void
  agentId?: string
}

export function MemoryEditor({ open, onClose, agentId }: MemoryEditorProps) {
  const { workspace } = useWorkspaceStore()
  const { storeMemory } = useMemoryStore()
  const [content, setContent] = useState('')
  const [memoryType, setMemoryType] = useState<string>('learning')
  const [isPrivate, setIsPrivate] = useState(true)

  const handleSave = async () => {
    if (!workspace?.id || !content.trim()) return
    await storeMemory({
      workspace_id: workspace.id,
      agent_id: agentId,
      memory_type: memoryType,
      content,
      is_private: isPrivate,
    })
    setContent('')
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Memory</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Select value={memoryType} onValueChange={setMemoryType}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="learning">Learning</SelectItem>
              <SelectItem value="task_result">Task Result</SelectItem>
              <SelectItem value="context">Context</SelectItem>
              <SelectItem value="pattern">Pattern</SelectItem>
            </SelectContent>
          </Select>
          <Textarea
            placeholder="What should this agent remember?"
            value={content}
            onChange={e => setContent(e.target.value)}
            rows={4}
          />
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="is-private"
              checked={isPrivate}
              onChange={e => setIsPrivate(e.target.checked)}
            />
            <label htmlFor="is-private" className="text-sm">Private (only this agent)</label>
          </div>
          <Button onClick={handleSave}>Save</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
