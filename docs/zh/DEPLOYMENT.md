# 部署指南 — Agentra (中文)

> 如何将 Agentra 交付到生产环境: **自动化 CI/CD** (push 一个 tag → Docker Hub 自动收货) 和 **手动自托管** (在 VM 上 `docker compose up`)。

- [自动化: GitHub Actions → Docker Hub](#自动化-github-actions--docker-hub)
- [手动: Docker Compose 自托管](#手动-docker-compose-自托管)
- [密钥参考](#密钥参考)
- [发版日历](#发版日历)

---

## 自动化: GitHub Actions → Docker Hub

**一句话总结** → `git tag v0.5.0 && git push origin v0.5.0`,等 ~3 分钟,镜像自动推送到 `docker.io/dougzeng/agentra`。

### 端到端流程

```
本地机器                     GitHub Actions                          Docker Hub
───────────────────────────  ─────────────────────────────────  ─────────────────────────
                            │
git tag v0.5.0               │
git push origin v0.5.0  ──→  │  .github/workflows/docker.yml
                            │      ├─ checkout 代码
                            │      ├─ cp .env.example .env
                            │      ├─ docker login dio dougzeng
                            │      ├─ docker buildx build server  ──→  dougzeng/agentra:server-v0.5.0
                            │      ├─ docker buildx build gateway ──→  dougzeng/agentra:gateway-v0.5.0
                            │      ├─ docker buildx build web     ──→  dougzeng/agentra:web-v0.5.0
                            │      └─ done
```

### 步骤 (一次性设置)

#### 1. 创建 Docker Hub access token

1. 登录 https://hub.docker.com
2. Account Settings → Security → New Access Token
3. 名称: `github-actions-agentra`
4. 权限: **Read & Write**
5. 复制 token (以 `dckr_pat_` 开头)

#### 2. 配置 GitHub 仓库 secret

在 GitHub 仓库:

```
Settings → Secrets and variables → Actions → New repository secret
```

| 名称 | 值 |
|---|---|
| `DOCKER_USERNAME` | `dougzeng` |
| `DOCKERHUB_TOKEN` | 步骤 1 生成的 token |

#### 3. 触发 workflow

```bash
cd agentra
# 确保所有改动已提交并推送
git push origin main

# tag 发布版本 (默认补丁版本递增)
git tag v0.5.0

# 推送 tag — 触发 docker.yml
git push origin v0.5.0
```

等待 2-3 分钟,监控:

```bash
gh run list --repo agentra-ai/agentra --workflow "Docker Hub"
# 或访问 https://github.com/agentra-ai/agentra/actions
```

#### 4. 在任何地方拉取部署

```bash
# 任意 Docker 宿主机:
docker pull dougzeng/agentra:server-v0.5.0
docker pull dougzeng/agentra:web-v0.5.0
docker pull dougzeng/agentra:gateway-v0.5.0
```

---

## 手动: Docker Compose 自托管

**一句话总结** → clone → `.env` → `docker compose up -d`,开放 3000 端口。

### 前置条件

- Docker 24+ 与 Docker Compose v2
- 最低 4 GB RAM (跑 agent 推荐 8 GB)
- Linux/macOS 宿主机或 VM

### 步骤

```
                    ┌─────────────────────────────────────────────────────────────────────────┐
                    │ .env 密钥                                                            │
                    │  用户生成,不入库                                                      │
                    └────┬──────────────┬──────────────┬──────────────┬───────────────┘
                         │              │              │              │
    ┌────────────────────▼──┐     ┌─────▼────────┐   ┌─▼──────────┐ ┌─▼─────────────┐
    │ postgres (pg17+        │     │ server       │   │ gateway     │ │ web           │
    │   pgvector)            │     │ :8080        │   │ :8081       │ │ :3000         │
    │ :5432                  │     │ Go           │   │ Go          │ │ Next.js 16    │
    └────────────────────────┘     └──────────────┘   └─────────────┘ └───────────────┘
          ▲                              ▲                 ▲
          │                              │                 │
    ┌─────┴────────────────┐    ┌────────┴─────────────────┐
    │ migrate              │    │ agent CLI (容器外部)       │
    │ (一次性 Job)          │    │ `agentra daemon start`   │
    └──────────────────────┘    └─────────────────────────────
```

#### 1. 克隆 + 配置密钥

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra

# 复制模板
cp .env.example .env

# 编辑关键行:
#   JWT_SECRET              → openssl rand -hex 32
#   POSTGRES_PASSWORD       → 任意强密码
#   RESEND_API_KEY          → 可选 (邮件 OTP)
nano .env
```

#### 2. 拉起完整栈

```bash
# 从源码构建 (首次 ~5 分钟)
docker compose up -d --build

# 观察日志
docker compose logs -f server web
```

如果使用 **Docker Hub 预构建镜像**,跳过 `--build`:

```bash
# 拉取最新发布的镜像
docker pull dougzeng/agentra:server-latest
docker pull dougzeng/agentra:web-latest
docker pull dougzeng/agentra:gateway-latest

# docker-compose.yml 里将 "build: ..." 替换为 "image: dougzeng/agentra:server-latest" 等
docker compose up -d   # 不需要 --build,自动拉取预构建镜像
```

#### 3. 验证

```bash
# Server 健康检查
curl http://localhost:8080/health
# 预期: {"status":"ok"}

# Web 端
open http://localhost:3000
# 预期: 登录页

# 数据库已 migrate 到最新
docker compose run --rm migrate
# 预期: "skip 039_agent_task_metrics (already applied)" ... "Done."
```

#### 4. 在宿主机启动 daemon (Mac/Linux,**不在容器里**)

```bash
# 构建 CLI
make build
sudo cp server/bin/agentra /usr/local/bin/agentra

# 认证 (创建 PAT + session)
agentra login

# 后台启动 daemon
agentra daemon start

# 在 Web 端验证 runtime 是否在线
# Settings → Runtimes → 你的机器应显示 "online"
```

#### 5. (可选) 手动跑 migration

```bash
# 需要更新 schema 时:
docker compose run --rm migrate up

# 回滚一步:
docker compose run --rm migrate down
```

---

## 密钥参考

| 密钥 | 位置 | 生成方式 |
|---|---|---|
| `JWT_SECRET` | `server/` | `openssl rand -hex 32` |
| `POSTGRES_PASSWORD` | `postgres/` | 任意强密码 |
| `RESEND_API_KEY` | 邮件 OTP | resend.com → API Keys |
| `GOOGLE_CLIENT_*` | OAuth (可选) | console.cloud.google.com |
| `STORAGE_DRIVER` | `minio` (默认) 或 `s3` | — |
| `DOCKER_USERNAME` | GitHub Actions secret | Docker Hub 用户名 |
| `DOCKERHUB_TOKEN` | GitHub Actions secret | hub.docker.com → Settings → Security |

---

## 发版日历

按 CLAUDE.md 策略:**每次发版提升补丁版本**,除非有重大功能升级。

| 时机 | Tag | 示例 |
|---|---|---|
| Bug 修复 | 补丁 | `v0.4.3` → `v0.4.4` |
| 新功能 | 次版本 | `v0.4.x` → `v0.5.0` |
| CLI 发版必须伴随每次 Production 部署 | — | `v0.5.0` 同时触发 `release.yml` (CLI 二进制) 和 `docker.yml` (镜像) |

两个 job 从同一 tag push 并行触发。CLI 二进制发布到 GitHub Releases;容器镜像发布到 Docker Hub。

---

*English version: [Deployment Guide](../DEPLOYMENT.md)*
