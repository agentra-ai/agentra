# One-Command Install Design

**Date**: 2026-05-10
**Based on**: Research of swarmclaw, agent-tasks, open-multi-agent, create-t3-app, create-next-app, and GoReleaser/Homebrew patterns
**Status**: Draft

---

## 1. Overview

### 1.1 Goal

Reduce Agentra's time-to-first-agent-task from ~15 minutes (current SELF_HOSTING.md flow: clone repo, copy .env, edit 10+ variables, docker compose up, install CLI, login, start daemon) to **under 2 minutes** with a single command.

### 1.2 Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Primary path | `npx create-agentra` (npm scaffolding) | Matches create-t3-app / create-next-app industry pattern; zero pre-reqs beyond Node.js; interactive prompts reduce config errors |
| Secondary path | Homebrew formula (GoReleaser `brews`) | Covers macOS users who prefer native package managers; GoReleaser auto-generates formula from existing `.goreleaser.yml` |
| Tertiary path | Docker one-liner (`curl | bash`) | Covers users who want a pre-built container; matches swarmclaw's GHCR image pattern |
| Scaffolding tool language | Node.js (TypeScript) | Same stack as frontend; one repo, one language for dev tooling; `@clack/prompts` for interactive UX |
| Web container | Next.js standalone output, served by `node server.js` | Already built in Dockerfile; keep it simple |
| DB decision for quickstart | PostgreSQL via Docker Compose (same as now) | Agentra requires pgvector; no SQLite fallback — the product IS PostgreSQL-native |
| Auth bootstrapping | Auto-create admin user via seed endpoint on first boot | Removes the manual "create first user" step; matches swarmclaw's auto-setup pattern |
| Gateway inclusion in quickstart | Optional, off by default | Quickstart targets single-machine eval; gateway adds Docker socket complexity |
| MinIO inclusion in quickstart | Optional, off by default | File attachments are a secondary feature; add when needed |

### 1.3 What We Are NOT Doing

- **No SQLite fallback.** Agentra uses pgvector for embeddings. SQLite with sqlite-vec would be a separate product decision, not an installation concern.
- **No "agentra cloud" signup flow in the scaffolder.** The scaffolder sets up self-hosted. A separate landing page handles cloud signups.
- **No desktop app (yet).** Swarmclaw's desktop app (Electron) is a significant engineering investment. Revisit post-1.0.
- **No single-binary embedded frontend.** Go's `embed.FS` serving a Next.js app introduces complexity with asset paths, SSR, and WebSocket routing. Docker Compose already solves multi-process orchestration cleanly.

---

## 2. Competitive Research Summary

### 2.1 swarmclaw (`@swarmclawai/swarmclaw`)

- **Global install**: `npm i -g @swarmclawai/swarmclaw` then `swarmclaw` starts server on `:3456`
- **Desktop app**: One-click download from swarmclaw.ai/downloads (macOS, Windows, Linux via AppImage/.deb). Electron-based with Developer ID signing.
- **Docker**: Clone repo, `docker compose up -d --build`. Has `render.yaml`, `fly.toml`, `railway.json` for PaaS deploys.
- **Repo quickstart**: `npm run quickstart` — installs deps, prepares config, starts server.
- **Published image**: `ghcr.io/swarmclawai/swarmclaw:latest`
- **Stats**: 73 dependencies, 14.2 MB unpacked, MIT license
- **Key takeaway**: Multiple paths (global npm, desktop app, Docker, repo clone) — users pick their comfort level. Desktop app for non-technical users is the differentiator.

### 2.2 agent-tasks (`agent-tasks`)

- **Install**: `npm install -g agent-tasks`
- **MCP server mode**: Add to MCP client config pointing to `npx agent-tasks`. Dashboard auto-starts at `localhost:3422` on first MCP connection.
- **Standalone**: `node dist/server.js --port 3422`
- **Architecture**: SQLite-based (via `better-sqlite3`). No external DB needed — all state in `~/.agent-tasks/agent-tasks.db`.
- **Stats**: 4 dependencies, 523.8 KB unpacked, MIT license
- **Key takeaway**: Extreme simplicity wins for developer tools. Zero-config startup. SQLite eliminates the DB setup step entirely. However, agent-tasks is a single-process MCP server, not a full platform with a web UI and multi-tenancy.

### 2.3 open-multi-agent (`@open-multi-agent/core`)

