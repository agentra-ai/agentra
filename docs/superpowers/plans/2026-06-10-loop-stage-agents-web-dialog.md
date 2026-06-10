# Loop Stage Agents Web Dialog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `StartLoopDialog` 暴露 per-stage agent override,让用户能为 plan/develop/review/fix 4 个 stage 各自选 agent,Go 端已经接受 `stage_agents` map,本 plan 只动 web 端。

**Architecture:** 三处串联改动 — type 加字段,hook 透传,dialog 加 4 个 picker + 提交过滤。提交时仅在 ≥1 个非空时把 `stage_agents` 放进 body,否则省略。Empty value = use default。

**Tech Stack:** TypeScript, Next.js 16 App Router, shadcn `Select`, `next-intl`, Vitest.

---

## File Map

- **Modify** `apps/web/shared/types/loop.ts` — `StartLoopRequest` 加 `stage_agents?`
- **Modify** `apps/web/features/loops/hooks.ts` — `useStartLoop` 用 `StartLoopRequest` 代替内联类型
- **Modify** `apps/web/shared/api/loops.test.ts` — `startLoop` 加 `stage_agents` 透传 case
- **Modify** `apps/web/features/loops/components/start-loop-dialog.tsx` — 4 个 stage picker + 提交逻辑
- **Modify** `apps/web/messages/en.json` — `loops.dialog.perStageOverrides` + `stageOverridePlaceholder`
- **Modify** `apps/web/messages/zh-CN.json` — 同上,中文

无新增文件。

---

## Task 1: 类型加 `stage_agents?`

**Files:**
- Modify: `apps/web/shared/types/loop.ts:23-27`

- [ ] **Step 1: 修改 `StartLoopRequest`**

把
```ts
export interface StartLoopRequest {
  issue_id: string;
  agent_id: string;
  max_iterations?: number;
}
```
改为
```ts
export interface StartLoopRequest {
  issue_id: string;
  agent_id: string;
  max_iterations?: number;
  stage_agents?: Partial<Record<LoopStage, string>>;
}
```

`LoopStage` 已在同文件里 (`export type LoopStage = "plan" | "develop" | "review" | "fix"`),直接复用。

- [ ] **Step 2: 跑 typecheck**

Run: `cd /Users/doug/ai/system/agentra && pnpm typecheck`
Expected: PASS,无 error。

- [ ] **Step 3: Commit**

```bash
git add apps/web/shared/types/loop.ts
git commit -m "feat(loops): add stage_agents to StartLoopRequest"
```

---

## Task 2: `useStartLoop` 用 `StartLoopRequest`

**Files:**
- Modify: `apps/web/features/loops/hooks.ts:80-92`

- [ ] **Step 1: 替换内联 input 类型**

把 `apps/web/features/loops/hooks.ts` 的 `useStartLoop` 整个函数替换为:

```ts
export function useStartLoop() {
  const upsertLoop = useLoopStore((s) => s.upsertLoop);
  const setLoops = useLoopStore((s) => s.setLoops);

  return useCallback(async (input: StartLoopRequest): Promise<Loop> => {
    const loop = await loopsApi.startLoop(input);
    upsertLoop(loop);
    loopsApi.listLoops()
      .then(setLoops)
      .catch((err) => console.error("failed to refresh loops after start", err));
    return loop;
  }, [upsertLoop, setLoops]);
}
```

把文件顶部的 import 区域加:
```ts
import type { Loop, StartLoopRequest } from "@/shared/types/loop";
```
替换现有的
```ts
import type { Loop } from "@/shared/types/loop";
```

- [ ] **Step 2: 跑 typecheck**

Run: `pnpm typecheck`
Expected: PASS。

- [ ] **Step 3: Commit**

```bash
git add apps/web/features/loops/hooks.ts
git commit -m "refactor(loops): useStartLoop adopts StartLoopRequest type"
```

---

## Task 3: `loops.test.ts` 加 `stage_agents` 透传 case

**Files:**
- Modify: `apps/web/shared/api/loops.test.ts:50-63`

- [ ] **Step 1: 加新的 test case**

在 `describe("startLoop", ...)` 块末尾加:

```ts
  it("forwards stage_agents in the POST body when provided", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    await startLoop({
      issue_id: "issue-1",
      agent_id: "agent-1",
      stage_agents: { develop: "agent-2", review: "agent-3" },
    });
    expect(mockPost).toHaveBeenCalledWith("/api/loops", {
      issue_id: "issue-1",
      agent_id: "agent-1",
      stage_agents: { develop: "agent-2", review: "agent-3" },
    });
  });
```

完整的 `describe("startLoop", ...)` 块长这样:

```ts
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

  it("forwards stage_agents in the POST body when provided", async () => {
    mockPost.mockResolvedValueOnce(sampleLoop);
    await startLoop({
      issue_id: "issue-1",
      agent_id: "agent-1",
      stage_agents: { develop: "agent-2", review: "agent-3" },
    });
    expect(mockPost).toHaveBeenCalledWith("/api/loops", {
      issue_id: "issue-1",
      agent_id: "agent-1",
      stage_agents: { develop: "agent-2", review: "agent-3" },
    });
  });
});
```

- [ ] **Step 2: 跑测试**

Run: `pnpm --filter @agentra/web exec vitest run src/shared/api/loops.test.ts`
Expected: 3 tests pass in `startLoop` describe 块 (现有 2 个 + 新 1 个)。

- [ ] **Step 3: Commit**

```bash
git add apps/web/shared/api/loops.test.ts
git commit -m "test(loops): cover stage_agents passthrough in startLoop"
```

---

## Task 4: i18n 加 `perStageOverrides` + `stageOverridePlaceholder`

**Files:**
- Modify: `apps/web/messages/en.json:501-510`
- Modify: `apps/web/messages/zh-CN.json:501-510`

- [ ] **Step 1: en.json 加 keys**

把
```json
    "dialog": {
      "title": "Start Agentic Loop",
      "description": "Choose an agent and how many iterations to allow.",
      "agent": "Agent",
      "agentPlaceholder": "Select an agent",
      "agentRequired": "Please select an agent",
      "maxIterations": "Max iterations",
      "invalidIterations": "Max iterations must be between 1 and 10",
      "submit": "Start Loop"
    }
```
替换为
```json
    "dialog": {
      "title": "Start Agentic Loop",
      "description": "Choose an agent and how many iterations to allow.",
      "agent": "Agent",
      "agentPlaceholder": "Select an agent",
      "agentRequired": "Please select an agent",
      "maxIterations": "Max iterations",
      "invalidIterations": "Max iterations must be between 1 and 10",
      "submit": "Start Loop",
      "perStageOverrides": "Per-stage overrides",
      "stageOverridePlaceholder": "Use default ({agent})"
    }
```

- [ ] **Step 2: zh-CN.json 加 keys**

把
```json
    "dialog": {
      "title": "启动工程循环",
      "description": "选择一个代理并设置允许的迭代次数。",
      "agent": "代理",
      "agentPlaceholder": "选择代理",
      "agentRequired": "请选择代理",
      "maxIterations": "最大迭代次数",
      "invalidIterations": "最大迭代次数应在 1 到 10 之间",
      "submit": "启动循环"
    }
```
替换为
```json
    "dialog": {
      "title": "启动工程循环",
      "description": "选择一个代理并设置允许的迭代次数。",
      "agent": "代理",
      "agentPlaceholder": "选择代理",
      "agentRequired": "请选择代理",
      "maxIterations": "最大迭代次数",
      "invalidIterations": "最大迭代次数应在 1 到 10 之间",
      "submit": "启动循环",
      "perStageOverrides": "按阶段覆盖",
      "stageOverridePlaceholder": "使用默认 ({agent})"
    }
```

- [ ] **Step 3: Commit**

```bash
git add apps/web/messages/en.json apps/web/messages/zh-CN.json
git commit -m "feat(loops): i18n for per-stage override section"
```

---

## Task 5: `StartLoopDialog` 加 4 个 stage picker

**Files:**
- Modify: `apps/web/features/loops/components/start-loop-dialog.tsx`

- [ ] **Step 1: 替换整个文件**

用以下完整内容替换 `apps/web/features/loops/components/start-loop-dialog.tsx`:

