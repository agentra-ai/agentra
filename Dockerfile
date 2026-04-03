# --- Server build stage ---
FROM golang:1.26-alpine AS server-builder

RUN apk add --no-cache git

WORKDIR /src

COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

COPY server/ ./server/

ARG VERSION=dev
ARG COMMIT=unknown

RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/server ./cmd/server
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o bin/agentra ./cmd/agentra
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/migrate ./cmd/migrate

# --- Frontend build stage ---
FROM node:22-alpine AS web-builder

RUN apk add --no-cache libc6-compat

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
ENV NEXT_TELEMETRY_DISABLED=1

RUN corepack enable

WORKDIR /src

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY apps/web/package.json ./apps/web/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web/ ./apps/web/

ARG REMOTE_API_URL=http://server:8080
ENV REMOTE_API_URL=${REMOTE_API_URL}

RUN pnpm --filter @agentra/web build

# --- Server runtime stage ---
FROM alpine:3.21 AS server-runtime

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=server-builder /src/server/bin/server ./
COPY --from=server-builder /src/server/bin/agentra ./
COPY --from=server-builder /src/server/bin/migrate ./
COPY server/migrations/ ./migrations/

EXPOSE 8080

ENTRYPOINT ["./server"]

# --- Frontend runtime stage ---
FROM node:22-alpine AS web-runtime

RUN apk add --no-cache libc6-compat

WORKDIR /app

ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=web-builder /src/apps/web/.next/standalone ./
COPY --from=web-builder /src/apps/web/.next/static ./apps/web/.next/static
COPY --from=web-builder /src/apps/web/public ./apps/web/public

EXPOSE 3000

CMD ["node", "apps/web/server.js"]
