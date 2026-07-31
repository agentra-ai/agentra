"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useDateFormatter } from "@/shared/hooks/use-date-formatter";
import type { MemoryEntry } from "../api/memoryApi";
import { useMemoryStore } from "../hooks/useMemoryStore";

const TYPE_COLORS: Record<string, string> = {
  learning: "bg-secondary text-secondary-foreground",
  task_result: "bg-accent text-accent-foreground",
  context: "bg-primary/10 text-primary",
  pattern: "bg-muted text-muted-foreground",
};

const TYPE_LABEL_KEYS: Record<string, "learnings" | "results" | "context" | "patterns"> = {
  learning: "learnings",
  task_result: "results",
  context: "context",
  pattern: "patterns",
};

interface MemoryItemProps {
  memory: MemoryEntry;
  onDeleted?: (memoryId: string) => void;
}

export function MemoryItem({ memory, onDeleted }: MemoryItemProps) {
  const t = useTranslations("memory");
  const tc = useTranslations("common");
  const formatter = useDateFormatter();
  const deleteMemory = useMemoryStore((state) => state.deleteMemory);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const handleDelete = async () => {
    try {
      await deleteMemory(memory);
      onDeleted?.(memory.id);
      setConfirmDelete(false);
    } catch {
      // The feature store owns the user-visible error state.
    }
  };

  return (
    <div className="border rounded-lg p-3 hover:bg-muted/50 transition-colors">
      <div className="flex items-center gap-2 mb-2">
        <Badge className={TYPE_COLORS[memory.memory_type] || "bg-muted text-muted-foreground"}>
          {t(`tabs.${TYPE_LABEL_KEYS[memory.memory_type] ?? "all"}`)}
        </Badge>
        <span className="text-xs text-muted-foreground">
          {formatter.dateTime(new Date(memory.created_at), { dateStyle: "short" })}
        </span>
      </div>
      <p className="text-sm whitespace-pre-wrap">{memory.content}</p>
      <div className="flex justify-end mt-2">
        {confirmDelete ? (
          <div className="flex gap-2">
            <Button size="xs" variant="destructive" onClick={handleDelete}>
              {tc("confirm")}
            </Button>
            <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(false)}>
              {tc("cancel")}
            </Button>
          </div>
        ) : (
          <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(true)}>
            {tc("delete")}
          </Button>
        )}
      </div>
    </div>
  );
}
