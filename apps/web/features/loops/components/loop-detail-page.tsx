"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Pause, Play, X, ExternalLink, GitBranch, AlertCircle, Loader2, ChevronLeft } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { LoopStatusBadge } from "./loop-status-badge";
import { LoopStageIndicator } from "./loop-stage-indicator";
import { useLoop, useLoopTransition } from "../hooks";
import type { LoopStatus } from "@/shared/types/loop";

interface LoopDetailPageProps {
  id: string;
}

function formatDateTime(iso?: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function LoopDetailPage({ id }: LoopDetailPageProps) {
  const t = useTranslations("loops");
  const tCommon = useTranslations("common");
  const router = useRouter();
  const loop = useLoop(id);
  const { pause, resume, cancel } = useLoopTransition(id);
  const [acting, setActing] = useState<"pause" | "resume" | "cancel" | null>(null);

  const onPause = useCallback(async () => {
    setActing("pause");
    try {
      await pause();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tCommon("error"));
    } finally {
      setActing(null);
    }
  }, [pause, tCommon]);

  const onResume = useCallback(async () => {
    setActing("resume");
    try {
      await resume();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tCommon("error"));
    } finally {
      setActing(null);
    }
  }, [resume, tCommon]);

  const onCancel = useCallback(async () => {
    setActing("cancel");
    try {
      await cancel();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tCommon("error"));
    } finally {
      setActing(null);
    }
  }, [cancel, tCommon]);

  if (!loop) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-6">
          <Skeleton className="h-4 w-32" />
        </div>
        <div className="space-y-3 p-6">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-2/3" />
        </div>
      </div>
    );
  }

  const status: LoopStatus = loop.status;
  const canPause = status === "running";
  const canResume = status === "paused";
  const canCancel = status === "running" || status === "paused" || status === "pending";
  const isTerminal = status === "done" || status === "failed" || status === "cancelled";

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b bg-background px-6 text-sm">
        <Button variant="ghost" size="icon-xs" onClick={() => router.push("/loops")}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        <span className="font-mono text-xs text-muted-foreground truncate">{loop.id}</span>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-3xl space-y-6 px-6 py-6">
          {/* Header: status + stage + iteration */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <LoopStatusBadge status={status} />
              <span className="text-xs text-muted-foreground tabular-nums">
                {t("iterationCounter", { current: loop.iteration, max: loop.max_iterations })}
              </span>
            </div>
            <LoopStageIndicator stage={loop.current_stage} />
          </div>

          {/* Failure callout */}
          {loop.failure_reason && (
            <div className="flex items-start gap-2 rounded-md border border-destructive/20 bg-destructive/5 p-3 text-xs text-destructive">
              <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
              <div>
                <p className="font-medium">{t("failureReason")}</p>
                <p className="mt-0.5">{loop.failure_reason}</p>
              </div>
            </div>
          )}

          {/* Info grid */}
          <div className="rounded-md border">
            <InfoRow label={t("info.issue")}>
              <Link href={`/issues/${loop.issue_id}`} className="text-primary hover:underline font-mono text-xs">
                {loop.issue_id}
              </Link>
            </InfoRow>
            {loop.branch_name && (
              <InfoRow label={t("info.branch")}>
                <span className="font-mono text-xs text-muted-foreground">{loop.branch_name}</span>
              </InfoRow>
            )}
            {loop.pr_url && (
              <InfoRow label={t("info.pr")}>
                <a
                  href={loop.pr_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                >
                  {loop.pr_number ? `#${loop.pr_number}` : loop.pr_url}
                  <ExternalLink className="h-3 w-3" />
                </a>
              </InfoRow>
            )}
            {loop.agent_id && (
              <InfoRow label={t("info.agent")}>
                <span className="font-mono text-xs text-muted-foreground">{loop.agent_id}</span>
              </InfoRow>
            )}
            <InfoRow label={t("info.started")}>
              <span className="text-xs text-muted-foreground">{formatDateTime(loop.started_at)}</span>
            </InfoRow>
            {loop.completed_at && (
              <InfoRow label={t("info.completed")}>
                <span className="text-xs text-muted-foreground">{formatDateTime(loop.completed_at)}</span>
              </InfoRow>
            )}
            <InfoRow label={t("info.created")}>
              <span className="text-xs text-muted-foreground">{formatDateTime(loop.created_at)}</span>
            </InfoRow>
          </div>

          {/* Actions */}
          {!isTerminal && (
            <div className="flex items-center gap-2">
              {canPause && (
                <Button variant="outline" size="sm" onClick={onPause} disabled={acting !== null}>
                  {acting === "pause" ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Pause className="h-3.5 w-3.5" />}
                  {t("actions.pause")}
                </Button>
              )}
              {canResume && (
                <Button variant="outline" size="sm" onClick={onResume} disabled={acting !== null}>
                  {acting === "resume" ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
                  {t("actions.resume")}
                </Button>
              )}
              {canCancel && (
                <Button variant="destructive" size="sm" onClick={onCancel} disabled={acting !== null}>
                  {acting === "cancel" ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <X className="h-3.5 w-3.5" />}
                  {t("actions.cancel")}
                </Button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 border-b px-3 py-2 last:border-b-0">
      <span className="w-24 shrink-0 text-xs text-muted-foreground">{label}</span>
      <div className="min-w-0 flex-1 truncate">{children}</div>
    </div>
  );
}
