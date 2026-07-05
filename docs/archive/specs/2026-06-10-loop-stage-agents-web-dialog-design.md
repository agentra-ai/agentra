# Loop Stage Agents Web Dialog 设计规格

**日期**: 2026-06-10
**状态**: Approved
**目标**: 在 `StartLoopDialog` 暴露 per-stage agent override,让用户能选择 loop 的某个 stage(plan / develop / review / fix)使用不同的 agent,而不强制全部 stage 用同一个 agent。

---

## 1. 概述

### 1.1 背景

Go 端 `LoopConfig.StageAgents` 已经在这次迭代里实现完毕(migration 038 的 `loops.config` JSONB 字段,`loop.StageAgent(stage)` 解析函数,coordinator 在每个 stage 创建 task 时读 `StageAgent`,`CreateLoop` HTTP handler 校验并接受 `stage_agents` map,CLI `--stages` 标志 + `parseStageAgents` 已经支持)。本规格只覆盖 web 端对话层的对接,后端不动。

### 1.2 范围

**In scope**
- `apps/web/shared/types/loop.ts` 的 `StartLoopRequest` 类型
- `apps/web/features/loops/hooks.ts` 的 `useStartLoop` 透传
- `apps/web/features/loops/components/start-loop-dialog.tsx` 布局 + 状态 + 提交

**Out of scope**
- Loop detail page 的 stage_agent 展示(明确 defer)
- 后端 API(已就绪)
- 任何 agent / loop store 之外的状态

### 1.3 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| Layout | 4 个 stage picker 始终可见,加上保留的 default picker | 用户一眼看到所有配置,不需要点开折叠面板 |
| Empty stage picker 语义 | Use default | 跟 Go `Loop.StageAgent` 的 fallback 链一致 |
| Default 是否保留 | 保留 | Go 端 `Loop.AgentID` 是 NOT NULL,必须有 fallback |
| 提交时 `stage_agents` 字段 | 仅当 ≥1 个非空时带上,否则省略 | 减少 API payload 噪声,后端 omitempty 行为一致 |
| 重新打开 dialog | 清空所有 stage picker | 防止上一次的选择泄露到下一次 |
| Clear/重置 stage override | 不需要显式 clear UI | 选一个不同 agent 就是覆盖,想"退回 default"留空即可 |

---

## 2. 数据流

```
用户操作 StartLoopDialog
    │
    │ (a) 选 default agent
    │ (b) 可选地填 4 个 stage picker 中的若干个
    │
    ▼
StartLoopDialog.handleSubmit
    │
    │ build body:
    │   { issue_id, agent_id, max_iterations,
    │     ...(stage_agents if any non-empty) }
    │
    ▼
useStartLoop (hooks.ts) → loopsApi.startLoop(body)
    │
    ▼
POST /api/loops  →  Go handler  →  loops.config JSONB  →  coordinator
```

---

## 3. 类型改动

**`apps/web/shared/types/loop.ts`**

```ts
export interface StartLoopRequest {
  issue_id: string;
  agent_id: string;
  max_iterations?: number;
  stage_agents?: Partial<Record<LoopStage, string>>;
}
```

`Partial<Record<...>>` 让每个 stage 都是可选的,`string` 是 agent UUID。空对象 = 没有 override,字段在提交时被省略。

---

## 4. Hook 改动

**`apps/web/features/loops/hooks.ts`** — `useStartLoop` 的 input 类型和实现:

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

`useStartLoop` 内部不再声明自己的内联类型,直接用 `StartLoopRequest`。`loopsApi.startLoop` 本来就接受 `StartLoopRequest`,不需要改 `shared/api/loops.ts`。

---

## 5. Dialog 改动

**`apps/web/features/loops/components/start-loop-dialog.tsx`**

### 5.1 状态

新增 4 个 stage override 状态:

```ts
type StageOverrides = Partial<Record<LoopStage, string>>;
const [stageOverrides, setStageOverrides] = useState<StageOverrides>({});
```

reopen 重置(现有 `useEffect` 块加一行 `setStageOverrides({})`)。

### 5.2 布局

