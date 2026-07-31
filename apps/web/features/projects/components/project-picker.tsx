"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Plus, FolderOpen } from "lucide-react";
import { api } from "@/shared/api";
import type { Project } from "@/shared/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";

interface ProjectPickerProps {
  workspaceId: string;
  selectedProjectId: string | null;
  onSelect: (projectId: string | null) => void;
  canManage: boolean;
  refreshToken?: number;
}

export function ProjectPicker({
  workspaceId,
  selectedProjectId,
  onSelect,
  canManage,
  refreshToken = 0,
}: ProjectPickerProps) {
  const t = useTranslations("projects");
  const tCommon = useTranslations("common");
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [newSlug, setNewSlug] = useState("");

  const loadProjects = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listProjects(workspaceId);
      setProjects(data);
    } catch (err) {
      console.error("Failed to load projects:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
    } finally {
      setLoading(false);
    }
  }, [tCommon, workspaceId]);

  useEffect(() => {
    loadProjects();
  }, [loadProjects, refreshToken]);

  const generateSlug = (title: string): string => {
    return title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 40);
  };

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    const slug = newSlug.trim() || generateSlug(newTitle);
    try {
      const created = await api.createProject(workspaceId, {
        title: newTitle.trim(),
        slug,
      });
      setProjects((current) => [created, ...current]);
      setNewTitle("");
      setNewSlug("");
      setShowCreate(false);
      onSelect(created.id);
    } catch (err) {
      console.error("Failed to create project:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
    }
  };

  const handleTitleChange = (value: string) => {
    setNewTitle(value);
    if (!newSlug.trim()) {
      setNewSlug(generateSlug(value));
    }
  };

  return (
    <div className="flex h-auto max-h-64 w-full shrink-0 flex-col border-b bg-muted/20 md:h-full md:max-h-none md:w-64 md:border-r md:border-b-0">
      <div className="flex items-center justify-between border-b p-3">
        <span className="text-sm font-medium">{t("title")}</span>
        {canManage && (
          <Button
            variant="ghost"
            size="icon-sm"
            className="h-7 w-7"
            onClick={() => setShowCreate(true)}
            aria-label={t("newProject")}
          >
            <Plus className="h-4 w-4" />
          </Button>
        )}
      </div>

      <div className="flex-1 space-y-0.5 overflow-y-auto p-2">
        <Button
          variant={selectedProjectId === null ? "secondary" : "ghost"}
          size="sm"
          className="w-full justify-start gap-2 text-sm"
          onClick={() => onSelect(null)}
        >
          <FolderOpen className="h-4 w-4" />
          {t("unassigned")}
        </Button>

        {loading && (
          <p className="px-2 py-4 text-center text-xs text-muted-foreground">
            ...
          </p>
        )}

        {!loading && error && (
          <div className="space-y-2 px-2 py-4 text-center">
            <p className="text-xs text-destructive">{error}</p>
            <Button size="sm" variant="outline" onClick={loadProjects}>
              {tCommon("retry")}
            </Button>
          </div>
        )}

        {!loading && !error && projects.length === 0 && (
          <p className="px-2 py-4 text-center text-xs text-muted-foreground">
            {t("noProjects")}
          </p>
        )}

        {projects.map((project) => (
          <Button
            key={project.id}
            variant={selectedProjectId === project.id ? "secondary" : "ghost"}
            size="sm"
            className="w-full justify-start truncate text-sm"
            onClick={() => onSelect(project.id)}
            title={project.title}
          >
            <span className="truncate">{project.title}</span>
          </Button>
        ))}
      </div>

      <Dialog open={canManage && showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("newProject")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div>
              <label
                htmlFor="project-title"
                className="mb-1 block text-xs text-muted-foreground"
              >
                {t("projectTitle")}
              </label>
              <Input
                id="project-title"
                value={newTitle}
                onChange={(e) => handleTitleChange(e.target.value)}
                placeholder={t("projectTitle")}
              />
            </div>
            <div>
              <label
                htmlFor="project-slug"
                className="mb-1 block text-xs text-muted-foreground"
              >
                {t("projectSlug")}
              </label>
              <Input
                id="project-slug"
                value={newSlug}
                onChange={(e) => setNewSlug(e.target.value)}
                placeholder="my-project"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setShowCreate(false)}>
              {t("cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={!newTitle.trim()}>
              {t("create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
