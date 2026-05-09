"use client";

import { useTaskGraph } from "../hooks/useTaskGraph";

export function GraphView({ issueId }: { issueId: string }) {
  const { nodes, edges, isLoading } = useTaskGraph();

  if (isLoading) return <div className="text-muted-foreground text-sm">Loading DAG view...</div>;
  if (nodes.length === 0) return <div className="text-muted-foreground text-sm">No graph data.</div>;

  return (
    <div className="border rounded-lg p-4 min-h-[300px] flex items-center justify-center">
      <div className="text-center">
        <p className="text-sm font-medium">DAG Visualization</p>
        <p className="text-xs text-muted-foreground">
          {nodes.length} nodes, {edges.length} edges
        </p>
      </div>
    </div>
  );
}