- **Install**: `npm install @open-multi-agent/core`
- **Usage**: Import the library, call `runTeam(team, goal)`. No server, no DB, no UI.
- **CLI binary**: `oma` for shell/CI usage.
- **Stats**: **3 runtime dependencies** (`@anthropic-ai/sdk`, `openai`, `zod`), 1.0 MB unpacked, MIT license
- **Key takeaway**: Library-first design means zero infrastructure. Users bring their own Node.js app. This is the lightest possible "install" — but it works because OMA is a library, not a platform.

### 2.4 create-t3-app / create-next-app (Scaffolding Pattern)

- **Invocation**: `npm create t3-app@latest` / `npx create-next-app@latest`
- **Flow**: Interactive CLI prompts (via `@clack/prompts`) -> generate project files -> print next steps
- **Prompts**: Project name, language (TS/JS), optional modules (Tailwind, tRPC, auth, ORM), import alias
- **Generated files**: `package.json`, `tsconfig.json`, `next.config.js`, `src/` scaffold, `.env`, `.gitignore`
- **Post-scaffold**: `cd <project> && npm install && npm run dev`
- **Key takeaway**: The `npm create` / `npx create-*` convention is universally understood by JS developers. Interactive prompts reduce decision fatigue. Generated files are the "source of truth" that users can version control.

### 2.5 GoReleaser Homebrew Formula

- **Configuration**: Add a `brews` section to `.goreleaser.yml`:
  ```yaml
  brews:
    - name: agentra
      homepage: https://agentra.ai
      description: "AI-native task management platform"
      license: MIT
      repository:
        owner: agentra-ai
        name: homebrew-tap
      dependencies:
        - name: go
  ```
- **Publishing**: On `git tag vX.Y.Z`, GoReleaser builds binaries for darwin/linux amd64/arm64, creates a GitHub release, and pushes a formula to `agentra-ai/homebrew-tap`.
- **User install**: `brew install agentra-ai/tap/agentra`
- **Requirements**: A separate `homebrew-tap` repo; `HOMEBREW_TAP_GITHUB_TOKEN` secret in CI.
- **Key takeaway**: GoReleaser makes this nearly free if you already use it for GitHub Releases. The main work is creating the tap repo and setting up the CI secret.

---

## 3. Option A: `npx create-agentra` (Recommended Primary Path)

### 3.1 User Experience

```bash
npx create-agentra@latest
```

The user sees an interactive terminal experience:

```
┌  create-agentra v1.0.0
│  Set up Agentra in under 2 minutes.
│
◇  Where should we create your Agentra project?
│  ./my-agentra
│
◇  What is your workspace name?
│  Acme Corp
│
◇  Admin email (for magic-link login)?
│  admin@acme.com
│
◇  Use a custom domain? (optional)
│  agentra.acme.com
│
◇  Enable GitHub OAuth? (optional)
│  Yes / No
│
◇  GitHub OAuth Client ID:
│  ...
│
◇  GitHub OAuth Client Secret:
│  ...
│
◇  Enable file attachments? (requires S3)
│  Yes / No
│
◇  Enable agent gateway? (Docker-in-Docker for isolated agent runtimes)
│  Yes / No
│
│  ◒ Generating project files...
│  ◒ Writing docker-compose.yml...
│  ◒ Writing .env...
│  ◒ Writing Caddyfile...
│
├── .env
├── .env.example
├── .gitignore
├── Caddyfile
├── docker-compose.yml
└── README.md

◇  Project created at ./my-agentra
│
◇  Starting Agentra...
│
◇  Agentra is running!
│
│  Frontend:  http://localhost:3000
│  API:       http://localhost:8080
│  Health:    http://localhost:8080/health
│
│  Next steps:
│    cd my-agentra
│    docker compose ps          # Check running services
│    docker compose logs -f     # Watch logs
│
│  Install the CLI on your machine:
│    brew install agentra-ai/tap/agentra
│    agentra login
│    agentra daemon start
│
└  Done in 47s 🎉
```

### 3.2 Scaffold Flow (Implementation)