```
┌─ Start Loop ─────────────────────────┐
│ Default agent *                       │
│ [Backend Engineer            ▾]      │
│                                        │
│ Per-stage overrides                    │
│ Plan     [Use default (Backend Eng…]  │
│ Develop  [Use default (Backend Eng…]  │
│ Review   [Use default (Backend Eng…]  │
│ Fix      [Use default (Backend Eng…]  │
│                                        │
│ Max iterations [5]                     │
│                                        │
│ [Cancel]  [Start Loop]                 │
└────────────────────────────────────────┘
```

- 4 个 stage picker 标题用 `Plan` / `Develop` / `Review` / `Fix`
- placeholder 形如 `Use default ({defaultAgentName})`,让用户看到 "use default" 实际指向谁
- shadcn `Select` 的 `value=""` 表示空,触发 placeholder

### 5.3 提交逻辑

```ts
const cleanedOverrides: StageOverrides = {};
for (const [k, v] of Object.entries(stageOverrides)) {
  if (v) cleanedOverrides[k] = v;
}

const body: StartLoopRequest = {
  issue_id: issueId,
  agent_id: agentId,
  max_iterations: parsed,
};
if (Object.keys(cleanedOverrides).length > 0) {
  body.stage_agents = cleanedOverrides;
}

const loop = await startLoop(body);
```

`cleanedOverrides` 过滤掉空字符串(防御性 — picker 的 onChange 不应该传空字符串,但保险起见)。

### 5.4 校验

- Default agent 必填(保留现有逻辑)
- 4 个 stage picker 都可选
- 4 个 stage picker 都空 → 不发 `stage_agents`
- 任意 stage picker 选了 → 发 `stage_agents` 只包含那些非空的 stage
- Stage picker 选了跟 default 一样的 agent → 行为上跟 "use default" 等价(后端会优先取 override,效果一致),但用户能看到这是显式选择;不强制警告

---

## 6. i18n

复用 `apps/web/features/loops/components/start-loop-dialog.tsx` 已有的 `useTranslations("loops")` namespace。新增 key:

| key | 值 |
|-----|----|
| `dialog.perStageOverrides` | `Per-stage overrides` |
| `dialog.stageOverridePlaceholder` | `Use default ({agent})` (带 placeholder 参数) |

Stage label (`Plan` / `Develop` / `Review` / `Fix`) 直接展示在 picker 标题上,跟 `LoopStage` enum 字符串大写首字母一致。本期不引入 `loops.dialog.stages.*` 翻译 key — 后续 i18n 迭代再补。

---

## 7. 测试

### 7.1 单元 / 类型

- `pnpm typecheck` 必过(`StartLoopRequest` 改动会让 `useStartLoop` 自动满足,不需要单独的单测)
- 现有 `apps/web/shared/api/loops.test.ts` 如果覆盖 `startLoop` 的 body 序列化,补一个 case 验证 `stage_agents` 字段透传

### 7.2 手动 / E2E

不走 e2e 新增(范围外),但 manual checklist:

- [ ] 打开 dialog,4 个 stage picker 都是 placeholder `Use default (X)`
- [ ] 选 default = Backend Engineer,所有 stage placeholder 跟着变
- [ ] 给 develop 选 Test Engineer,其他保持空,提交
- [ ] DevTools Network 看到 POST `/api/loops` body:
  ```json
  {
    "issue_id": "...",
    "agent_id": "<Backend Engineer uuid>",
    "max_iterations": 5,
    "stage_agents": { "develop": "<Test Engineer uuid>" }
  }
  ```
- [ ] 不填任何 stage,提交,body **不包含** `stage_agents` 字段
- [ ] 4 个 stage 都填,提交,body 包含全部 4 个

---

## 8. 风险 / 取舍

- **视觉空间**: dialog 现在更长。如果 issue detail 区域窄,可能需要滚动。当前 `sm:max-w-2xl` 高度内可容纳 4 个 picker。
- **UX 一致性**: 4 个 picker 都是 shadcn `Select`,跟 default picker 一致,用户学习成本低。
- **i18n 兜底**: Stage label 暂时用英文 enum,后续可在 i18n 字典里加 `loops.dialog.stages.plan` 等。
- **未来扩展**: 如果以后要支持 "按 stage 限定 agent 子集"(例如 plan 只允许 read-only agents),会在 agent 后端加校验,UI 不需要变。
