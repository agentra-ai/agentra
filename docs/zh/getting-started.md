---
title: 快速入门
description: 5 分钟内启动 Agenttra
---

# 快速入门

5 分钟内让 Agenttra 运行起来。

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
cp .env.example .env
```

编辑 `.env`，至少设置：

```env
JWT_SECRET=your-secret-key-here
```

## 3. 部署

```bash
docker compose up -d --build
```

这将启动：
- PostgreSQL + pgvector
- 后端 API (Go + Chi)
- 前端 (Next.js)

## 4. 访问应用

- 前端：http://localhost:3000
- API：http://localhost:8080

## 下一步

- [安装 CLI](cli.md) 连接 Agent 运行时
- [配置生产环境](self-hosting.md)
