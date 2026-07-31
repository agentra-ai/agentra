"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Plus, Pencil, Trash2, Check, X } from "lucide-react";
import { api } from "@/shared/api";
import type { Issue, Milestone, Project } from "@/shared/types";
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

interface ProjectDetailProps {
  workspaceId: string;
  projectId: string;
  canManage: boolean;
  onDeleted: () => void;
}

export function ProjectDetail({
  workspaceId,
  projectId,
  canManage,
  onDeleted,
}: ProjectDetailProps) {
  const t = useTranslations("projects");
  const tCommon = useTranslations("common");
  const tIssues = useTranslations("issues");

  const [project, setProject] = useState<Project | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [unassignedIssues, setUnassignedIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");

  const [showMilestoneDialog, setShowMilestoneDialog] = useState(false);
  const [milestoneTitle, setMilestoneTitle] = useState("");
  const [showIssueDialog, setShowIssueDialog] = useState(false);
  const [updatingIssueId, setUpdatingIssueId] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [projData, issueData, milestoneData, unassignedData] = await Promise.all([
        api.getProject(workspaceId, projectId),
        api.listProjectIssues(workspaceId, projectId),
        api.listMilestones(workspaceId, projectId),
        api.listUnassignedIssues(workspaceId),
      ]);
      setProject(projData);
      setIssues(issueData);
      setMilestones(milestoneData);
      setUnassignedIssues(unassignedData);
    } catch (err) {
      console.error("Failed to load project details:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
    } finally {
      setLoading(false);
    }
  }, [projectId, tCommon, workspaceId]);

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
      setError(err instanceof Error ? err.message : tCommon("error"));
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(t("confirmDelete"))) return;
    try {
      await api.deleteProject(workspaceId, projectId);
      onDeleted();
    } catch (err) {
      console.error("Failed to delete project:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
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
      setError(err instanceof Error ? err.message : tCommon("error"));
    }
  };

  const handleMilestoneStatus = async (milestoneId: string, newStatus: string) => {
    try {
      await api.updateMilestone(workspaceId, projectId, milestoneId, { status: newStatus });
      loadData();
    } catch (err) {
      console.error("Failed to update milestone:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
    }
  };

  const handleAssignIssue = async (issueId: string) => {
    try {
      setUpdatingIssueId(issueId);
      setError(null);
      await api.assignIssueToProject(
        workspaceId,
        projectId,
        issueId,
        "assign",
      );
      await loadData();
      setShowIssueDialog(false);
    } catch (err) {
      console.error("Failed to assign issue:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
    } finally {
      setUpdatingIssueId(null);
    }
  };

  const handleRemoveIssue = async (issueId: string) => {
    try {
      setUpdatingIssueId(issueId);
      setError(null);
      await api.assignIssueToProject(
        workspaceId,
        projectId,
        issueId,
        "remove",
      );
      await loadData();
    } catch (err) {
      console.error("Failed to remove issue:", err);
      setError(err instanceof Error ? err.message : tCommon("error"));
    } finally {
      setUpdatingIssueId(null);
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
      <div className="flex flex-1 flex-col items-center justify-center gap-3">
        <p className="text-sm text-destructive">
          {error ?? tCommon("noResults")}
        </p>
        <Button size="sm" variant="outline" onClick={loadData}>
          {tCommon("retry")}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col overflow-y-auto p-6">
      {/* Header */}
      <div className="mb-6 flex items-start justify-between">
        <div className="flex items-center gap-2">
          {canManage && editingTitle ? (
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
              <h2 className="text-xl font-semibold">{project.title}</h2>
              {canManage && (
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={t("edit")}
                  onClick={() => {
                    setTitleDraft(project.title);
                    setEditingTitle(true);
                  }}
                >
                  <Pencil className="h-3.5 w-3.5" />
                </Button>
              )}
            </>
          )}
        </div>
        {canManage && (
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={t("delete")}
            onClick={handleDelete}
          >
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        )}
      </div>

      {error && (
        <div className="mb-4 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      {project.deadline && (
        <p className="mb-6 text-xs text-muted-foreground">
          {t("deadline")}: {new Date(project.deadline).toLocaleDateString()}
        </p>
      )}

      {/* Milestones */}
      <div className="mb-6">
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-sm font-medium">{t("milestones")}</h2>
          {canManage && (
            <Button size="sm" variant="ghost" onClick={() => setShowMilestoneDialog(true)}>
              <Plus className="mr-1 h-3 w-3" />
              {t("newMilestone")}
            </Button>
          )}
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
                  {canManage ? (
                    <Button
                      size="sm"
                      variant="outline"
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
                    </Button>
                  ) : (
                    <Badge variant="outline">{t(`status.${m.status}`)}</Badge>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Issues */}
      <div>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-medium">{t("issues")}</h3>
          {canManage && (
            <Button size="sm" variant="ghost" onClick={() => setShowIssueDialog(true)}>
              <Plus className="mr-1 h-3 w-3" />
              {t("assignIssue")}
            </Button>
          )}
        </div>
        {issues.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t("noIssues")}</p>
        ) : (
          <div className="space-y-1">
            {issues.map((issue) => (
              <div
                key={issue.id}
                className="flex items-center justify-between rounded border px-3 py-2 text-sm"
              >
                <Link
                  href={`/issues/${issue.id}`}
                  className="flex min-w-0 flex-1 items-center hover:underline"
                >
                  <span className="mr-2 text-xs text-muted-foreground">{issue.identifier}</span>
                  <span className="flex-1 truncate">{issue.title}</span>
                </Link>
                <Badge variant="outline" className="ml-2 shrink-0">
                  {tIssues(`status.${issue.status}`)}
                </Badge>
                {canManage && (
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    className="ml-1 shrink-0"
                    aria-label={t("removeIssue")}
                    disabled={updatingIssueId === issue.id}
                    onClick={() => handleRemoveIssue(issue.id)}
                  >
                    <X className="h-3.5 w-3.5" />
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* New Milestone Dialog */}
      <Dialog open={canManage && showMilestoneDialog} onOpenChange={setShowMilestoneDialog}>
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

      <Dialog open={canManage && showIssueDialog} onOpenChange={setShowIssueDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("assignIssue")}</DialogTitle>
          </DialogHeader>
          <div className="max-h-80 space-y-1 overflow-y-auto py-2">
            {unassignedIssues.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">
                {t("noIssues")}
              </p>
            ) : (
              unassignedIssues.map((issue) => (
                <Button
                  key={issue.id}
                  variant="ghost"
                  className="h-auto w-full justify-start py-2 text-left"
                  disabled={updatingIssueId === issue.id}
                  onClick={() => handleAssignIssue(issue.id)}
                >
                  <span className="mr-2 text-xs text-muted-foreground">
                    {issue.identifier}
                  </span>
                  <span className="truncate">{issue.title}</span>
                </Button>
              ))
            )}
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setShowIssueDialog(false)}>
              {t("cancel")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
