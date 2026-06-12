"use client";

import { useState } from "react";
import { Search, Brain } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
  EmptyMedia,
} from "@/components/ui/empty";

type MemoryFilter = "all" | "learnings" | "results" | "context" | "patterns";

const FILTERS: { value: MemoryFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "learnings", label: "Learnings" },
  { value: "results", label: "Results" },
  { value: "context", label: "Context" },
  { value: "patterns", label: "Patterns" },
];

export function MemoryTab() {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<MemoryFilter>("all");

  return (
    <div className="space-y-6">
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Brain className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Memory</h2>
        </div>

        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search memories..."
            className="pl-9"
          />
        </div>

        <div className="flex flex-wrap items-center gap-2 border-b">
          {FILTERS.map((f) => {
            const active = filter === f.value;
            return (
              <button
                key={f.value}
                type="button"
                onClick={() => setFilter(f.value)}
                className={
                  "px-3 py-1.5 text-sm transition-colors -mb-px border-b-2 " +
                  (active
                    ? "border-foreground text-foreground font-medium"
                    : "border-transparent text-muted-foreground hover:text-foreground")
                }
                aria-pressed={active}
              >
                {f.label}
              </button>
            );
          })}
        </div>

        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Brain />
            </EmptyMedia>
            <EmptyTitle>No memories yet.</EmptyTitle>
            <EmptyDescription>
              Memories you save will show up here. Use search or filters to find them later.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </section>
    </div>
  );
}
