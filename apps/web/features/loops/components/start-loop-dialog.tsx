"use client";

import { useState, useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import { Play, Loader2 } from "lucide-react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { useWorkspaceStore } from "@/features/workspace";
import { useStartLoop } from "../hooks";
import type { Loop } from "@/shared/types/loop";

interface StartLoopDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  issueId: string;
  onSuccess?: (loop: Loop) => void;
}

export function StartLoopDialog({ open, onOpenChange, issueId, onSuccess }: StartLoopDialogProps) {
  const t = useTranslations("loops");
  const tCommon = useTranslations("common");
  const agents = useWorkspaceStore((s) => s.agents);
  const activeAgents = useMemo(() => agents.filter((a) => !a.archived_at), [agents]);
  const [agentId, setAgentId] = useState<string>("");
  const [maxIterations, setMaxIterations] = useState<string>("5");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const startLoop = useStartLoop();

  useEffect(() => {
    if (open) {
      setAgentId(activeAgents[0]?.id ?? "");
      setMaxIterations("5");
      setError(null);
    }
  }, [open, activeAgents]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!agentId) {
      setError(t("dialog.agentRequired"));
      return;
    }
    const parsed = parseInt(maxIterations, 10);
    if (Number.isNaN(parsed) || parsed < 1 || parsed > 10) {
      setError(t("dialog.invalidIterations"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const loop = await startLoop({ issue_id: issueId, agent_id: agentId, max_iterations: parsed });
      onOpenChange(false);
      onSuccess?.(loop);
    } catch (e) {
      const message = e instanceof Error ? e.message : tCommon("error");
      setError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("dialog.title")}</DialogTitle>
            <DialogDescription>{t("dialog.description")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="loop-agent">{t("dialog.agent")}</Label>
              <Select value={agentId} onValueChange={(v) => setAgentId(v ?? "")}>
                <SelectTrigger id="loop-agent" className="w-full">
                  <SelectValue placeholder={t("dialog.agentPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {activeAgents.map((a) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="loop-iterations">{t("dialog.maxIterations")}</Label>
              <Input
                id="loop-iterations"
                type="number"
                min={1}
                max={10}
                value={maxIterations}
                onChange={(e) => setMaxIterations(e.target.value)}
              />
            </div>

            {error && <p className="text-xs text-destructive">{error}</p>}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              {tCommon("cancel")}
            </Button>
            <Button type="submit" disabled={submitting || !agentId}>
              {submitting ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Play className="h-3.5 w-3.5" />
              )}
              {t("dialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
