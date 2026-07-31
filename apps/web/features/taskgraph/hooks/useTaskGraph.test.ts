import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  taskGraphApi,
  type TaskGraphEdge,
  type TaskGraphNode,
} from "../api/taskGraphApi";
import { useTaskGraph } from "./useTaskGraph";

vi.mock("../api/taskGraphApi", () => ({
  taskGraphApi: {
    getGraph: vi.fn(),
    updateNode: vi.fn(),
    deleteNode: vi.fn(),
    autoDecompose: vi.fn(),
  },
}));

function node(overrides: Partial<TaskGraphNode> = {}): TaskGraphNode {
  return {
    id: "node-1",
    workspace_id: "workspace-1",
    issue_id: "issue-1",
    node_type: "executor",
    status: "pending",
    context: {},
    position_x: 0,
    position_y: 0,
    depth: 0,
    created_at: "2026-07-31T00:00:00Z",
    ...overrides,
  };
}

function edge(overrides: Partial<TaskGraphEdge> = {}): TaskGraphEdge {
  return {
    id: "edge-1",
    from_node_id: "node-1",
    to_node_id: "node-2",
    edge_type: "depends_on",
    ...overrides,
  };
}

describe("task graph store", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useTaskGraph.setState({ nodes: [], edges: [], isLoading: false, error: null });
  });

  it("loads graph data through the authenticated API adapter", async () => {
    vi.mocked(taskGraphApi.getGraph).mockResolvedValue({
      nodes: [node()],
      edges: [edge()],
    });

    await useTaskGraph.getState().fetchGraph("issue-1");

    expect(taskGraphApi.getGraph).toHaveBeenCalledWith("issue-1");
    expect(useTaskGraph.getState()).toMatchObject({
      nodes: [node()],
      edges: [edge()],
      isLoading: false,
      error: null,
    });
  });

  it("uses the canonical server response when updating a node", async () => {
    const updated = node({ status: "running", position_x: 25 });
    useTaskGraph.setState({ nodes: [node()], edges: [], isLoading: false, error: null });
    vi.mocked(taskGraphApi.updateNode).mockResolvedValue(updated);

    await useTaskGraph.getState().updateNode("node-1", { status: "running" });

    expect(useTaskGraph.getState().nodes).toEqual([updated]);
  });

  it("removes edges connected to a deleted node", async () => {
    useTaskGraph.setState({
      nodes: [node(), node({ id: "node-2" })],
      edges: [edge()],
      isLoading: false,
      error: null,
    });
    vi.mocked(taskGraphApi.deleteNode).mockResolvedValue();

    await useTaskGraph.getState().deleteNode("node-1");

    expect(useTaskGraph.getState().nodes.map((item) => item.id)).toEqual(["node-2"]);
    expect(useTaskGraph.getState().edges).toEqual([]);
  });

  it("stores API errors and clears loading state", async () => {
    vi.mocked(taskGraphApi.getGraph).mockRejectedValue(new Error("request failed"));

    await useTaskGraph.getState().fetchGraph("issue-1");

    expect(useTaskGraph.getState()).toMatchObject({
      nodes: [],
      edges: [],
      isLoading: false,
      error: "request failed",
    });
  });
});
