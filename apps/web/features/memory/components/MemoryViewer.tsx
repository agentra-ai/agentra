import { useEffect, useState } from 'react'
import { useWorkspaceStore } from '@/features/workspace'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { MemoryList } from './MemoryList'
import { MemorySearch } from './MemorySearch'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function MemoryViewer() {
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
        <h2 className="text-lg font-semibold">Memory</h2>
        <Button size="sm">+ Add Memory</Button>
      </div>
      <MemorySearch workspaceId={workspace?.id} />
      <Tabs value={activeTab} onValueChange={v => setActiveTab(v as typeof activeTab)}>
        <TabsList>
          <TabsTrigger value="all">All</TabsTrigger>
          <TabsTrigger value="learning">Learnings</TabsTrigger>
          <TabsTrigger value="task_result">Results</TabsTrigger>
          <TabsTrigger value="context">Context</TabsTrigger>
          <TabsTrigger value="pattern">Patterns</TabsTrigger>
        </TabsList>
        <TabsContent value={activeTab}>
          {isLoading ? <div>Loading...</div> : <MemoryList memories={filtered} />}
        </TabsContent>
      </Tabs>
    </div>
  )
}
