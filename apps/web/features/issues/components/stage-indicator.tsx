"use client";

import { BookOpen, Code, FlaskConical, GitCommit, CheckCircle, Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { AgentStage } from "@/shared/types/events";

interface StageIndicatorProps {
  stage: AgentStage;
  timestamp?: Date;
  className?: string;
}

const stageConfig: Record<AgentStage, { label: string; icon: typeof BookOpen; variant: "default" | "secondary" | "outline" }> = {
  idle: { label: "Idle", icon: Loader2, variant: "outline" },
  reading: { label: "Reading", icon: BookOpen, variant: "secondary" },
  implementing: { label: "Implementing", icon: Code, variant: "secondary" },
  testing: { label: "Testing", icon: FlaskConical, variant: "secondary" },
  committing: { label: "Committing", icon: GitCommit, variant: "secondary" },
  done: { label: "Done", icon: CheckCircle, variant: "outline" },
};

export function StageIndicator({ stage, timestamp, className }: StageIndicatorProps) {
  const config = stageConfig[stage];
  const Icon = config.icon;

  return (
    <Badge variant={config.variant} className={className}>
      <Icon className="shrink-0" />
      <span>{config.label}</span>
      {timestamp && (
        <span className="text-muted-foreground ml-1">
          {timestamp.toLocaleTimeString()}
        </span>
      )}
    </Badge>
  );
}
