"use client";

import { Badge } from "@/components/ui/badge";
import { useTranslations } from "next-intl";
import type { LoopStatus } from "@/shared/types/loop";

const STATUS_VARIANT: Record<LoopStatus, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "outline",
  running: "default",
  paused: "secondary",
  done: "secondary",
  failed: "destructive",
  cancelled: "outline",
};

export function LoopStatusBadge({ status, className }: { status: LoopStatus; className?: string }) {
  const t = useTranslations("loops");
  return (
    <Badge variant={STATUS_VARIANT[status]} className={className}>
      {t(`status.${status}`)}
    </Badge>
  );
}
