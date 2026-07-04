'use client'

import { useState } from 'react'
import { useTranslations } from 'next-intl'
import { memoryApi } from '../api/memoryApi'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface MemorySearchProps {
  workspaceId?: string
}

export function MemorySearch({ workspaceId }: MemorySearchProps) {
  const t = useTranslations('memory')
  const tc = useTranslations('common')
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
        placeholder={t('search.placeholder')}
        value={query}
        onChange={e => setQuery(e.target.value)}
        onKeyDown={e => e.key === 'Enter' && handleSearch()}
      />
      <Button onClick={handleSearch} disabled={isSearching}>
        {isSearching ? tc('searching') : tc('search')}
      </Button>
    </div>
  )
}
