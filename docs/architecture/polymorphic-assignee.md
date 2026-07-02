# Polymorphic Assignee — Data Model & Architecture

> **Why agents and humans share the same `issues.assignee_*` columns — and why this is our core moat.**

## The Problem

Most "AI-native" tools bolt agents on as a chat bubble, a sidebar, or a background cron. Agents live in a **separate data plane** from human teammates:

- Linear / Plane / Jira — no agent runtime at all
- SWE-agent / OpenDevin — agents exist only as demo scripts, never as persisted assignees
- Claude Code / Codex — agents run locally with no task management, no team visibility

The result: agents are **support functions**, not team members. You can't `@mention` an agent across an issue, track their work on a Kanban, or review their output inline with everyone else's.

## The Agentra Decision

At the database level, **agents and humans are structurally identical**:

```sql
CREATE TABLE issues (
  id            UUID PRIMARY KEY,
  workspace_id  UUID NOT NULL REFERENCES workspaces(id),
  title         TEXT NOT NULL,
  status        issue_status NOT NULL DEFAULT 'backlog',
  priority      issue_priority NOT NULL DEFAULT 'medium',
  assignee_type actor_type NOT NULL,            -- 'member' OR 'agent'
  assignee_id   UUID NOT NULL,                  -- points to users.id OR agents.id
  ...
);
```

There is **no `agent_issues` table**. There is no `is_ai` flag. An issue with `assignee_type = 'agent'` lives in the exact same board, timeline, comment thread, and reaction graph as one with `assignee_type = 'member'`.

### Why polymorphism, not inheritance

We chose **polymorphic association** over alternative approaches:

| Approach | Why we rejected it |
|----------|-------------------|
| Single `users` table with `is_agent` column | Forces agent-specific fields (runtime_id, instructions, visibility, model) onto every human user |
| Separate `agent_issues` / `member_issues` tables | Duplicates every lifecycle, comment, board, and reaction query |
| Join table `issue_assignees` with type enum | Extra JOIN on every read; gains nothing over the two-column form |
| Inheritance (`agent_assignees` table) | PostgreSQL inheritance has poor ORM/sqlc support; complicates FK constraints |

The two-column pattern (`assignee_type` + `assignee_id`) is:
- **Queryable** — `WHERE assignee_type = 'agent'` is an index-friendly filter
- **Migratable** — every issue keeps its history even if an agent is deleted
- **Extensible** — a future `bot` actor type needs only a new enum value
- **Honest** — the data model reflects the product truth: agents *are* assignees

## Read Path

```
IssueController.show(issue_id)
  └─ issue = issues.find(issue_id)
  └─ actor = resolve_actor(issue.assignee_type, issue.assignee_id)
       ├─ 'member' → users.find(issue.assignee_id)         → name, email, avatar
       └─ 'agent'  → agents.find(issue.assignee_id)          → name, instructions, visibility, runtime
  └─ render JSON with unified actor envelope
```

`resolve_actor` is the **single dispatcher**. All downstream code (board rendering, comment badges, notification routing, reaction feeds) interfaces with the unified `ActorEnvelope`:

```go
type ActorEnvelope struct {
  ID          string        `json:"id"`
  Type        ActorType     `json:"type"`        // "member" | "agent"
  Name        string        `json:"name"`
  AvatarURL   *string       `json:"avatar_url,omitempty"`
  IsAgent     bool          `json:"is_agent"`    // derived convenience field
  // agent-only fields are nil for members
  Visibility  *string       `json:"visibility,omitempty"`
  RuntimeID   *string       `json:"runtime_id,omitempty"`
}
```

## Write Path — Polymorphic Picker (Frontend)

```
AssigneePicker
  ├─ MemberSection
  │   └─ filtered members → PickerItem (avatar + initials)
  └─ AgentSection
      └─ filtered agents → PickerItem (Bot icon + name + private lock)
          └─ canAssignAgent() gate:
              ├─ public agent → everyone can assign
              └─ private agent → only owner/admin/agent.owner_id
```

The picker queries `GET /api/members` and `GET /api/agents` in parallel, then merges them into a unified list. Selection writes back `assignee_type + assignee_id` — no translation layer.

## Visibility Rules

```typescript
function canAssignAgent(
  agent: Agent,
  userId: string | undefined,
  memberRole: 'owner' | 'admin' | 'member'
): boolean {
  if (agent.visibility === 'public') return true;
  if (!userId) return false;
  return memberRole === 'owner'
      || memberRole === 'admin'
      || agent.owner_id === userId;
}
```

Private agents belong to a single developer (e.g. a personal coding assistant with custom API keys). Public agents belong to the workspace. The enforcement happens at:
- **Middleware** — `assign-issue` handler validates visibility before write
- **Picker UI** — private agents appear as `disabled + ⌁ Lock` for unauthorized users
- **API** — `POST /api/issues` rejects invalid assignee with 403

## How Competitors Would Copy This (and why they haven't)

Adding polymorphic assignee to a legacy task manager sounds easy. In practice:

1. **Database migration scar tissue** — your normalized `member_issues` FK constraints touch every table: boards, comments, reactions, notifications, activity logs. Rewriting them means a multi-quarter migration with zero-downtime guarantees on production data.

2. **Frontend re-architecture** — your entire `<AssigneeFace />` component tree assumes `user_id`. Making it polymorphic propagates through 12+ components (board cards, detail headers, comment avatars, mention pickers, inbox badges, email templates).

3. **Agent runtime coupling** — without Agentra's Backend SDK (`pkg/agent/agent.go`), an agent assignee is just a string in a column. Building the daemon + task lifecycle + WebSocket sync to actually execute those assignees is another 6+ months of operational scar tissue.

**Our lead**: the data model shipped in `v0.1`. Every feature since has been built on top of it. Competitors starting today face **6+ months** of migration before they can even offer the same primitive.

## Related Reading

- `shared/types/issue.ts` — `ActorType` enum and `Issue` schema
- `pkg/db/queries/issue.sql` — polymorphic read queries (generated by sqlc)
- `server/internal/handler/issues.go` — write path validation
- `features/issues/components/pickers/assignee-picker.tsx` — unified picker UI
- `features/workspace/hooks.ts` — `useActorName` resolver
