'use client'

import { useEffect, useState } from 'react'
import { useTranslations } from 'next-intl'
import { useWorkspaceStore } from '@/features/workspace'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { MemoryList } from './MemoryList'
import { MemorySearch } from './MemorySearch'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function MemoryViewer() {
  const t = useTranslations('memory')
  const { workspace } = useWorkspaceStore()
  const { memories, isLoading, fetchTeamMemories } = useMemoryStore()
  const [activeTab, setActiveTab] = useState<'all' | 'learning' | 'task_result' | 'context' | 'pattern'>('all')

  useEffect(() => {
    if (workspace?.id) {
      fetchTeamMemories(workspace.id)
    }
  }, [workspace?.id, fetchTeamMemories])

  const filtered = activeTab === 'all'
    ? memories
    : memories.filter(m => m.memory_type === activeTab)

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t('title')}</h2>
        <Button size="sm">{t('addButton')}</Button>
      </div>
      <MemorySearch workspaceId={workspace?.id} />
      <Tabs value={activeTab} onValueChange={v => setActiveTab(v as typeof activeTab)}>
        <TabsList>
          <TabsTrigger value="all">{t('tabs.all')}</TabsTrigger>
          <TabsTrigger value="learning">{t('tabs.learnings')}</TabsTrigger>
          <TabsTrigger value="task_result">{t('tabs.results')}</TabsTrigger>
          <TabsTrigger value="context">{t('tabs.context')}</TabsTrigger>
          <TabsTrigger value="pattern">{t('tabs.patterns')}</TabsTrigger>
        </TabsList>
        <TabsContent value={activeTab}>
          {isLoading ? <div>{t('loading')}</div> : <MemoryList memories={filtered} />}
        </TabsContent>
      </Tabs>
    </div>
  )
}
