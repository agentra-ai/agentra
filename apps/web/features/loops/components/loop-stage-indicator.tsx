"use client";

import { Check } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { LoopStage } from "@/shared/types/loop";

const STAGE_ORDER: LoopStage[] = ["plan", "develop", "review", "fix"];

export function LoopStageIndicator({
  stage,
  className,
}: {
  stage?: LoopStage;
  className?: string;
}) {
  const t = useTranslations("loops");
  const activeIdx = stage ? STAGE_ORDER.indexOf(stage) : -1;

  return (
    <div className={cn("flex items-center gap-1.5", className)}>
      {STAGE_ORDER.map((s, idx) => {
        const isCurrent = idx === activeIdx;
        const isDone = activeIdx >= 0 && idx < activeIdx;
        return (
          <div key={s} className="flex items-center gap-1.5">
            <div
              className={cn(
                "flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-medium",
                isCurrent && "bg-primary text-primary-foreground",
                isDone && "bg-primary/20 text-primary",
                !isCurrent && !isDone && "bg-muted text-muted-foreground",
              )}
            >
              {isDone ? <Check className="h-3 w-3" /> : idx + 1}
            </div>
            <span
              className={cn(
                "text-xs",
                isCurrent ? "text-foreground font-medium" : "text-muted-foreground",
              )}
            >
              {t(`stage.${s}`)}
            </span>
            {idx < STAGE_ORDER.length - 1 && (
              <span className="mx-1 h-px w-3 bg-border" aria-hidden />
            )}
          </div>
        );
      })}
    </div>
  );
}
