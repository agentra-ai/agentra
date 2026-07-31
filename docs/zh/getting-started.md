---
title: 快速入门
description: 5 分钟内启动 Agentra
---

# 快速入门

5 分钟内让 Agentra 运行起来。

## 前置要求

- Docker 和 Docker Compose
- Git

## 1. 克隆仓库

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
```

## 2. 配置环境

```bash
./scripts/bootstrap-env.sh
```

该命令只创建一次 `.env`，分别生成 PostgreSQL/JWT/MinIO 随机密钥，将权限设为 `0600`，且不会打印密钥。它拒绝覆盖已有文件；可用 `./scripts/bootstrap-env.sh --check .env` 校验。

不要直接替换已有数据库 volume 使用的 `.env`：PostgreSQL 只在首次初始化时读取 bootstrap 密码。已有部署应按自托管升级指南协同轮换凭据。

## 3. 部署

```bash
docker compose up -d --build
```

这将启动：
- PostgreSQL + pgvector
- MinIO 对象存储
- 一次性 migration job
- 后端 API (Go + Chi)
- 前端 (Next.js)

前后端默认绑定 `127.0.0.1`。默认 profile 不发布 PostgreSQL、MinIO、Adminer 或挂载 Docker socket 的 gateway 端口。

## 4. 访问应用

- 前端：http://127.0.0.1:3000
- API：http://127.0.0.1:8080

## 下一步

- [安装 CLI](cli.md) 连接 Agent 运行时
- [配置生产环境](self-hosting.md)