```tsx
"use client";

import { useState, useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import { Play, Loader2 } from "lucide-react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { useWorkspaceStore } from "@/features/workspace";
import { useStartLoop } from "../hooks";
import type { Loop, LoopStage, StartLoopRequest } from "@/shared/types/loop";

interface StartLoopDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  issueId: string;
  onSuccess?: (loop: Loop) => void;
}

type StageOverrides = Partial<Record<LoopStage, string>>;

const STAGE_ORDER: LoopStage[] = ["plan", "develop", "review", "fix"];

export function StartLoopDialog({ open, onOpenChange, issueId, onSuccess }: StartLoopDialogProps) {
  const t = useTranslations("loops");
  const tStage = useTranslations("loops.stage");
  const tCommon = useTranslations("common");
  const agents = useWorkspaceStore((s) => s.agents);
  const activeAgents = useMemo(() => agents.filter((a) => !a.archived_at), [agents]);
  const [agentId, setAgentId] = useState<string>("");
  const [stageOverrides, setStageOverrides] = useState<StageOverrides>({});
  const [maxIterations, setMaxIterations] = useState<string>("5");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const startLoop = useStartLoop();

  useEffect(() => {
    if (open) {
      setAgentId(activeAgents[0]?.id ?? "");
      setStageOverrides({});
      setMaxIterations("5");
      setError(null);
    }
  }, [open, activeAgents]);

  const defaultAgentName = useMemo(
    () => activeAgents.find((a) => a.id === agentId)?.name ?? "",
    [activeAgents, agentId],
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!agentId) {
      setError(t("dialog.agentRequired"));
      return;
    }
    const parsed = parseInt(maxIterations, 10);
    if (Number.isNaN(parsed) || parsed < 1 || parsed > 10) {
      setError(t("dialog.invalidIterations"));
      return;
    }
    setSubmitting(true);
    setError(null);

    const cleaned: StageOverrides = {};
    for (const [k, v] of Object.entries(stageOverrides)) {
      if (v) cleaned[k as LoopStage] = v;
    }

    const body: StartLoopRequest = {
      issue_id: issueId,
      agent_id: agentId,
      max_iterations: parsed,
    };
    if (Object.keys(cleaned).length > 0) {
      body.stage_agents = cleaned;
    }

    try {
      const loop = await startLoop(body);
      onOpenChange(false);
      onSuccess?.(loop);
    } catch (e) {
      const message = e instanceof Error ? e.message : tCommon("error");
      setError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("dialog.title")}</DialogTitle>
            <DialogDescription>{t("dialog.description")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="loop-agent">{t("dialog.agent")}</Label>
              <Select value={agentId} onValueChange={(v) => setAgentId(v ?? "")}>
                <SelectTrigger id="loop-agent" className="w-full">
                  <SelectValue placeholder={t("dialog.agentPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {activeAgents.map((a) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>{t("dialog.perStageOverrides")}</Label>
              {STAGE_ORDER.map((stage) => (
                <div key={stage} className="space-y-1">
                  <Label htmlFor={`loop-stage-${stage}`} className="text-xs text-muted-foreground">
                    {tStage(stage)}
                  </Label>
                  <Select
                    value={stageOverrides[stage] ?? ""}
                    onValueChange={(v) =>
                      setStageOverrides((prev) => {
                        const next = { ...prev };
                        if (v) next[stage] = v;
                        else delete next[stage];
                        return next;
                      })
                    }
                  >
                    <SelectTrigger id={`loop-stage-${stage}`} className="w-full">
                      <SelectValue
                        placeholder={t("dialog.stageOverridePlaceholder", {
                          agent: defaultAgentName || t("dialog.agent"),
                        })}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {activeAgents.map((a) => (
                        <SelectItem key={a.id} value={a.id}>
                          {a.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              ))}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="loop-iterations">{t("dialog.maxIterations")}</Label>
              <Input
                id="loop-iterations"
                type="number"
                min={1}
                max={10}
                value={maxIterations}
                onChange={(e) => setMaxIterations(e.target.value)}
              />
            </div>

            {error && <p className="text-xs text-destructive">{error}</p>}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              {tCommon("cancel")}
            </Button>
            <Button type="submit" disabled={submitting || !agentId}>
              {submitting ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Play className="h-3.5 w-3.5" />
              )}
              {t("dialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: 跑 typecheck**

Run: `pnpm typecheck`
Expected: PASS。

- [ ] **Step 3: 跑全量 web 测试**

Run: `pnpm test`
Expected: 之前所有 pass + 新增 1 个 `stage_agents` case pass。

- [ ] **Step 4: Commit**

```bash
git add apps/web/features/loops/components/start-loop-dialog.tsx
git commit -m "feat(loops): per-stage agent overrides in StartLoopDialog"
```

---

## Self-Review

**Spec coverage:**
- §3 Type 改动 — Task 1 ✓
- §4 Hook 改动 — Task 2 ✓
- §5 Dialog 改动 (5.1 状态 / 5.2 布局 / 5.3 提交 / 5.4 校验) — Task 5 ✓
- §6 i18n — Task 4 ✓
- §7 测试 (单元 + 手动 checklist) — Task 3 (单元) + Task 5 step 3 (全量) ✓;手动 checklist 留给执行者

**Placeholders:** 无 TBD / TODO / "类似 Task N"。

**Type 一致性:**
- `StartLoopRequest.stage_agents` 在 Task 1 定义为 `Partial<Record<LoopStage, string>>`
- `StageOverrides = Partial<Record<LoopStage, string>>` 在 Task 5 直接复用同一形状
- `loopsApi.startLoop(input)` 接受 `StartLoopRequest` 整个 body,Task 5 提交时构造的 body 满足
- `LoopStage` 在 `shared/types/loop.ts` 已经存在
- i18n key 在 Task 4 加上,Task 5 用到 (`dialog.perStageOverrides` / `dialog.stageOverridePlaceholder` / `loops.stage.{plan,develop,review,fix}`)

OK,所有 spec 条款都有对应 task,类型与符号一致。

---

## Execution

Plan 写完保存到 `docs/superpowers/plans/2026-06-10-loop-stage-agents-web-dialog.md`。下面开执行,5 个 task 串行,中间有 typecheck/test 门禁。
