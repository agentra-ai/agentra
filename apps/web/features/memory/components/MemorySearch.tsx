import { useState } from 'react'
import { memoryApi } from '../api/memoryApi'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface MemorySearchProps {
  workspaceId?: string
}

export function MemorySearch({ workspaceId }: MemorySearchProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<any[]>([])
  const [isSearching, setIsSearching] = useState(false)

  const handleSearch = async () => {
    if (!query.trim() || !workspaceId) return
    setIsSearching(true)
    try {
      const res = await memoryApi.searchMemories(workspaceId, query)
      setResults(res.data.memories || [])
    } finally {
      setIsSearching(false)
    }
  }

  return (
    <div className="flex gap-2">
      <Input
        placeholder="Search memories..."
        value={query}
        onChange={e => setQuery(e.target.value)}
        onKeyDown={e => e.key === 'Enter' && handleSearch()}
      />
      <Button onClick={handleSearch} disabled={isSearching}>
        {isSearching ? 'Searching...' : 'Search'}
      </Button>
    </div>
  )
}
