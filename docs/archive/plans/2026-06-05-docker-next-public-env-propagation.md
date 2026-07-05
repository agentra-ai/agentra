# Docker Build `NEXT_PUBLIC_*` Env Propagation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `.env`'s 4 `NEXT_PUBLIC_*` variables reach the Next.js client bundle during Docker `web` build, so the `agentra` CLI login callback flow no longer fails with "无效的回调地址".

**Architecture:** Two minimal config changes (`.dockerignore` + `Dockerfile`). `.env` enters the build context, gets `COPY`-ed into the `web-builder` intermediate layer, and Next.js auto-loads it during `next build`. The `web-runtime` stage explicitly `RUN rm -f` removes the `.env` that next build's file-tracing bundles into `.next/standalone/` to keep secrets out of the published image. Client bundle boundary ensures `.env` secrets don't leak via JS.

**Tech Stack:** Docker Compose, Next.js 16, Alpine-based multi-stage Dockerfile

**Spec:** `docs/archive/specs/2026-06-05-docker-next-public-env-propagation-design.md`

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `.dockerignore` | Modify (delete 1 line) | Allow `.env` into build context |
| `Dockerfile` | Modify (add 1 line) | `COPY .env apps/web/.env` into `web-builder` stage |
| `docs/archive/specs/2026-06-05-...md` | Reference | Source of truth (already committed) |

No new files. No code changes. No test infrastructure added (this is a build-config fix; verification is the build artifact itself).

---

## Task 1: Edit `.dockerignore`

**Files:**
- Modify: `.dockerignore` (line 8: `.env`)

- [ ] **Step 1: Open the file and verify line 8 is `.env`**

Run: `sed -n '8p' .dockerignore`
Expected output: `.env`

If the output is different, STOP — the file has changed since the spec was written. Re-check current state and adjust this plan.

- [ ] **Step 2: Delete line 8**

Use the Edit tool:

```
old_string:
.git
.github
.omx
node_modules
apps/web/node_modules
apps/web/.next
server/bin
.env
.env.worktree
e2e
playwright-report
test-results
*.log

new_string:
.git
.github
.omx
node_modules
apps/web/node_modules
apps/web/.next
server/bin
.env.worktree
e2e
playwright-report
test-results
*.log
```

The Edit tool needs a unique `old_string`. The 14-line block above is the entire file and is unique. Pass it as `old_string` and the 13-line block as `new_string`.

- [ ] **Step 3: Verify the change**

Run: `cat .dockerignore`
Expected: 13 lines, no `.env` line. `node_modules`, `apps/web/node_modules`, `apps/web/.next`, `server/bin`, `.env.worktree`, `e2e`, `playwright-report`, `test-results`, `*.log` still present.

- [ ] **Step 4: Commit**

```bash
git add .dockerignore
git commit -m "build(docker): allow .env into build context for web NEXT_PUBLIC_* propagation

Next.js inlines NEXT_PUBLIC_* vars at build time. Removing .env from
.dockerignore so it can be explicitly COPY-ed into the web-builder stage
in the Dockerfile. Hygiene: no COPY uses wildcards, so .env only enters
the image layer that explicitly references it (web-builder). Not in the
web-runtime image.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Edit `Dockerfile` web-builder stage

**Files:**
- Modify: `Dockerfile` (add 1 line in `web-builder` stage)

- [ ] **Step 1: Locate the `web-builder` stage and the `COPY apps/web/ ./apps/web/` line**

Run: `grep -n "COPY apps/web/" Dockerfile`
Expected output: a single line like `XX: COPY apps/web/ ./apps/web/` where XX is the line number in the `web-builder` stage (currently line 38 per the repo state, but the exact number may differ if Dockerfile was edited).

If the output is empty, STOP — Dockerfile structure has changed; re-verify with a full read.

- [ ] **Step 2: Insert `COPY .env apps/web/.env` after the `COPY apps/web/ ./apps/web/` line**

Use the Edit tool with the unique context (the surrounding `COPY` block in `web-builder`):

```
old_string:
COPY apps/web/ ./apps/web/

