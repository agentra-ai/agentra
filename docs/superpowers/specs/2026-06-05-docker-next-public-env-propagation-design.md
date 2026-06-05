# Docker Build `NEXT_PUBLIC_*` Env Propagation 设计规格

**日期**: 2026-06-05
**状态**: Draft

---

## 1. 概述

修复 Docker 部署下 web 镜像构建时 `NEXT_PUBLIC_*` 环境变量丢失的问题。

`apps/web` 在 Next.js 客户端运行时会读取 4 个 `NEXT_PUBLIC_*` 变量（来自仓库根 `.env`）：
- `NEXT_PUBLIC_SITE_URL`
- `NEXT_PUBLIC_API_URL`
- `NEXT_PUBLIC_WS_URL`
- `NEXT_PUBLIC_CLI_CALLBACK_HOSTS`

Next.js 把 `NEXT_PUBLIC_*` 在 `next build` 期间 inline 到 client bundle。当前 `Dockerfile` 的 `web-builder` 阶段没有把这些变量传进来，结果是 client bundle 中这 4 个值都是 `undefined`。

最直观也最容易被踩中的症状：`apps/web/shared/env.ts` 的 `getCliCallbackHosts()` 返回 `[]`，登录页 `validateCliCallback()` 拒绝所有 `cli_callback` URL，agentra CLI 走登录时浏览器出现"无效的回调地址"。

### 1.1 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 传播方式 | `COPY .env apps/web/.env` 进 web-builder 阶段 | 沿用 Next.js 自动加载 `.env` 的默认行为，改动最小。必须把文件放在 Next.js 项目根（`apps/web/`），而不是 WORKDIR（`/src`），因为 `loadEnvConfig` 不会沿目录树上溯 |
| 卫生边界 | `.env` 只进 builder 中间层，不进 `web-runtime` 镜像 | `web-runtime` 只 `COPY --from=web-builder` 3 个白名单路径；client bundle 只 inline `NEXT_PUBLIC_*` 前缀 |
| 范围 | 4 个 `NEXT_PUBLIC_*` 全部修 | 4 个同根问题一起处理，不只修 callback |
| 主机 CLI | 不在 spec 改动范围 | 用户确认 CLI 从主机跑；主机侧走自己的 env，不依赖 Docker `.env` |

### 1.2 不在范围内

- 改 callback 架构（web 代理、容器内 CLI 改造、polling 替代 redirect 等）
- 改 `docker-compose.yml` 中任何 service 的 `environment:` 段
- 改 `agentra` CLI 流程或默认值
- 引入 BuildKit `--mount=type=secret` 方案（卫生更严但增加维护面，YAGNI）
- 加 build 期断言脚本（防止未来回归，但本 spec 范围内不做）
- 修 SSR 端 `process.env.NEXT_PUBLIC_*` 读取（仍为 `undefined`，与改动前一致；如需后续可加 `environment:` 段）

---

## 2. 文件改动

### 2.1 `.dockerignore`

删除第 8 行：

```
.env
```

**理由**：当前 `.env` 被显式排除在 build context 外。`Dockerfile` 里 `COPY .env apps/web/.env` 在不删这一行的情况下不会生效（Docker 不会 COPY 被 `.dockerignore` 排除的文件）。

**风险评估**：低。`Dockerfile` 里所有 `COPY` 都使用具体文件/目录路径，**无**通配符（没有 `COPY . .` 这类指令）。`.env` 不会进任何未被显式引用的镜像层。

### 2.2 `Dockerfile`（`web-builder` 阶段）

在 `COPY apps/web/ ./apps/web/` 之后新增一行：

```dockerfile
# Propagate .env so Next.js can inline NEXT_PUBLIC_* into the client bundle.
# Next.js's loadEnvConfig reads .env from the Next.js project root
# (where next.config.ts lives, i.e. apps/web/), not from cwd, and does
# not walk up the directory tree. The web-builder WORKDIR is /src, but
# `pnpm --filter @agentra/web build` changes cwd to apps/web/, so the
# file must land at apps/web/.env.
COPY .env apps/web/.env
```

**改动后的 `web-builder` 段**（参考结构）：

```dockerfile
FROM node:22-alpine AS web-builder

RUN apk add --no-cache libc6-compat

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
ENV NEXT_TELEMETRY_DISABLED=1

RUN corepack enable

WORKDIR /src

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY apps/web/package.json ./apps/web/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web/ ./apps/web/

# Propagate .env so Next.js can inline NEXT_PUBLIC_* into the client bundle.
# Next.js's loadEnvConfig reads .env from the Next.js project root
# (where next.config.ts lives, i.e. apps/web/), not from cwd, and does
# not walk up the directory tree. The web-builder WORKDIR is /src, but
# `pnpm --filter @agentra/web build` changes cwd to apps/web/, so the
# file must land at apps/web/.env.
COPY .env apps/web/.env

ARG REMOTE_API_URL=http://server:8080
ENV REMOTE_API_URL=${REMOTE_API_URL}

RUN pnpm --filter @agentra/web build
```

