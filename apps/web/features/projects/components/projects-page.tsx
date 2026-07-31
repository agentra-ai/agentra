"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { FolderKanban } from "lucide-react";
import { useTranslations } from "next-intl";
import { api } from "@/shared/api";
import type { Issue } from "@/shared/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, WorkspaceAvatar } from "@/features/workspace";
import { ProjectDetail } from "./project-detail";
import { ProjectPicker } from "./project-picker";

function UnassignedIssues({ workspaceId }: { workspaceId: string }) {
  const t = useTranslations("projects");
  const tCommon = useTranslations("common");
  const tIssues = useTranslations("issues");
  const [issues, setIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadIssues = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      setIssues(await api.listUnassignedIssues(workspaceId));
    } catch (loadError) {
      setError(
        loadError instanceof Error ? loadError.message : tCommon("error"),
      );
    } finally {
      setLoading(false);
    }
  }, [tCommon, workspaceId]);

  useEffect(() => {
    loadIssues();
  }, [loadIssues]);

  return (
    <div className="flex min-w-0 flex-1 flex-col overflow-y-auto p-4 sm:p-6">
      <h2 className="text-xl font-semibold">{t("unassigned")}</h2>
      <p className="mt-1 text-sm text-muted-foreground">{t("details")}</p>

      {loading ? (
        <p className="py-8 text-sm text-muted-foreground">
          {tCommon("loading")}
        </p>
      ) : error ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3">
          <p className="text-sm text-destructive">{error}</p>
          <Button size="sm" variant="outline" onClick={loadIssues}>
            {tCommon("retry")}
          </Button>
        </div>
      ) : issues.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 text-muted-foreground">
          <FolderKanban className="h-10 w-10 opacity-40" />
          <p className="text-sm">{t("noIssues")}</p>
        </div>
      ) : (
        <div className="mt-5 space-y-1">
          {issues.map((issue) => (
            <Link
              key={issue.id}
              href={`/issues/${issue.id}`}
              className="flex min-w-0 items-center rounded-md border px-3 py-2 text-sm transition-colors hover:bg-accent/50"
            >
              <span className="mr-2 shrink-0 text-xs text-muted-foreground">
                {issue.identifier}
              </span>
              <span className="min-w-0 flex-1 truncate">{issue.title}</span>
              <Badge variant="outline" className="ml-2 shrink-0">
                {tIssues(`status.${issue.status}`)}
              </Badge>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function ProjectsWorkspace({
  workspaceId,
  workspaceName,
  canManage,
}: {
  workspaceId: string;
  workspaceName: string;
  canManage: boolean;
}) {
  const t = useTranslations("projects");
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(
    null,
  );
  const [refreshToken, setRefreshToken] = useState(0);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        <WorkspaceAvatar name={workspaceName} size="sm" />
        <span className="truncate text-sm text-muted-foreground">
          {workspaceName}
        </span>
        <span className="text-muted-foreground">/</span>
        <h1 className="text-sm font-medium">{t("title")}</h1>
      </div>

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <ProjectPicker
          workspaceId={workspaceId}
          selectedProjectId={selectedProjectId}
          onSelect={setSelectedProjectId}
          canManage={canManage}
          refreshToken={refreshToken}
        />
        {selectedProjectId ? (
          <ProjectDetail
            workspaceId={workspaceId}
            projectId={selectedProjectId}
            canManage={canManage}
            onDeleted={() => {
              setSelectedProjectId(null);
              setRefreshToken((value) => value + 1);
            }}
          />
        ) : (
          <UnassignedIssues workspaceId={workspaceId} />
        )}
      </div>
    </div>
  );
}

export function ProjectsPage() {
  const user = useAuthStore((state) => state.user);
  const workspace = useWorkspaceStore((state) => state.workspace);
  const members = useWorkspaceStore((state) => state.members);

  if (!workspace) return null;

  const currentMember = members.find((member) => member.user_id === user?.id);
  const canManage =
    currentMember?.role === "owner" || currentMember?.role === "admin";

  return (
    <ProjectsWorkspace
      key={workspace.id}
      workspaceId={workspace.id}
      workspaceName={workspace.name}
      canManage={canManage}
    />
  );
}