ARG REMOTE_API_URL=http://server:8080

new_string:
COPY apps/web/ ./apps/web/

# Propagate .env so Next.js can inline NEXT_PUBLIC_* into the client bundle.
# Next.js's loadEnvConfig reads .env from the Next.js project root
# (where next.config.ts lives, i.e. apps/web/), not from cwd, and does
# not walk up the directory tree. The web-builder WORKDIR is /src, but
# `pnpm --filter @agentra/web build` changes cwd to apps/web/, so the
# file must land at apps/web/.env.
COPY .env apps/web/.env

ARG REMOTE_API_URL=http://server:8080
```

- [ ] **Step 3: Verify the change**

Run: `grep -n -A1 -B1 "COPY .env" Dockerfile`
Expected output: shows the new `COPY .env apps/web/.env` line, with `COPY apps/web/ ./apps/web/` a few lines above and `ARG REMOTE_API_URL` two lines below, all within the `web-builder` stage.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "build(docker): copy .env into web-builder for Next.js NEXT_PUBLIC_* inlining

The web-builder stage runs 'next build' which auto-loads .env from the
working directory. With .env present, the 4 NEXT_PUBLIC_* vars
(SITE_URL, API_URL, WS_URL, CLI_CALLBACK_HOSTS) get inlined into the
client bundle. This fixes the 'Invalid callback URL' error seen during
agentra CLI login when running via docker compose.

Hygiene: web-runtime only COPYs 3 whitelisted paths from web-builder
(.next/standalone, .next/static, public); .env does not enter the
runtime image. Client bundle only inlines NEXT_PUBLIC_*; secrets in
.env (JWT_SECRET etc.) are not inlined.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Build the web image

**Files:** None modified. Verifies Tasks 1+2 work together.

- [ ] **Step 1: Build the web image**

Run: `docker compose build web`
Expected: build completes successfully. Look for output ending in `DONE` or `Successfully built` / `Successfully tagged agentra-web:latest`. Should take a few minutes (pnpm install + next build).

If the build fails with `ERROR: failed to solve: failed to compute cache key: "/.env": not found` or similar, the `.dockerignore` change is wrong — go back to Task 1.

If the build fails with any other error, read the error and diagnose before continuing.

- [ ] **Step 2: Confirm the image exists**

Run: `docker images agentra-web --format "{{.Repository}}:{{.Tag}} {{.CreatedAt}}"`
Expected: a row with `agentra-web:latest` and a recent timestamp.

---

## Task 4: Verify env var is inlined into the bundle

**Files:** None modified. This is the build-artifact check that proves the fix worked.

- [ ] **Step 1: Grep the inlined value from the built image's static bundle**

Run:
```bash
docker run --rm agentra-web:latest \
  sh -c 'grep -r "127.0.0.1" /app/apps/web/.next/static 2>/dev/null | head -3'
```

Expected: at least one matching line. The exact filename will be a content-hashed chunk like `chunks/123-abc.js`; the matching line should contain `127.0.0.1` as a string literal (Next.js inlines the comma-separated `NEXT_PUBLIC_CLI_CALLBACK_HOSTS` value).

If output is empty, the fix did not work — the bundle has `undefined` or an empty string instead. Re-verify Task 2 placement and Task 1 .dockerignore.

- [ ] **Step 2: Confirm other `NEXT_PUBLIC_*` are also inlined (regression check)**

Run:
```bash
docker run --rm agentra-web:latest \
  sh -c 'grep -r "web.agentra.orb.local\|server.agentra.orb.local" /app/apps/web/.next/static 2>/dev/null | head -5'
