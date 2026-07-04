"use client";

import { useTranslations } from "next-intl";

const TYPE_COLORS: Record<string, string> = {
  root: "bg-gray-100 text-gray-800",
  planner: "bg-blue-100 text-blue-800",
  executor: "bg-green-100 text-green-800",
  synthesis: "bg-yellow-100 text-yellow-800",
};

const STATUS_COLORS: Record<string, string> = {
  pending: "text-muted-foreground",
  running: "text-blue-600",
  completed: "text-green-600",
  failed: "text-destructive",
  blocked: "text-yellow-600",
};

export function NodeCard({ node }: { node: any }) {
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