**效果**：
- `web-builder` 阶段内 `.env` 落在 Next.js 项目根 `/src/apps/web/.env`（**不是** `/src/.env`）
- `pnpm --filter @agentra/web build` 触发 `next build`，Next.js 的 `loadEnvConfig` 从 Next.js 项目根（即 `apps/web/`）加载 `.env`
- 4 个 `NEXT_PUBLIC_*` 被 inline 到 client bundle
- 其他变量（`JWT_SECRET`、`POSTGRES_PASSWORD`、`MINIO_SECRET_KEY`）被加载到 server-side 构建上下文，**不会**被 inline 到 client bundle

### 2.3 不改 `docker-compose.yml`

不需要改 `web` service 的 `args:` 或 `environment:`。`COPY .env apps/web/.env` 由 Dockerfile 自身完成。

---

## 3. 卫生边界

| 位置 | 是否含 `.env` 内容 | 备注 |
|------|-------------------|------|
| `web-builder` 中间层 | 是（builder 镜像内 `/src/apps/web/.env`） | 临时层，CI 本地生成，不分发 |
| `web-runtime` 镜像 | 否 | 仅 `COPY --from=web-builder` 3 个白名单路径 |
| client bundle (JS) | 仅 `NEXT_PUBLIC_*` | Next.js inline 规则只处理此前缀 |
| server bundle (SSR) | `.env` 文件不在 runtime image 中 | SSR 读 `process.env.NEXT_PUBLIC_*` 仍为 `undefined`，与改动前一致 |

**SSR 端说明**：本 spec 修复的是 client bundle inline 问题，不修 SSR 端。SSR 端如果需要 `NEXT_PUBLIC_*` 走环境变量，后续在 `docker-compose.yml` 给 `web` service 加 `environment:` 段即可——但这是另一个改动，本次不做。

---

## 4. 验证

### 4.1 构建验证

```bash
docker compose build web
```

预期：构建成功，无 `COPY` 失败报错。

### 4.2 bundle 内联值验证

```bash
docker run --rm agentra-web:latest sh -c \
  'grep -r "localhost,127.0.0.1" /app/apps/web/.next/static 2>/dev/null | head -3'
```

预期：能 grep 到 `localhost,127.0.0.1`（即 `NEXT_PUBLIC_CLI_CALLBACK_HOSTS` 的默认内联值）。

### 4.3 功能验证（callback 流程）

**路径 A：CLI 从主机跑**（用户实际操作方式）

```bash
make build                                       # 生成 server/bin/agentra
AGENTRA_APP_URL=http://web.agentra.orb.local \
  ./server/bin/agentra login
```

预期：浏览器自动打开 `http://web.agentra.orb.local/login?cli_callback=...&cli_state=...`；输入邮箱 → dev code → 进入"授权 CLI"或直接完成；不再出现"无效的回调地址"。

**路径 B：浏览器手测**

1. 浏览器打开
   `http://web.agentra.orb.local/login?cli_callback=http%3A%2F%2Flocalhost%3A54321%2Fcallback&cli_state=test`
2. 输入邮箱（任意），收 dev code
3. 输入 dev code（或 master code `888888`）
4. **预期**：跳转 `http://localhost:54321/callback?token=...&state=test`，**不**再出现"无效的回调地址"

**注意**：路径 B 的最后一步浏览器会尝试访问 `http://localhost:54321/callback`；如果 CLI 没有在主机监听 54321 端口，会得到连接失败——但这是 CLI 缺失，不是本次修复的失败信号。`validateCliCallback` 不再报错就证明修复生效。

### 4.4 其他 `NEXT_PUBLIC_*` 回归验证

浏览器 DevTools → Network 面板：

- 任意 fetch 请求应包含 `NEXT_PUBLIC_API_URL` 拼出来的 origin（即 `http://server.agentra.orb.local`）
- WebSocket 握手目标应匹配 `NEXT_PUBLIC_WS_URL`（即 `ws://server.agentra.orb.local/ws`）
- 页面 DOM 中链接的 origin 应匹配 `NEXT_PUBLIC_SITE_URL`

如果以上 origin 都不是 `undefined` 或默认占位，验证通过。

---

## 5. 回滚

```bash
git revert <commit-sha>
docker compose build web
docker compose up -d web
```

效果：
- `.dockerignore` 恢复排除 `.env`
- `Dockerfile` 移除 `COPY .env apps/web/.env`
- 回到改动前状态：4 个 `NEXT_PUBLIC_*` 在 client bundle 中都是 `undefined`，`validateCliCallback` 拒绝所有 callback URL

---

## 6. 风险与权衡

| 风险 | 等级 | 缓解 |
|------|------|------|
| `.env` 进 builder 镜像层 | 低 | builder 是临时层，不分发；runtime 镜像不引用 `.env` |
| secrets 误 inline 进 client bundle | 极低 | Next.js inline 规则只处理 `NEXT_PUBLIC_*` 前缀 |
| 未来某次 `COPY` 改成通配符导致 `.env` 泄漏 | 极低 | PR review 留意；本 spec 不引入 |
| 主机 `.env` 与 Docker `.env` 不一致引发困惑 | 低 | 文档化两条路径；本 spec 不引入新耦合 |

---

## 7. 未来工作（不在本 spec）

- 主机侧 `agentra login` 流程文档化（`docs/` howto）
- Build 期断言脚本：缺任一 `NEXT_PUBLIC_*` 即构建失败
- 评估 BuildKit `--mount=type=secret` 替代方案（卫生更严）
- 修 SSR 端：给 `web` service 在 `docker-compose.yml` 加 `environment:` 段
