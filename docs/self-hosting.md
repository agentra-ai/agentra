---
title: Self-Hosting
description: Deploy Agenttra for production self-hosting
---

# Self-Hosting Guide

Complete guide to deploying Agenttra in production.

## Environment Variables

See `.env.example` for all configuration options.

### Required

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | Secret key for JWT token signing |
| `DATABASE_URL` | PostgreSQL connection string |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend API port |
| `FRONTEND_PORT` | `3000` | Frontend port |
| `FRONTEND_ORIGIN` | `http://localhost:3000` | Frontend origin for CORS |

## Docker Compose

For production, use external PostgreSQL:

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

## Database

Agenttra requires PostgreSQL 17 with pgvector extension:

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

## Reverse Proxy

For production, run behind nginx or Traefik with SSL termination.
