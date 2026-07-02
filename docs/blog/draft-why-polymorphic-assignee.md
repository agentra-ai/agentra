# Why Agentra Treats Agents as First-Class Team Members

> 2026-07-02 · 8 min read · [Edit on GitHub](https://github.com/agentra-ai/agentra/edit/main/docs/blog/draft-why-polymorphic-assignee.md)

## The uncomfortable observation

Most "AI coding assistant" products add AI as a **side effect**:
- Chat docked to the right side of your issue view
- A background cron that auto-replies to PR comments
- A CLI that runs locally with no team visibility

In every case, the AI agent lives in a **separate data plane** from your human teammates. The user model knows nothing about agents. The issue model knows nothing about agents. Agents are ghosts — invoked, then discarded.

## The Agentra bet

We inverted the assumption: **agents are assignees, not decorators**.

```sql
CREATE TABLE issues (
  ...
  assignee_type actor_type NOT NULL,   -- 'member' OR 'agent'
  assignee_id   UUID NOT NULL,         -- points to users.id OR agents.id
);
```

There is no `is_ai` boolean. No separate `agent_actions` table. An agent-assigned issue lives on the **same board**, in the **same comment thread**, with the **same reaction graph** as every human-assigned issue.

This isn't a product choice — it's a **data model choice**. And data model choices are the hardest to undo.

## Why the data model matters

### 1. Queries stay simple

```sql
-- "Show me everything assigned to Alice, whether Alice is human or agent"
SELECT * FROM issues
WHERE assignee_id = $1;

-- "Show me all agent work across the workspace"
SELECT * FROM issues
WHERE assignee_type = 'agent' AND workspace_id = $2;
```

No polymorphic JOIN. No type-specific tables. Index-friendly filters work natively.

### 2. UI stays unified

A single `<ActorAvatar />` component renders either `Bot` icon (agent) or initials (human) based on `assignee_type`. A single `<AssigneePicker />` merges members and agents in one searchable list.

When you introduce a new actor type (maybe `bot`, maybe `integration`), the UI already knows how to render it.

### 3. History survives entity lifecycle

When a human leaves, you `SET assignee_id = NULL`. When an agent is deleted, same operation. The issue's status, comments, and activity log remain intact. No cascade deletes. No lost context.

## Why competitors haven't done this

Polymorphic assignee sounds trivial until you try it on a legacy codebase:

1. **Your normalized FK constraints touch every table** — boards, comments, reactions, notification preferences, email templates. Migrating them is a multi-quarter, zero-downtime saga on production data.

2. **Your frontend assumes `user_id`** — `<AssigneeFace />`, `<AuthorBadge />`, `<MentionPicker />`, `<InboxNotifier />` all need polymorphic refactoring. In our codebase, that's 12+ components.

3. **An agent assignee is useless without an agent runtime** — Agentra ships a daemon that auto-detects local CLIs (Claude Code, Codex), creates isolated execution environments, and streams results back over WebSocket. Building that runtime is another 6+ months.

The migration cost is our **BD moat**: every quarter we ship on top of this primitive, competitors face compounding catch-up work.

## What this gives you as a user

In Agentra, your agents:
- **Appear on your Kanban** — same swimlanes as humans, same WIP limits
- **Reply in-thread** — agent comments nest under the trigger comment, same as a colleague's
- **React and are reacted to** — `+1`, `👍`, `🎉` work uniformly
- **Have a track record** — success rate, average task duration, common failure modes — visible on their profile
- **Self-update** — the daemon downloads new binaries via heartbeat and restarts without human babysitting

## Try it

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
cp .env.example .env  # set JWT_SECRET
docker compose up -d --build
```

Then `agentra login` → create workspace → create an agent → assign an issue.

The agent picks it up. You watch it work. No copy-pasting prompts.

## Deep dive

- Architecture doc: [Polymorphic Assignee — Data Model & Architecture](../architecture/polymorphic-assignee.md)
- Data schema: `shared/types/issue.ts`
- Read path: `pkg/db/queries/issue.sql` + `server/internal/handler/issues.go`
- Frontend picker: `features/issues/components/pickers/assignee-picker.tsx`

---

*Agentra is Apache 2.0. Built for 2–10 person teams where humans and agents ship together.*
