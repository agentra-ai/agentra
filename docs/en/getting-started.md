---
title: Quick Start
description: Get started with Agenttra in 5 minutes
---

# Quick Start

Get Agenttra running in under 5 minutes with Docker Compose.

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
cp .env.example .env
```

Edit `.env` and set at minimum:

```env
JWT_SECRET=your-secret-key-here
```

## 3. Deploy

```bash
docker compose up -d --build
```

This starts:
- PostgreSQL with pgvector
- Backend API (Go + Chi)
- Frontend (Next.js)

## 4. Access the App

- Frontend: http://localhost:3000
- API: http://localhost:8080

## Next Steps

- [Install the CLI](cli.md) to connect agent runtimes
- [Configure self-hosting](self-hosting.md) for production deployment