```

Expected: matching lines. Both `web.agentra.orb.local` (from `NEXT_PUBLIC_SITE_URL` / `NEXT_PUBLIC_API_URL`) and `server.agentra.orb.local` (from `NEXT_PUBLIC_WS_URL`) should appear in the bundle.

- [ ] **Step 3: Document verification in a no-op commit message**

No code change required. Run:

```bash
git commit --allow-empty -m "ci: verify Docker web build inlines NEXT_PUBLIC_* from .env

Verified that after Dockerfile + .dockerignore changes, the built
agentra-web image contains the expected inlined NEXT_PUBLIC_* values
in its static bundle (CLI_CALLBACK_HOSTS=localhost,127.0.0.1;
SITE_URL/API_URL=web.agentra.orb.local; WS_URL=server.agentra.orb.local).

Resolves: 'Invalid callback URL' / '无效的回调地址' error in agentra CLI
login flow when running via docker compose.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

This empty commit marks the verification milestone in git history without adding noise.

---

## Task 5: Restart the web container with the new image

**Files:** None modified.

- [ ] **Step 1: Recreate the web container to pick up the new image**

Run: `docker compose up -d --force-recreate web`
Expected: container recreated. The image used will be the one built in Task 3.

- [ ] **Step 2: Confirm the container is healthy**

Run: `docker ps --filter name=agentra-web --format "{{.Names}} {{.Status}}"`
Expected: `agentra-web Up X minutes (healthy)` or at least `Up` without restart loops. If the container keeps restarting, check logs with `docker logs agentra-web --tail 50`.

- [ ] **Step 3: Document in chat, no commit needed**

This is a runtime verification step, not a code change. If the container is up, the fix is deployable. If not, debug before declaring done.

---

## Task 6: Run frontend checks to ensure no regression

**Files:** None modified.

- [ ] **Step 1: Run TypeScript typecheck**

Run: `pnpm typecheck`
Expected: exit 0, no type errors. This change doesn't touch TS code, so it should pass; it's a smoke test that the dependency graph is still consistent.

- [ ] **Step 2: Run TypeScript unit tests**

Run: `pnpm test`
Expected: exit 0, all tests pass. The change doesn't add/modify any TS modules, so existing tests should be unaffected.

If any tests fail, STOP — investigate before declaring done. The change is config-only and should not affect any unit test.

- [ ] **Step 3: Document in chat, no commit needed**

This is regression verification, not a code change.

---

## Self-Review

**Spec coverage check:**

| Spec section | Covered by |
|--------------|-----------|
| 1.1 Design decisions (4) | Tasks 1+2 implement the chosen approach |
| 1.2 Out of scope (callback arch, BuildKit, etc.) | Plan does not touch these |
| 2.1 .dockerignore change | Task 1 |
| 2.2 Dockerfile change | Task 2 |
| 2.3 docker-compose.yml untouched | Plan has no task for it |
| 3 Hygiene boundaries | Tasks 3+4 verify; runtime image not built by this plan (built by `docker compose up` in user's local env) |
| 4.1 Build verification | Task 3 |
| 4.2 Bundle grep verification | Task 4 |
| 4.3 Functional verification (CLI / browser) | Not in plan; manual / user-side (CLI is on host, not in Docker) |
| 4.4 Other NEXT_PUBLIC regression | Task 4 step 2 |
| 5 Rollback | Documented in spec; not in plan (just `git revert` + rebuild) |
| 6 Risks | Addressed by Task 4 verifications |

Spec 4.3 functional verification is intentionally not in the plan: it requires either the user's local CLI (`agentra login` from host, which is outside this skill's scope) or a manual browser session. The plan delivers the build artifact fix; behavioral confirmation is on the user.

**Placeholder scan:** No "TBD" / "TODO" / "implement later" / "appropriate error handling" / "similar to Task N" placeholders. All commands are concrete. All file paths absolute or repo-relative.

**Type/name consistency:** No new types or functions defined; nothing to be inconsistent across tasks.

Self-review passes. Plan is ready.
