"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useWorkspaceStore } from "@/features/workspace";
import type { MemoryType } from "../api/memoryApi";
import { useMemoryStore } from "../hooks/useMemoryStore";
import { MemoryEditor } from "./MemoryEditor";
import { MemoryList } from "./MemoryList";
import { MemorySearch } from "./MemorySearch";

type MemoryTab = "all" | MemoryType;

export function MemoryViewer() {
  const t = useTranslations("memory");
  const workspace = useWorkspaceStore((state) => state.workspace);
  const { memories, isLoading, error, fetchTeamMemories } = useMemoryStore();
  const [activeTab, setActiveTab] = useState<MemoryTab>("all");
  const [editorOpen, setEditorOpen] = useState(false);

  useEffect(() => {
    if (workspace?.id) void fetchTeamMemories(workspace.id);
  }, [workspace?.id, fetchTeamMemories]);

  const filtered = activeTab === "all"
    ? memories
    : memories.filter((memory) => memory.memory_type === activeTab);

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("title")}</h2>
        <Button size="sm" onClick={() => setEditorOpen(true)} disabled={!workspace?.id}>
          {t("addButton")}
        </Button>
      </div>
      <MemorySearch workspaceId={workspace?.id} />
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as MemoryTab)}>
        <TabsList>
          <TabsTrigger value="all">{t("tabs.all")}</TabsTrigger>
          <TabsTrigger value="learning">{t("tabs.learnings")}</TabsTrigger>
          <TabsTrigger value="task_result">{t("tabs.results")}</TabsTrigger>
          <TabsTrigger value="context">{t("tabs.context")}</TabsTrigger>
          <TabsTrigger value="pattern">{t("tabs.patterns")}</TabsTrigger>
        </TabsList>
        <TabsContent value={activeTab}>
          {isLoading ? <div>{t("loading")}</div> : <MemoryList memories={filtered} />}
        </TabsContent>
      </Tabs>
      <MemoryEditor open={editorOpen} onClose={() => setEditorOpen(false)} />
    </div>
  );
}
