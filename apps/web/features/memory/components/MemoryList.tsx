import { useTranslations } from "next-intl";
import type { MemoryEntry } from "../api/memoryApi";
import { MemoryItem } from "./MemoryItem";

interface MemoryListProps {
  memories: MemoryEntry[];
  onDeleted?: (memoryId: string) => void;
}

export function MemoryList({ memories, onDeleted }: MemoryListProps) {
  const t = useTranslations("memory");
  if (memories.length === 0) {
    return <p className="text-muted-foreground text-sm">{t("empty")}</p>;
  }
  return (
    <div className="flex flex-col gap-2">
      {memories.map((memory) => (
        <MemoryItem key={memory.id} memory={memory} onDeleted={onDeleted} />
      ))}
    </div>
  );
}