```
User runs: npx create-agentra@latest
  │
  ├─ 1. Intro banner (package.json version, one-line description)
  │
  ├─ 2. Prompt: project directory (default: ./agentra)
  │     - Validate: directory must not exist or be empty
  │
  ├─ 3. Prompt: workspace name (default: "My Workspace")
  │
  ├─ 4. Prompt: admin email (required, validated with regex)
  │     - This email will receive magic-link auth emails
  │
  ├─ 5. Prompt: custom domain (optional)
  │     - If provided: generate Caddyfile with auto-TLS
  │     - If empty: localhost-only setup
  │
  ├─ 6. Prompt: GitHub OAuth (optional)
  │     - If yes: prompt for client ID + secret
  │     - If no: skip, magic-link auth only
  │
  ├─ 7. Prompt: file attachments (S3/MinIO) (optional, default: No)
  │     - If yes: include MinIO in docker-compose, prompt for S3 config
  │
  ├─ 8. Prompt: agent gateway (optional, default: No)
  │     - If yes: include gateway service, Docker socket mount
  │
  ├─ 9. Generate files:
  │     - .env (from template, with user-provided values)
  │     - .gitignore (node_modules, .env, data/)
  │     - docker-compose.yml (from template, conditional services)
  │     - Caddyfile (only if custom domain provided)
  │     - README.md (generated with project-specific next steps)
  │
  ├─ 10. Pull images & start:
  │     - docker compose pull (pre-built images from GHCR)
  │     - docker compose up -d
  │     - Wait for health checks
  │
  └─ 11. Print success summary with URLs + next steps
```

### 3.3 Generated `docker-compose.yml` Template

The scaffolder generates a simplified docker-compose.yml that uses **pre-built images** from GHCR (no local build):

```yaml
name: agentra

services:
  postgres:
    image: pgvector/pgvector:pg17
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-agentra}
      POSTGRES_USER: ${POSTGRES_USER:-agentra}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-agentra}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-agentra}"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  migrate:
    image: ghcr.io/agentra-ai/agentra:latest
    entrypoint: ["./migrate"]
    command: ["up"]
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy
    restart: "no"

  server:
    image: ghcr.io/agentra-ai/agentra:latest
    entrypoint: ["./server"]
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
    ports:
      - "${PORT:-8080}:8080"
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O - http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  web:
    image: ghcr.io/agentra-ai/agentra-web:latest
    env_file:
      - .env
    ports:
      - "${FRONTEND_PORT:-3000}:3000"
    depends_on:
      server:
        condition: service_healthy
    restart: unless-stopped

  # Optional: only included if user selected "Enable agent gateway"
  gateway:
    image: ghcr.io/agentra-ai/agentra-gateway:latest
    environment:
      AGENTRA_SERVER_URL: ws://server:8080/ws
      GATEWAY_ID: gateway-1
      GATEWAY_WORKSPACE_ID: ${GATEWAY_WORKSPACE_ID:-}
      AGENTRA_AUTH_TOKEN: ${AGENTRA_AUTH_TOKEN:-}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      server:
        condition: service_healthy
    restart: unless-stopped
    profiles:
      - gateway

  # Optional: only included if user selected "Enable file attachments"
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ACCESS_KEY:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_SECRET_KEY:-minioadmin}
    volumes:
      - miniodata:/data
    ports:
      - "${MINIO_API_PORT:-9000}:9000"
      - "${MINIO_CONSOLE_PORT:-9001}:9001"
    restart: unless-stopped
    profiles:
      - storage

volumes:
  pgdata:
  miniodata:
```

### 3.4 Generated `.env` Template

```bash
# === Agentra Configuration ===
# Generated by create-agentra on 2026-05-10

# Database
POSTGRES_DB=agentra
POSTGRES_USER=agentra
POSTGRES_PASSWORD=<auto-generated-32-char-hex>

# Server
PORT=8080
FRONTEND_PORT=3000
JWT_SECRET=<auto-generated-64-char-hex>

# Initial admin user (used for first-run seeding)
AGENTRA_ADMIN_EMAIL=admin@acme.com
AGENTRA_WORKSPACE_NAME=Acme Corp

# Public URLs
FRONTEND_ORIGIN=http://localhost:3000
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080/ws

# Custom domain (uncomment and set for production)
# FRONTEND_ORIGIN=https://agentra.acme.com
# NEXT_PUBLIC_API_URL=https://agentra.acme.com
# NEXT_PUBLIC_WS_URL=wss://agentra.acme.com/ws

# Email (Resend)
RESEND_API_KEY=
RESEND_FROM_EMAIL=noreply@agentra.ai

# GitHub OAuth (optional)
# GITHUB_CLIENT_ID=
# GITHUB_CLIENT_SECRET=
# GITHUB_REDIRECT_URI=http://localhost:3000/auth/callback

# File Storage (optional, uncomment to enable)
# MINIO_ACCESS_KEY=minioadmin
# MINIO_SECRET_KEY=minioadmin
# MINIO_SERVER_URL=http://localhost:9000

# Gateway (optional, uncomment to enable)
# GATEWAY_WORKSPACE_ID=
# AGENTRA_AUTH_TOKEN=
```

