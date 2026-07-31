"use client";

import { useEffect, useMemo } from "react";
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
import type { Edge, Node, NodeProps, NodeTypes } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { TaskGraphNode } from "../api/taskGraphApi";
import { useTaskGraph } from "../hooks/useTaskGraph";

type TaskGraphFlowNode = Node<{ node: TaskGraphNode }, "taskNode">;

function TaskNode({ data }: NodeProps<TaskGraphFlowNode>) {
  const t = useTranslations("taskGraph");
  const node = data.node;
  const typeColors: Record<string, string> = {
    executor: "bg-primary/10 border-primary/30",
    synthesis: "bg-accent border-border",
    planner: "bg-secondary border-border",
    root: "bg-muted border-border",
  };
  const statusColors: Record<string, string> = {
    pending: "bg-muted text-muted-foreground",
    running: "bg-primary/10 text-primary",
    completed: "bg-accent text-accent-foreground",
    failed: "bg-destructive/10 text-destructive",
    blocked: "bg-secondary text-secondary-foreground",
  };

  return (
    <div className={`relative px-3 py-2 rounded-lg border-2 min-w-[160px] max-w-[200px] ${typeColors[node.node_type] || "bg-muted border-border"}`}>
      <Handle type="target" position={Position.Top} className="!bg-muted-foreground !w-2 !h-2" />

      <div className="flex items-center gap-2 mb-1">
        <span className="text-xs font-medium uppercase">{node.node_type}</span>
        <span className={`text-xs px-1.5 py-0.5 rounded ${statusColors[node.status] || "bg-muted text-muted-foreground"}`}>
          {node.status}
        </span>
      </div>

      <p className="text-sm font-medium line-clamp-2">
        {node.context?.description || t("graph.noDescription")}
      </p>

      {node.context?.suggested_agent && (
        <p className="text-xs text-muted-foreground mt-1">
          {t("graph.agentLabel", { name: node.context.suggested_agent })}
        </p>
      )}

      <Handle type="source" position={Position.Bottom} className="!bg-muted-foreground !w-2 !h-2" />
    </div>
  );
}

const nodeTypes = { taskNode: TaskNode } satisfies NodeTypes;

export function GraphView({ issueId }: { issueId: string }) {
  const t = useTranslations("taskGraph");
  const { nodes: storeNodes, edges: storeEdges, fetchGraph, isLoading, error } = useTaskGraph();

  // Fetch graph when issueId changes
  useEffect(() => {
    if (issueId) fetchGraph(issueId);
  }, [issueId, fetchGraph]);

  const graphNodes: TaskGraphFlowNode[] = useMemo(() =>
    storeNodes.map((node) => ({
      id: node.id,
      type: "taskNode",
      position: { x: node.position_x || 0, y: node.position_y || 0 },
      data: { node },
    })),
    [storeNodes]
  );

  const graphEdges: Edge[] = useMemo(() =>
    storeEdges.map((edge) => ({
      id: edge.id,
      source: edge.from_node_id,
      target: edge.to_node_id,
      type: "smoothstep",
      animated: edge.edge_type === "depends_on",
      markerEnd: { type: MarkerType.ArrowClosed },
      label: edge.edge_type === "depends_on" ? undefined : edge.edge_type,
    })),
    [storeEdges]
  );

  const [nodes, setNodes, onNodesChange] = useNodesState<TaskGraphFlowNode>(graphNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(graphEdges);

  useEffect(() => {
    setNodes(graphNodes);
  }, [graphNodes, setNodes]);

  useEffect(() => {
    setEdges(graphEdges);
  }, [graphEdges, setEdges]);

  if (isLoading) return <div className="text-muted-foreground text-sm p-4">{t("graph.loading")}</div>;
  if (error) return <div className="text-destructive text-sm p-4">{t("graph.error", { detail: error })}</div>;
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
