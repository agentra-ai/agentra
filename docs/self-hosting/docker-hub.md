# Docker Hub Images

Multi-arch container images are published when you push a `v*` tag:

```
dougzeng/agentra:server-v0.4.1      # backend API + REST + WS
dougzeng/agentra:server-v0.4        # minor-pin alias
dougzeng/agentra:gateway-v0.4.1     # carrier-grade agent runtime
dougzeng/agentra:web-v0.4.1         # Next.js standalone frontend
```

## Architecture

```
         ┌─────────────────────────┐
         │   dougzeng/agentra:web  │ :3000
         │   Next.js 16 standalone  │
         └────────────┬────────────┘
                      │ REST + WS
         ┌────────────▼────────────┐
         │  dougzeng/agentra:server │ :8080
         │  Go Chi + sqlc + pgvector │
         └───┬───────────────┬──────┘
             │               │
    ┌────────▼────────┐  ┌──▼─────────────────────┐
    │ postgres:17     │  │ dougzeng/agentra:gateway│ :8081
    │ pgvector        │  │ Local agent execution   │
    └─────────────────┘  │ (mount /var/run/docker.sock) │
                         └──────────────────────────┘
```

## Deploy from Docker Hub

```bash
# 1. Pulls
docker pull dougzeng/agentra:server-v0.4.1
docker pull dougzeng/agentra:web-v0.4.1
docker pull dougzeng/agentra:gateway-v0.4.1

# 2. .env
cp .env.example .env
sed -i '' 's/JWT_SECRET=.*/JWT_SECRET='$(openssl rand -hex 32)'/' .env

# 3. Run
docker compose up -d --no-build
```

## CI source

Tagged in: `.github/workflows/docker.yml`

```yaml
on:
  push:
    tags: ["v*"]
```

## Versioning

Each release bumps the **patch version** per project convention:
`v0.4.12 → v0.4.13`.

See the full [Release History](../releases.md).