Key design points:
- **JWT_SECRET and POSTGRES_PASSWORD are auto-generated** — no "change-me-in-production" defaults
- **AGENTRA_ADMIN_EMAIL and AGENTRA_WORKSPACE_NAME** feed a first-run seed endpoint that creates the workspace + admin user on server startup
- **Optional sections are commented out** — users uncomment what they need
- **No hardcoded secrets** — every secret is generated at scaffold time

### 3.5 First-Run Seed Endpoint

The server gains a new startup behavior: if the workspace table is empty, it auto-creates the workspace and admin user from env vars:

```
Server startup
  │
  ├─ Run migrations (as today)
  │
  ├─ Check: does any workspace exist?
  │     ├─ Yes → skip seeding
  │     └─ No  → create workspace from AGENTRA_WORKSPACE_NAME
  │              → create user from AGENTRA_ADMIN_EMAIL
  │              → print: "First-run seed complete. Admin: admin@acme.com"
  │
  └─ Start listening
```

This eliminates the manual "create first workspace and user" step. The admin receives a magic-link email on first login attempt.

### 3.6 Scaffolder Package Structure

```
packages/create-agentra/          # New package in monorepo
├── package.json                  # name: "create-agentra", bin: "create-agentra"
├── tsconfig.json
├── src/
│   ├── index.ts                  # Entry point: parse args, run prompts, generate
│   ├── prompts.ts                # @clack/prompts interactive flow
│   ├── templates/
│   │   ├── docker-compose.ts     # Template with conditional sections
│   │   ├── env.ts                # .env template with value interpolation
│   │   ├── caddyfile.ts          # Caddyfile template (custom domain only)
│   │   ├── gitignore.ts          # .gitignore template
│   │   └── readme.ts             # README template with project-specific info
│   ├── generator.ts              # File writing, directory creation
│   ├── docker.ts                 # docker compose pull + up orchestration
│   └── utils.ts                  # Random string generation, validation
├── test/
│   └── generator.test.ts         # Unit tests for template generation
└── README.md
```

