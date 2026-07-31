"use client";

import { useTranslations } from "next-intl";
import type { TaskGraphNode } from "../api/taskGraphApi";

const TYPE_COLORS: Record<string, string> = {
  root: "bg-muted text-muted-foreground",
  planner: "bg-secondary text-secondary-foreground",
  executor: "bg-primary/10 text-primary",
  synthesis: "bg-accent text-accent-foreground",
};

const STATUS_COLORS: Record<string, string> = {
  pending: "text-muted-foreground",
  running: "text-primary",
  completed: "text-accent-foreground",
  failed: "text-destructive",
  blocked: "text-secondary-foreground",
};

export function NodeCard({ node }: { node: TaskGraphNode }) {
  const t = useTranslations("taskGraph");
  return (
    <div className="border rounded-lg p-3">
      <div className="flex items-center gap-2 mb-1">
        <span className={`text-xs px-2 py-0.5 rounded ${TYPE_COLORS[node.node_type] ?? "bg-muted text-muted-foreground"}`}>
          {node.node_type}
        </span>
        <span className={`text-xs font-medium ${STATUS_COLORS[node.status] ?? "text-muted-foreground"}`}>
          {node.status}
        </span>
      </div>
      {node.agent_id && (
        <div className="text-xs text-muted-foreground">
          {t("graph.agentLabel", { name: node.agent_id })}
        </div>
      )}
    </div>
  );
}
