"use client";

import { useFormatter, useTranslations } from "next-intl";
import { BookOpen, Code, FlaskConical, GitCommit, CheckCircle, Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { AgentStage } from "@/shared/types/events";

interface StageIndicatorProps {
  stage: AgentStage;
  timestamp?: Date;
  className?: string;
}

const stageConfig: Record<AgentStage, { stageKey: string; icon: typeof BookOpen; variant: "default" | "secondary" | "outline" }> = {
  idle: { stageKey: "idle", icon: Loader2, variant: "outline" },
  reading: { stageKey: "reading", icon: BookOpen, variant: "secondary" },
  implementing: { stageKey: "implementing", icon: Code, variant: "secondary" },
  testing: { stageKey: "testing", icon: FlaskConical, variant: "secondary" },
  committing: { stageKey: "committing", icon: GitCommit, variant: "secondary" },
  done: { stageKey: "done", icon: CheckCircle, variant: "outline" },
};

export function StageIndicator({ stage, timestamp, className }: StageIndicatorProps) {
  const config = stageConfig[stage];
  const Icon = config.icon;
  const f = useFormatter();
  const t = useTranslations("issues");

  return (
    <Badge variant={config.variant} className={className}>
      <Icon className="shrink-0" />
      <span>{t(`agent.stage.${config.stageKey}`)}</span>
      {timestamp && (
        <span className="text-muted-foreground ml-1">
          {f.dateTime(timestamp, { hour: "2-digit", minute: "2-digit" })}
        </span>
      )}
    </Badge>
  );
}