Dependencies (minimal, following create-next-app's 0-dependency approach):
- `@clack/prompts` — interactive terminal prompts
- `execa` — run `docker compose` commands (or use `child_process` to keep deps at 1)

### 3.7 CI/CD for the Scaffolder

- The `create-agentra` package is published to npm on every tag push (alongside the GoReleaser release)
- Version matches the main Agentra release version
- The GHCR images (`ghcr.io/agentra-ai/agentra:latest`, `ghcr.io/agentra-ai/agentra-web:latest`) are built and pushed on every merge to `main`
- The scaffolder always references `:latest` images — users get the latest stable release

---

## 4. Option B: `brew install agentra` (Go Binary)

### 4.1 User Experience

```bash
# Add tap (one-time)
brew tap agentra-ai/tap

# Install
brew install agentra

# Initialize a new Agentra project
agentra init

# Start the daemon
agentra daemon start
```

### 4.2 `agentra init` Flow

The `agentra init` command (new subcommand of the existing `agentra` CLI binary) provides a non-interactive or minimally-interactive setup:

```bash
# Interactive mode
agentra init

# Non-interactive mode (CI/CD, scripts)
agentra init \
  --workspace "Acme Corp" \
  --admin-email admin@acme.com \
  --domain agentra.acme.com \
  --github-client-id xxx \
  --github-client-secret yyy
```

Flags:
- `--workspace` — workspace name (default: prompts interactively)
- `--admin-email` — admin email (default: prompts)
- `--domain` — custom domain (optional)
- `--github-client-id` / `--github-client-secret` — GitHub OAuth (optional)
- `--with-gateway` — include agent gateway (default: false)
- `--with-storage` — include MinIO (default: false)
- `--output-dir` — where to write files (default: `./agentra`)
- `--start` — run `docker compose up -d` after generation (default: true)

Output: same `.env`, `docker-compose.yml`, `Caddyfile`, `.gitignore` as the npm scaffolder.

### 4.3 GoReleaser Configuration

Add to `.goreleaser.yml`:

```yaml
brews:
  - name: agentra
    homepage: https://agentra.ai
    description: "AI-native task management platform — like Linear, but with AI agents as first-class citizens"
    license: MIT
    repository:
      owner: agentra-ai
      name: homebrew-tap
      branch: main
    commit_author:
      name: agentra-bot
      email: bot@agentra.ai
    dependencies:
      - name: go
        type: build
    install: |
      bin.install "agentra"
    test: |
      system "#{bin}/agentra --version"
```

### 4.4 CI Setup

1. Create repo: `github.com/agentra-ai/homebrew-tap`
2. Create GitHub personal access token with `public_repo` scope
3. Add `HOMEBREW_TAP_GITHUB_TOKEN` to repo secrets (already referenced in `release.yml`)
4. On next `git tag vX.Y.Z`, GoReleaser auto-pushes formula to the tap

### 4.5 What `brew install agentra` Does NOT Do

- Does NOT install Docker (user must have Docker separately)
- Does NOT set up the server (that's `agentra init`)
- Does NOT start anything automatically

The Homebrew formula installs only the CLI binary. The `agentra init` subcommand handles project scaffolding (same output as the npm scaffolder).

---

## 5. Option C: Docker One-Liner

### 5.1 curl | bash (Host Install)

```bash
curl -sSL https://agentra.ai/install.sh | bash
```

The install script:
1. Detects OS and architecture
2. Downloads the latest `agentra` binary from GitHub Releases
3. Installs to `/usr/local/bin/agentra`
4. Prints: `agentra init` to get started

### 5.2 docker run (Containerized)

```bash
docker run -d \
  --name agentra \
  -p 8080:8080 \
  -p 3000:3000 \
  -e POSTGRES_DB=agentra \
  -e POSTGRES_USER=agentra \
  -e POSTGRES_PASSWORD=agentra \
  -e JWT_SECRET=change-me \
  ghcr.io/agentra-ai/agentra-standalone:latest
```

This requires a "standalone" image that bundles PostgreSQL alongside the app (using `supervisord` or a similar process manager). This is NOT the same as the multi-service Docker Compose setup — it is a single-container convenience for evaluation.

**Decision: Defer the standalone image.** The multi-service Docker Compose setup is the correct architecture for production. A single-container image would require significant rework (bundling PostgreSQL, managing process lifecycle) and encourages an anti-pattern for production. If demand arises, revisit post-1.0.

### 5.3 What We WILL Ship for Docker

The `install.sh` script (Option C path 1: download binary) alongside the `docker compose up` generated by `create-agentra` and `agentra init`. No standalone Docker image in v1.

---

## 6. Implementation Priority

### Phase 1: Foundation (Weeks 1-2)

| # | Task | Effort | Depends On |
|---|------|--------|------------|
| 1 | Build and publish multi-arch Docker images to GHCR (`agentra`, `agentra-web`, `agentra-gateway`) | 3d | Dockerfile exists |
| 2 | Add first-run seed endpoint to server (auto-create workspace + admin from env vars) | 1d | — |
| 3 | Create `homebrew-tap` repo, add token to CI, add `brews` section to `.goreleaser.yml` | 1d | — |
| 4 | Write `install.sh` script, host on agentra.ai | 1d | GitHub Releases exist |

### Phase 2: npm Scaffolder (Weeks 2-3)

| # | Task | Effort | Depends On |
|---|------|--------|------------|
| 5 | Create `packages/create-agentra/` package with `@clack/prompts` interactive flow | 3d | — |
| 6 | Implement template generation (docker-compose, .env, Caddyfile, .gitignore, README) | 2d | — |
| 7 | Add `docker compose pull && up` orchestration | 1d | #1 |
| 8 | Publish `create-agentra` to npm, add to release CI | 1d | #5-7 |
| 9 | Write tests for template generation | 1d | #6 |

### Phase 3: Go CLI `init` (Weeks 3-4)

| # | Task | Effort | Depends On |
|---|------|--------|------------|
| 10 | Implement `agentra init` subcommand in Go (reuse template logic from npm scaffolder, or call it as a library) | 2d | #6 |
| 11 | Add `agentra init` to CLI help, update CLI_AND_DAEMON.md | 1d | #10 |

### Phase 4: Polish (Week 4)

| # | Task | Effort | Depends On |
|---|------|--------|------------|
| 12 | End-to-end test: `npx create-agentra@latest` -> docker compose up -> open browser -> see Agentra | 2d | #8 |
| 13 | Update README.md, SELF_HOSTING.md, agentra.ai/install landing page | 1d | #8, #11 |
| 14 | Performance optimization: measure and optimize image pull times, startup time | 1d | #1 |

---

## 7. Success Metrics

### Primary Metric: Time to First Agent Task

| Stage | Target | Measurement |
|-------|--------|-------------|
| `npx create-agentra` runs | <5s | Time from enter to first prompt |
| All prompts answered | <30s | Typical user interaction time |
| `docker compose pull` | <60s | Depends on network; GHCR is fast |
| `docker compose up -d` to healthy | <30s | PostgreSQL startup + migrations + server + web |
| Open browser, request magic link, click link | <20s | Admin email is pre-seeded |
| Create first issue, assign to agent | <15s | UI interaction |
| **Total** | **<2 minutes** | **Target** |

### Secondary Metrics

| Metric | Target | Notes |
|--------|--------|-------|
| `create-agentra` package size (unpacked) | <500 KB | follow create-next-app's minimalism |
| `create-agentra` dependencies | <5 | currently only `@clack/prompts` needed |
| Successful scaffold rate | >95% | measured via telemetry (opt-in) |
| GHCR image size (agentra server) | <50 MB | alpine-based, CGO_ENABLED=0 |
| GHCR image size (agentra-web) | <200 MB | Next.js standalone output |

---

## 8. Open Questions

1. **Should `create-agentra` live in the monorepo or a separate repo?**
   - Recommendation: monorepo (`packages/create-agentra/`). Keeps versioning aligned, same CI, same release cadence. create-t3-app lives in a separate repo; create-next-app lives in the Next.js monorepo. Agentra is closer to the Next.js pattern — a platform with a scaffolder.

2. **Should `agentra init` share code with `create-agentra` or be independent?**
   - Recommendation: `agentra init` generates the same files using Go's `text/template`. Do NOT try to share code between Node.js and Go — the template logic is simple enough to maintain in two places. The `.env` and `docker-compose.yml` templates are ~100 lines each.

3. **Do we need a "quickstart" Docker image with everything bundled?**
   - Recommendation: No for v1. The Docker Compose approach is production-correct. A single-container image would encourage bad deployment patterns. Revisit if user feedback demands it.

4. **Should `create-agentra` auto-start after scaffolding?**
   - Recommendation: Yes, with an opt-out flag (`--no-start`). The "wow" moment is seeing Agentra running immediately. Users who want to inspect files first can pass `--no-start`.

5. **How do we handle the `RESEND_API_KEY` requirement for magic-link auth?**
   - Recommendation: The scaffolder warns that email auth requires a Resend API key but does NOT block startup. The server starts in a "degraded auth" mode where it logs magic-link tokens to stdout (for local dev only). Production users must set `RESEND_API_KEY`. This follows the pattern of tools that print setup warnings rather than refusing to start.

---

## 9. Appendix: Generated README Template

The scaffolder generates a project-specific README.md:

```markdown
# Acme Corp — Agentra Workspace

Generated by `create-agentra` on 2026-05-10.

## Services

| Service | URL | Description |
|---------|-----|-------------|
| Frontend | http://localhost:3000 | Web application |
| API | http://localhost:8080 | Backend REST + WebSocket API |
| Health | http://localhost:8080/health | Health check endpoint |

## Quick Start

docker compose up -d       # Start all services
docker compose ps           # Check status
docker compose logs -f      # Watch logs
docker compose down         # Stop all services

## Configuration

Edit `.env` to change configuration. See `.env.example` for all available options.

## CLI Setup

Install the Agentra CLI to run AI agents on your machine:

brew install agentra-ai/tap/agentra
agentra login
agentra daemon start

## Upgrading

docker compose pull          # Pull latest images
docker compose up -d         # Restart with new images

## Data

Data is stored in Docker volumes:
- `pgdata` — PostgreSQL database
- `miniodata` — File attachments (if enabled)

Back up these volumes to persist your data.
```

---

## 10. Appendix: Comparison Matrix

| Platform | Install Command | Time to First Use | Dependencies | Components Started | Auth |
|----------|----------------|-------------------|-------------|-------------------|------|
| **swarmclaw** | `npm i -g @swarmclawai/swarmclaw && swarmclaw` | ~30s | 73 npm | Single Node.js process (SQLite) | ACCESS_KEY |
| **agent-tasks** | `npm i -g agent-tasks` (MCP config) | ~15s | 4 npm | Single Node.js process (SQLite) | None (local) |
| **open-multi-agent** | `npm i @open-multi-agent/core` | ~10s | 3 npm | Library import (no server) | API keys in code |
| **Agentra (proposed)** | `npx create-agentra@latest` | <2min | 0 (pre-req: Node.js, Docker) | PostgreSQL + server + web + (optional: gateway, minio) | Magic link + OAuth |
