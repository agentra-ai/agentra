import { describe, it, expect, beforeEach } from "vitest";
import { useLoopStore } from "./store";
import type { Loop } from "@/shared/types/loop";

function makeLoop(overrides: Partial<Loop> = {}): Loop {
  return {
    id: "loop-1",
    issue_id: "issue-1",
    workspace_id: "ws-1",
    status: "pending",
    iteration: 0,
    max_iterations: 5,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("loop store", () => {
  beforeEach(() => {
    useLoopStore.setState({ loops: {}, loopIds: [], loading: false, error: null });
  });

  it("upsertLoop adds a new loop and tracks its id", () => {
    const loop = makeLoop({ id: "a" });
    useLoopStore.getState().upsertLoop(loop);
    const s = useLoopStore.getState();
    expect(s.loops["a"]).toEqual(loop);
    expect(s.loopIds).toEqual(["a"]);
  });

  it("upsertLoop updates an existing loop without changing order", () => {
    useLoopStore.getState().upsertLoop(makeLoop({ id: "a", status: "pending" }));
    useLoopStore.getState().upsertLoop(makeLoop({ id: "b", status: "pending" }));
    useLoopStore.getState().upsertLoop(makeLoop({ id: "a", status: "running" }));

    const s = useLoopStore.getState();
    expect(s.loopIds).toEqual(["a", "b"]);
    expect(s.loops["a"]?.status).toBe("running");
  });

  it("setLoops replaces state and resets order", () => {
    useLoopStore.getState().upsertLoop(makeLoop({ id: "old" }));
    useLoopStore.getState().setLoops([makeLoop({ id: "x" }), makeLoop({ id: "y" })]);

    const s = useLoopStore.getState();
    expect(s.loops["old"]).toBeUndefined();
    expect(s.loopIds).toEqual(["x", "y"]);
    expect(s.loops["x"]).toBeDefined();
    expect(s.loops["y"]).toBeDefined();
  });

  it("removeLoop drops the entry and its id", () => {
    useLoopStore.getState().upsertLoop(makeLoop({ id: "a" }));
    useLoopStore.getState().upsertLoop(makeLoop({ id: "b" }));
    useLoopStore.getState().removeLoop("a");
    const s = useLoopStore.getState();
    expect(s.loops["a"]).toBeUndefined();
    expect(s.loopIds).toEqual(["b"]);
  });

  it("setLoading and setError update the flags", () => {
    useLoopStore.getState().setLoading(true);
    expect(useLoopStore.getState().loading).toBe(true);
    useLoopStore.getState().setError("boom");
    expect(useLoopStore.getState().error).toBe("boom");
  });
});
