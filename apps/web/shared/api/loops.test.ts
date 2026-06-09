import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Loop } from "@/shared/types/loop";

vi.mock("@/shared/api", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

import { api } from "@/shared/api";
import { listLoops, getLoop, startLoop, pauseLoop, resumeLoop, cancelLoop } from "./loops";

const mockGet = api.get as ReturnType<typeof vi.fn>;
const mockPost = api.post as ReturnType<typeof vi.fn>;

const sampleLoop: Loop = {
  id: "loop-1",
  issue_id: "issue-1",
  workspace_id: "ws-1",
  status: "running",
  iteration: 1,
  max_iterations: 5,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("listLoops", () => {
  it("calls GET /api/loops and unwraps the loops array", async () => {
    mockGet.mockResolvedValueOnce({ loops: [sampleLoop] });
    const result = await listLoops();
    expect(mockGet).toHaveBeenCalledWith("/api/loops");
    expect(result).toEqual([sampleLoop]);
  });
});

describe("getLoop", () => {
  it("calls GET /api/loops/:id and returns the loop", async () => {
    mockGet.mockResolvedValueOnce(sampleLoop);
    const result = await getLoop("loop-1");
    expect(mockGet).toHaveBeenCalledWith("/api/loops/loop-1");
    expect(result).toEqual(sampleLoop);
  });
});

describe("startLoop", () => {
  it("POSTs the start payload to /api/loops", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    const result = await startLoop({ issue_id: "issue-1", agent_id: "agent-1", max_iterations: 5 });
    expect(mockPost).toHaveBeenCalledWith("/api/loops", { issue_id: "issue-1", agent_id: "agent-1", max_iterations: 5 });
    expect(result).toEqual(sampleLoop);
  });

  it("works without max_iterations", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    await startLoop({ issue_id: "issue-1", agent_id: "agent-1" });
    expect(mockPost).toHaveBeenCalledWith("/api/loops", { issue_id: "issue-1", agent_id: "agent-1" });
  });
});

describe("pauseLoop", () => {
  it("POSTs to /api/loops/:id/pause", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    const result = await pauseLoop("loop-1");
    expect(mockPost).toHaveBeenCalledWith("/api/loops/loop-1/pause");
    expect(result).toEqual(sampleLoop);
  });
});

describe("resumeLoop", () => {
  it("POSTs to /api/loops/:id/resume", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    const result = await resumeLoop("loop-1");
    expect(mockPost).toHaveBeenCalledWith("/api/loops/loop-1/resume");
    expect(result).toEqual(sampleLoop);
  });
});

describe("cancelLoop", () => {
  it("POSTs to /api/loops/:id/cancel", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    const result = await cancelLoop("loop-1");
    expect(mockPost).toHaveBeenCalledWith("/api/loops/loop-1/cancel");
    expect(result).toEqual(sampleLoop);
  });
});
