---
title: Quick Start
description: Get started with Agentra in 5 minutes
---

# Quick Start

Get Agentra running in under 5 minutes with Docker Compose.

## Prerequisites

- Docker and Docker Compose
- Git

## 1. Clone the Repository

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
```

## 2. Configure Environment

```bash
./scripts/bootstrap-env.sh
```

The command creates `.env` once, generates independent PostgreSQL/JWT/MinIO secrets, sets mode `0600`, and never prints the generated values. It refuses to overwrite an existing file; validate one with `./scripts/bootstrap-env.sh --check .env`.

Do not replace `.env` on an existing database volume: PostgreSQL only consumes its bootstrap password on first initialization. Follow the self-hosting upgrade guidance for coordinated credential rotation.

## 3. Deploy

```bash
docker compose up -d --build
```

This starts:
- PostgreSQL with pgvector
- MinIO object storage
- A one-shot migration job
- Backend API (Go + Chi)
- Frontend (Next.js)

Backend and frontend bind to `127.0.0.1` by default. PostgreSQL, MinIO, Adminer, and the Docker-socket gateway do not publish host ports in the default profile.

## 4. Access the App

- Frontend: http://127.0.0.1:3000
- API: http://127.0.0.1:8080

## Next Steps

- [Install the CLI](cli.md) to connect agent runtimes
- [Configure self-hosting](self-hosting.md) for production deployment
