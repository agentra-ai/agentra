ARG VERSION=dev
ARG COMMIT=unknown

# --- Server build stage ---
FROM golang:1.26-alpine AS server-builder

ARG VERSION
ARG COMMIT

RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/main" > /etc/apk/repositories && \
    echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache git

WORKDIR /src

COPY server/ ./server/
RUN cd server && go mod download

RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/agentra-ai/agentra/server/internal/buildinfo.Version=${VERSION} -X github.com/agentra-ai/agentra/server/internal/buildinfo.Commit=${COMMIT}" -o bin/server ./cmd/server
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/agentra-ai/agentra/server/internal/buildinfo.Version=${VERSION} -X github.com/agentra-ai/agentra/server/internal/buildinfo.Commit=${COMMIT}" -o bin/agentra ./cmd/agentra
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/migrate ./cmd/migrate

# --- Frontend build stage ---
FROM node:22-alpine AS web-builder

ARG VERSION
ARG COMMIT

RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/main" > /etc/apk/repositories && \
    echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache libc6-compat

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
ENV NEXT_TELEMETRY_DISABLED=1
ENV NEXT_PUBLIC_AGENTRA_VERSION=${VERSION}
ENV NEXT_PUBLIC_AGENTRA_COMMIT=${COMMIT}

RUN corepack enable

WORKDIR /src

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY apps/web/package.json ./apps/web/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web/ ./apps/web/

# Propagate .env so Next.js can inline NEXT_PUBLIC_* into the client bundle.
# Next.js's loadEnvConfig reads .env from the Next.js project root
# (where next.config.ts lives, i.e. apps/web/), not from cwd, and does
# not walk up the directory tree. The web-builder WORKDIR is /src, but
# `pnpm --filter @agentra/web build` changes cwd to apps/web/, so the
# file must land at apps/web/.env.
COPY .env apps/web/.env

ARG REMOTE_API_URL=http://server:8080
ARG NEXT_PUBLIC_CLI_CALLBACK_HOSTS=localhost,127.0.0.1
ENV REMOTE_API_URL=${REMOTE_API_URL}
ENV NEXT_PUBLIC_CLI_CALLBACK_HOSTS=${NEXT_PUBLIC_CLI_CALLBACK_HOSTS}

RUN pnpm --filter @agentra/web build

# --- Server runtime stage ---
FROM golang:1.26-alpine AS server-runtime

RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/main" > /etc/apk/repositories && \
    echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=server-builder /src/server/bin/server ./
COPY --from=server-builder /src/server/bin/agentra ./
COPY --from=server-builder /src/server/bin/migrate ./
COPY server/migrations/ ./migrations/

EXPOSE 8080

ENTRYPOINT ["./server"]

# --- Frontend runtime stage ---
FROM node:22-alpine AS web-runtime

RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/main" > /etc/apk/repositories && \
    echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache libc6-compat

WORKDIR /app

ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=web-builder /src/apps/web/.next/standalone ./
COPY --from=web-builder /src/apps/web/.next/static ./apps/web/.next/static
COPY --from=web-builder /src/apps/web/public ./apps/web/public

# Remove .env that next build's file-tracing bundled into .next/standalone.
# The build needs .env to inline NEXT_PUBLIC_* into the client bundle,
# but the runtime doesn't — and .env contains secrets we don't want shipped.
RUN rm -f /app/apps/web/.env

EXPOSE 3000

CMD ["node", "apps/web/server.js"]
