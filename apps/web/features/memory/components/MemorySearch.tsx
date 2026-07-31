"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { memoryApi, type MemoryEntry } from "../api/memoryApi";
import { MemoryList } from "./MemoryList";

interface MemorySearchProps {
  workspaceId?: string;
}

export function MemorySearch({ workspaceId }: MemorySearchProps) {
  const t = useTranslations("memory");
  const tc = useTranslations("common");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<MemoryEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isSearching, setIsSearching] = useState(false);

  const handleSearch = async () => {
    const normalizedQuery = query.trim();
    if (!normalizedQuery || !workspaceId) return;
    setIsSearching(true);
    setError(null);
    try {
      const response = await memoryApi.searchMemories(workspaceId, normalizedQuery);
      setResults(response.memories ?? []);
    } catch (searchError) {
      setError(searchError instanceof Error ? searchError.message : "Memory search failed");
    } finally {
      setIsSearching(false);
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <Input
          placeholder={t("search.placeholder")}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") void handleSearch();
          }}
        />
        <Button onClick={handleSearch} disabled={isSearching || !workspaceId || !query.trim()}>
          {isSearching ? tc("searching") : tc("search")}
        </Button>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {results && (
        <MemoryList
          memories={results}
          onDeleted={(memoryId) => {
            setResults((current) => current?.filter((memory) => memory.id !== memoryId) ?? null);
          }}
        />
      )}
    </div>
  );
}
