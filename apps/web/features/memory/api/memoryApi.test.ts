import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@/shared/api";
import { memoryApi, type MemoryEntry } from "./memoryApi";

vi.mock("@/shared/api", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

const memory: MemoryEntry = {
  id: "memory-1",
  workspace_id: "workspace-1",
  memory_type: "learning",
  content: "A useful convention",
  created_at: "2026-07-31T00:00:00Z",
};

describe("memory API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates team memories on the workspace route", async () => {
    vi.mocked(api.post).mockResolvedValue(memory);

    await memoryApi.storeMemory({
      workspace_id: "workspace-1",
      memory_type: "learning",
      content: "A useful convention",
    });

    expect(api.post).toHaveBeenCalledWith("/api/workspaces/workspace-1/memories", {
      memory_type: "learning",
      content: "A useful convention",
    });
  });

  it("creates private agent memories on the agent route", async () => {
    vi.mocked(api.post).mockResolvedValue({ ...memory, agent_id: "agent-1" });

    await memoryApi.storeMemory({
      workspace_id: "workspace-1",
      agent_id: "agent-1",
      memory_type: "context",
      content: "Agent context",
      is_private: true,
    });

    expect(api.post).toHaveBeenCalledWith("/api/agents/agent-1/memories", {
      memory_type: "context",
      content: "Agent context",
      is_private: true,
    });
  });

  it("deletes memories through their canonical scope", async () => {
    vi.mocked(api.delete).mockResolvedValue();

    await memoryApi.deleteMemory(memory);
    await memoryApi.deleteMemory({ ...memory, agent_id: "agent-1" });

    expect(api.delete).toHaveBeenNthCalledWith(1, "/api/workspaces/workspace-1/memories/memory-1");
    expect(api.delete).toHaveBeenNthCalledWith(2, "/api/agents/agent-1/memories/memory-1");
  });

  it("searches through the workspace-scoped route", async () => {
    vi.mocked(api.get).mockResolvedValue({ memories: [memory] });

    await memoryApi.searchMemories("workspace-1", "error handling");

    expect(api.get).toHaveBeenCalledWith(
      "/api/workspaces/workspace-1/memories/search?q=error+handling",
    );
  });
});
