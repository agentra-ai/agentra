---
title: 自托管部署
description: 生产环境自托管 Agentra
---

# 自托管指南

生产环境部署 Agentra 的完整指南。

## 环境变量

参考 `.env.example` 查看所有配置选项。

### 必需

| 变量 | 描述 |
|------|------|
| `JWT_SECRET` | JWT 签名密钥 |
| `DATABASE_URL` | PostgreSQL 连接字符串 |
| `POSTGRES_PASSWORD` | 内置 PostgreSQL 服务密码 |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | 内置对象存储凭据 |

### 可选

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `PORT` | `8080` | 后端 API 端口 |
| `FRONTEND_PORT` | `3000` | 前端端口 |
| `FRONTEND_ORIGIN` | `http://127.0.0.1:3000` | 前端 CORS 源 |

## Docker Compose

生成安全环境并启动内置栈：

```bash
./scripts/bootstrap-env.sh
docker compose up -d --build
```

bootstrap 将 PostgreSQL/JWT/MinIO 随机密钥写入仅所有者可访问的 `.env`，并拒绝覆盖已有文件。默认栈只在 loopback 发布前后端；PostgreSQL 与 MinIO 只留在 Compose 网络，Adminer（`debug`）必须显式启用 profile。

可用 `make env-check` 审计环境文件。不要在未协同轮换凭据时覆盖旧部署的 `.env`：PostgreSQL 只在初始化新 data volume 时应用 `POSTGRES_PASSWORD`。

## 数据库

Agentra 需要 PostgreSQL 17 + pgvector 扩展：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

## 反向代理

生产环境建议使用 nginx 或 Traefik 配合 SSL 终端。

## 健康探针

- `GET /livez` 仅检查服务进程是否能响应 HTTP，用于存活探针。
- `GET /readyz` 检查 PostgreSQL、二进制内嵌的最新迁移、已配置对象存储和调度器；服务可安全接收流量前返回 HTTP 503。
- `GET /health` 保留为 Agentra CLI 使用的轻量检查。

配置 CLI 后运行 `agentra doctor`。它会把上述探针与认证、workspace 成员资格、Runtime CLI、文件系统、Git origin、本地 daemon、Web UI 和带认证的 WebSocket 检查组合起来。支持工单或自动化可使用 `agentra doctor --output json`；必需检查失败时返回非零退出码。
