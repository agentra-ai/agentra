"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useWorkspaceStore } from "@/features/workspace";
import type { MemoryType } from "../api/memoryApi";
import { useMemoryStore } from "../hooks/useMemoryStore";

interface MemoryEditorProps {
  open: boolean;
  onClose: () => void;
  agentId?: string;
}

export function MemoryEditor({ open, onClose, agentId }: MemoryEditorProps) {
  const t = useTranslations("memory");
  const tc = useTranslations("common");
  const workspace = useWorkspaceStore((state) => state.workspace);
  const storeMemory = useMemoryStore((state) => state.storeMemory);
  const [content, setContent] = useState("");
  const [memoryType, setMemoryType] = useState<MemoryType>("learning");
  const [isPrivate, setIsPrivate] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  const handleSave = async () => {
    if (!workspace?.id || !content.trim()) return;
    setIsSaving(true);
    try {
      await storeMemory({
        workspace_id: workspace.id,
        agent_id: agentId,
        memory_type: memoryType,
        content: content.trim(),
        is_private: isPrivate,
      });
      setContent("");
      onClose();
    } catch {
      // The feature store owns the user-visible error state.
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => {
      if (!nextOpen) onClose();
    }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("editor.title")}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Select value={memoryType} onValueChange={(value) => setMemoryType(value as MemoryType)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="learning">{t("editor.types.learning")}</SelectItem>
              <SelectItem value="task_result">{t("editor.types.taskResult")}</SelectItem>
              <SelectItem value="context">{t("editor.types.context")}</SelectItem>
              <SelectItem value="pattern">{t("editor.types.pattern")}</SelectItem>
            </SelectContent>
          </Select>
          <Textarea
            placeholder={t("editor.contentPlaceholder")}
            value={content}
            onChange={(event) => setContent(event.target.value)}
            rows={4}
          />
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="is-private"
              checked={isPrivate}
              onChange={(event) => setIsPrivate(event.target.checked)}
            />
            <label htmlFor="is-private" className="text-sm">{t("editor.private")}</label>
          </div>
          <Button onClick={handleSave} disabled={isSaving || !content.trim()}>
            {isSaving ? tc("saving") : tc("save")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
