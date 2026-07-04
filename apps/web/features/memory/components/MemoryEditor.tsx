'use client'

import { useState } from 'react'
import { useTranslations } from 'next-intl'
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
  const t = useTranslations('memory')
  const tc = useTranslations('common')
  const { workspace } = useWorkspaceStore()
  const { storeMemory } = useMemoryStore()
  const [content, setContent] = useState('')
  const [memoryType, setMemoryType] = useState<string | null>('learning')
  const [isPrivate, setIsPrivate] = useState(true)

  const handleSave = async () => {
    if (!workspace?.id || !content.trim() || !memoryType) return
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
          <DialogTitle>{t('editor.title')}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Select value={memoryType ?? 'learning'} onValueChange={setMemoryType}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="learning">{t('editor.types.learning')}</SelectItem>
              <SelectItem value="task_result">{t('editor.types.taskResult')}</SelectItem>
              <SelectItem value="context">{t('editor.types.context')}</SelectItem>
              <SelectItem value="pattern">{t('editor.types.pattern')}</SelectItem>
            </SelectContent>
          </Select>
          <Textarea
            placeholder={t('editor.contentPlaceholder')}
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
            <label htmlFor="is-private" className="text-sm">{t('editor.private')}</label>
          </div>
          <Button onClick={handleSave}>{tc('save')}</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
