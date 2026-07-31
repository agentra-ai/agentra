---
title: Self-Hosting
description: Deploy Agentra for production self-hosting
---

# Self-Hosting Guide

Complete guide to deploying Agentra in production.

## Environment Variables

See `.env.example` for all configuration options.

### Required

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | Secret key for JWT token signing |
| `DATABASE_URL` | PostgreSQL connection string |
| `POSTGRES_PASSWORD` | Password used by the bundled PostgreSQL service |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | Credentials used by bundled object storage |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend API port |
| `FRONTEND_PORT` | `3000` | Frontend port |
| `FRONTEND_ORIGIN` | `http://127.0.0.1:3000` | Frontend origin for CORS |

## Docker Compose

Generate a secure environment and start the bundled stack:

```bash
./scripts/bootstrap-env.sh
docker compose up -d --build
```

The bootstrap writes random PostgreSQL/JWT/MinIO secrets to an owner-only `.env` and refuses to overwrite it. The default stack publishes backend and frontend on loopback only. PostgreSQL and MinIO stay on the Compose network; Adminer (`debug`) and the Docker-socket gateway (`cloud-runtime`) require explicit profiles.

Use `make env-check` to audit an environment file. Never overwrite an older deployment's `.env` without coordinated credential rotation: PostgreSQL only applies `POSTGRES_PASSWORD` when initializing a new data volume.

## Database

Agentra requires PostgreSQL 17 with pgvector extension:

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

## Reverse Proxy

For production, run behind nginx or Traefik with SSL termination.

## Health Probes

- `GET /livez` checks only that the server process can answer HTTP requests. Use it for liveness probes.
- `GET /readyz` checks PostgreSQL, the latest embedded migration, configured object storage, and the scheduler. It returns HTTP 503 until the server can safely receive traffic.
- `GET /health` remains the lightweight endpoint used by the Agentra CLI.

After configuring the CLI, run `agentra doctor`. It combines these probes with authentication, workspace membership, runtime CLI, filesystem, Git origin, local daemon, Web UI, and authenticated WebSocket checks. Use `agentra doctor --output json` in support bundles or automation; a required failure returns a non-zero exit code.
