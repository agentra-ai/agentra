---
title: 自托管部署
description: 生产环境自托管 Agenttra
---

# 自托管指南

生产环境部署 Agenttra 的完整指南。

## 环境变量

参考 `.env.example` 查看所有配置选项。

### 必需

| 变量 | 描述 |
|------|------|
| `JWT_SECRET` | JWT 签名密钥 |
| `DATABASE_URL` | PostgreSQL 连接字符串 |

### 可选

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `PORT` | `8080` | 后端 API 端口 |
| `FRONTEND_PORT` | `3000` | 前端端口 |
| `FRONTEND_ORIGIN` | `http://localhost:3000` | 前端 CORS 源 |

## Docker Compose

生产环境使用外部 PostgreSQL：

```yaml
services:
  backend:
    image: agentra/backend
    environment:
      - DATABASE_URL=postgresql://user:pass@host:5432/agentra
      - JWT_SECRET=${JWT_SECRET}
    ports:
      - "8080:8080"

  frontend:
    image: agentra/frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://backend:8080
      - NEXT_PUBLIC_WS_URL=ws://backend:8080
```

## 数据库

Agenttra 需要 PostgreSQL 17 + pgvector 扩展：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

## 反向代理

生产环境建议使用 nginx 或 Traefik 配合 SSL 终端。
