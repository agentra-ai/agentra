"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { Plus, Pencil, Trash2, Check, X } from "lucide-react";
import { api } from "@/shared/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";

interface ProjectDetails {
  id: string;
  title: string;
  slug: string;
  deadline: string | null;
  created_at: string;
  updated_at: string;
}

interface Issue {
  id: string;
  identifier: string;
  title: string;
  status: string;
  priority: string;
}

interface Milestone {
  id: string;
  title: string;
  status: string;
  deadline: string | null;
}

interface ProjectDetailProps {
  projectId: string;
}

export function ProjectDetail({ projectId }: ProjectDetailProps) {
  const { id: workspaceId } = useParams<{ id: string }>();
  const t = useTranslations("projects");

  const [project, setProject] = useState<ProjectDetails | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [loading, setLoading] = useState(true);

  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");

  const [showMilestoneDialog, setShowMilestoneDialog] = useState(false);
  const [milestoneTitle, setMilestoneTitle] = useState("");

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      const [projData, issueData, milestoneData] = await Promise.all([
        api.getProject(workspaceId, projectId),
        api.listProjectIssues(workspaceId, projectId),
        api.listMilestones(workspaceId, projectId),
      ]);
      setProject(projData);
      setIssues(issueData);
      setMilestones(milestoneData);
    } catch (err) {
      console.error("Failed to load project details:", err);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, projectId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleTitleSave = async () => {
    if (!project || !titleDraft.trim()) return;
    try {
      const updated = await api.updateProject(workspaceId, projectId, { title: titleDraft.trim() });
      setProject({ ...project, title: updated.title ?? titleDraft.trim() });
      setEditingTitle(false);
    } catch (err) {
      console.error("Failed to update title:", err);
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(t("confirmDelete"))) return;
    try {
      await api.deleteProject(workspaceId, projectId);
      window.location.reload();
    } catch (err) {
      console.error("Failed to delete project:", err);
    }
  };

  const handleAddMilestone = async () => {
    if (!milestoneTitle.trim()) return;
    try {
      await api.createMilestone(workspaceId, projectId, { title: milestoneTitle.trim() });
      setMilestoneTitle("");
      setShowMilestoneDialog(false);
      loadData();
    } catch (err) {
      console.error("Failed to create milestone:", err);
    }
  };

  const handleMilestoneStatus = async (milestoneId: string, newStatus: string) => {
    try {
      await api.updateMilestone(workspaceId, projectId, milestoneId, { status: newStatus });
      loadData();
    } catch (err) {
      console.error("Failed to update milestone:", err);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <p className="text-sm text-muted-foreground">...</p>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <p className="text-sm text-muted-foreground">Project not found</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col overflow-y-auto p-6">
      {/* Header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2">
          {editingTitle ? (
            <div className="flex items-center gap-2">
              <Input
                value={titleDraft}
                onChange={(e) => setTitleDraft(e.target.value)}
                className="h-8 text-lg font-semibold"
                autoFocus
              />
              <Button size="icon-sm" variant="ghost" onClick={handleTitleSave}>
                <Check className="h-4 w-4" />
              </Button>
              <Button size="icon-sm" variant="ghost" onClick={() => setEditingTitle(false)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          ) : (
            <>
              <h1 className="text-xl font-semibold">{project.title}</h1>
              <Button
                size="icon-sm"
                variant="ghost"
                onClick={() => {
                  setTitleDraft(project.title);
                  setEditingTitle(true);
                }}
              >
                <Pencil className="h-3.5 w-3.5" />
              </Button>
            </>
          )}
        </div>
        <Button size="sm" variant="ghost" onClick={handleDelete}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      </div>

      {project.deadline && (
        <p className="mb-6 text-xs text-muted-foreground">
          {t("deadline")}: {new Date(project.deadline).toLocaleDateString()}
        </p>
      )}

      {/* Milestones */}
      <div className="mb-6">
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-sm font-medium">{t("milestones")}</h2>
          <Button size="sm" variant="ghost" onClick={() => setShowMilestoneDialog(true)}>
            <Plus className="mr-1 h-3 w-3" />
            {t("newMilestone")}
          </Button>
        </div>
        {milestones.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t("noMilestones")}</p>
        ) : (
          <div className="space-y-1">
            {milestones.map((m) => (
              <div
                key={m.id}
                className="flex items-center justify-between rounded border px-3 py-2 text-sm"
              >
                <span>{m.title}</span>
                <div className="flex items-center gap-2">
                  <Badge
                    variant={
                      m.status === "completed"
                        ? "secondary"
                        : m.status === "archived"
                          ? "outline"
                          : "default"
                    }
                    className="cursor-pointer"
                    onClick={() => {
                      const next =
                        m.status === "active"
                          ? "completed"
                          : m.status === "completed"
                            ? "archived"
                            : "active";
                      handleMilestoneStatus(m.id, next);
                    }}
                  >
                    {t(`status.${m.status}`)}
                  </Badge>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Issues */}
      <div>
        <h2 className="mb-2 text-sm font-medium">{t("issues")}</h2>
        {issues.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t("noIssues")}</p>
        ) : (
          <div className="space-y-1">
            {issues.map((issue) => (
              <div
                key={issue.id}
                className="flex items-center justify-between rounded border px-3 py-2 text-sm"
              >
                <span className="mr-2 text-xs text-muted-foreground">{issue.identifier}</span>
                <span className="flex-1 truncate">{issue.title}</span>
                <Badge variant="outline" className="ml-2 shrink-0">
                  {issue.status}
                </Badge>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* New Milestone Dialog */}
      <Dialog open={showMilestoneDialog} onOpenChange={setShowMilestoneDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("newMilestone")}</DialogTitle>
          </DialogHeader>
          <div className="py-2">
            <Input
              value={milestoneTitle}
              onChange={(e) => setMilestoneTitle(e.target.value)}
              placeholder={t("milestoneTitle")}
              autoFocus
            />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setShowMilestoneDialog(false)}>
              {t("cancel")}
            </Button>
            <Button onClick={handleAddMilestone} disabled={!milestoneTitle.trim()}>
              {t("create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
