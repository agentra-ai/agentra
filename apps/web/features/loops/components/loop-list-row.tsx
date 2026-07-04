"use client";

import Link from "next/link";
import { useFormatter, useTranslations } from "next-intl";
import { ExternalLink } from "lucide-react";
import { TableCell, TableRow } from "@/components/ui/table";
import { LoopStatusBadge } from "./loop-status-badge";
import { LoopStageIndicator } from "./loop-stage-indicator";
import type { Loop } from "@/shared/types/loop";

function truncateId(id: string, len = 8): string {
  return id.length > len ? id.slice(0, len) : id;
}

export function LoopListRow({ loop }: { loop: Loop }) {
  const t = useTranslations("loops");
  const f = useFormatter();
  return (
    <TableRow>
      <TableCell className="font-mono text-xs">
        <Link href={`/loops/${loop.id}`} className="hover:underline">
          {truncateId(loop.id)}
        </Link>
      </TableCell>
      <TableCell>
        <Link href={`/issues/${loop.issue_id}`} className="text-muted-foreground hover:underline font-mono text-xs">
          {truncateId(loop.issue_id)}
        </Link>
      </TableCell>
      <TableCell>
        <LoopStatusBadge status={loop.status} />
      </TableCell>
      <TableCell>
        <LoopStageIndicator stage={loop.current_stage} />
      </TableCell>
      <TableCell className="text-muted-foreground tabular-nums text-xs">
        {loop.iteration}/{loop.max_iterations}
      </TableCell>
      <TableCell>
        {loop.pr_url ? (
          <a
            href={loop.pr_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
          >
            {loop.pr_number ? `#${loop.pr_number}` : t("pr")}
            <ExternalLink className="h-3 w-3" />
          </a>
        ) : (
          <span className="text-muted-foreground/50">—</span>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground text-xs">
        {f.dateTime(new Date(loop.created_at), { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
      </TableCell>
    </TableRow>
  );
}
