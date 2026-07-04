"use client";

import { useCallback, useMemo } from "react";
import { useTranslations } from "next-intl";
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  Handle,
  Position,
  MarkerType,
} from "@xyflow/react";
import type { Node, Edge } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useTaskGraph } from "../hooks/useTaskGraph";

interface DAGNode {
  id: string;
  node_type: string;
  status: string;
  context: Record<string, any>;
  position_x?: number;
  position_y?: number;
  depth: number;
}

interface DAGEdge {
  id: string;
  from_node_id: string;
  to_node_id: string;
  edge_type: string;
}

function TaskNode({ data }: { data: DAGNode }) {
  const t = useTranslations("taskGraph");
  const typeColors: Record<string, string> = {
    executor: "bg-blue-100 border-blue-300",
    synthesis: "bg-purple-100 border-purple-300",
    planner: "bg-amber-100 border-amber-300",
    root: "bg-green-100 border-green-300",
  };
  const statusColors: Record<string, string> = {
    pending: "bg-gray-100 text-gray-600",
    running: "bg-blue-100 text-blue-700",
    completed: "bg-green-100 text-green-700",
    failed: "bg-red-100 text-red-700",
    blocked: "bg-amber-100 text-amber-700",
  };

  return (
    <div className={`relative px-3 py-2 rounded-lg border-2 min-w-[160px] max-w-[200px] ${typeColors[data.node_type] || "bg-gray-100 border-gray-300"}`}>
      <Handle type="target" position={Position.Top} className="!bg-gray-400 !w-2 !h-2" />

      <div className="flex items-center gap-2 mb-1">
        <span className="text-xs font-medium uppercase">{data.node_type}</span>
        <span className={`text-xs px-1.5 py-0.5 rounded ${statusColors[data.status] || "bg-gray-100 text-gray-600"}`}>
          {data.status}
        </span>
      </div>

      <p className="text-sm font-medium line-clamp-2">
        {data.context?.description || t("graph.noDescription")}
      </p>

      {data.context?.suggested_agent && (
        <p className="text-xs text-gray-500 mt-1">
          {t("graph.agentLabel", { name: data.context.suggested_agent })}
        </p>
      )}

      <Handle type="source" position={Position.Bottom} className="!bg-gray-400 !w-2 !h-2" />
    </div>
  );
}

const nodeTypes = { taskNode: TaskNode };

export function GraphView({ issueId }: { issueId: string }) {
  const t = useTranslations("taskGraph");
  const { nodes: storeNodes, edges: storeEdges, fetchGraph, isLoading, error } = useTaskGraph();

  // Fetch graph when issueId changes
  useMemo(() => {
    if (issueId) fetchGraph(issueId);
  }, [issueId, fetchGraph]);

  const initialNodes: Node[] = useMemo(() =>
    storeNodes.map((n: any) => ({
      id: n.id,
      type: "taskNode",
      position: { x: n.position_x || 0, y: n.position_y || 0 },
      data: n,
    })),
    [storeNodes]
  );

  const initialEdges: Edge[] = useMemo(() =>
    storeEdges.map((e: any) => ({
      id: e.id,
      source: e.from_node_id,
      target: e.to_node_id,
      type: "smoothstep",
      animated: e.edge_type === "depends_on",
      markerEnd: { type: MarkerType.ArrowClosed },
      label: e.edge_type === "depends_on" ? undefined : e.edge_type,
    })),
    [storeEdges]
  );

  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);

  if (isLoading) return <div className="text-muted-foreground text-sm p-4">{t("graph.loading")}</div>;
  if (error) return <div className="text-red-500 text-sm p-4">{t("graph.error", { detail: error })}</div>;
  if (storeNodes.length === 0) return <div className="text-muted-foreground text-sm p-4">{t("graph.empty")}</div>;

  return (
    <div className="h-[400px] border rounded-lg overflow-hidden">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        attributionPosition="bottom-left"
      >
        <Controls />
        <MiniMap />
        <Background />
      </ReactFlow>
    </div>
  );
}