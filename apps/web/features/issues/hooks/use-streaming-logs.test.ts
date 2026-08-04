import { describe, expect, it } from "vitest";
import type { TaskMessagePayload } from "@/shared/types/events";
import { mergeLogEntries, taskMessagesToLogEntries } from "./use-streaming-logs";

function message(seq: number, content?: string, output?: string, runId = "run-1"): TaskMessagePayload {
  return { task_id: "task-1", run_id: runId, issue_id: "issue-1", seq, type: "text", content, output };
}

describe("streaming log snapshots", () => {
  it("deduplicates replayed cursors and preserves content/output order", () => {
    const initial = taskMessagesToLogEntries([message(2, "second"), message(1, "first", "result")]);
    const replay = taskMessagesToLogEntries([message(2, "second"), message(3, "third")]);

    expect(mergeLogEntries(initial, replay).map((entry) => entry.text)).toEqual([
      "first",
      "result",
      "second",
      "third",
    ]);
  });

  it("keeps only the newest bounded snapshot", () => {
    const entries = taskMessagesToLogEntries([
      message(1, "one"),
      message(2, "two"),
      message(3, "three"),
    ]);

    expect(mergeLogEntries([], entries, 2).map((entry) => entry.text)).toEqual(["two", "three"]);
  });

  it("does not collapse the same cursor from different runs", () => {
    const entries = taskMessagesToLogEntries([
      message(1, "first attempt", undefined, "run-1"),
      message(1, "second attempt", undefined, "run-2"),
    ]);

    expect(mergeLogEntries([], entries).map((entry) => entry.text)).toEqual([
      "first attempt",
      "second attempt",
    ]);
  });
});
