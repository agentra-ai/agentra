"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { useTaskGraph } from "../hooks/useTaskGraph";
import { NodeCard } from "./NodeCard";

export function SubtaskTree({ issueId }: { issueId: string }) {
  const t = useTranslations("taskGraph");
  const { nodes, edges, isLoading, error, fetchGraph } = useTaskGraph();

  useEffect(() => {
    if (issueId) fetchGraph(issueId);
  }, [issueId, fetchGraph]);

  if (isLoading) return <div className="text-muted-foreground text-sm">{t("tree.loading")}</div>;
  if (error) return <div className="text-destructive text-sm">{t("tree.error", { detail: error })}</div>;
  if (nodes.length === 0) return <div className="text-muted-foreground text-sm">{t("tree.empty")}</div>;

  // Render nodes ordered by depth
  const sorted = [...nodes].sort((a, b) => a.depth - b.depth);

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-semibold mb-2">{t("tree.title")}</h3>
      {sorted.map((node) => (
        <div key={node.id} style={{ marginLeft: `${node.depth * 20}px` }}>
          <NodeCard node={node} />
        </div>
      ))}
    </div>
  );
}
