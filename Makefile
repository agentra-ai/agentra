.PHONY: dev daemon cli agentra build test migrate-up migrate-down sqlc seed clean bootstrap-env env-check ensure-env setup start stop check worktree-env setup-main start-main stop-main check-main setup-worktree start-worktree stop-worktree check-worktree db-up db-down

MAIN_ENV_FILE ?= .env
WORKTREE_ENV_FILE ?= .env.worktree
ENV_FILE ?= $(if $(wildcard $(MAIN_ENV_FILE)),$(MAIN_ENV_FILE),$(if $(wildcard $(WORKTREE_ENV_FILE)),$(WORKTREE_ENV_FILE),$(MAIN_ENV_FILE)))

ifneq ($(wildcard $(ENV_FILE)),)
include $(ENV_FILE)
endif

POSTGRES_DB ?= agentra
POSTGRES_USER ?= agentra
POSTGRES_PORT ?= 5432
PORT ?= 8080
FRONTEND_PORT ?= 3000
FRONTEND_ORIGIN ?=
AGENTRA_APP_URL ?= $(FRONTEND_ORIGIN)
DATABASE_URL ?=
NEXT_PUBLIC_API_URL ?=
NEXT_PUBLIC_WS_URL ?=
GOOGLE_REDIRECT_URI ?= $(FRONTEND_ORIGIN)/auth/callback
AGENTRA_SERVER_URL ?=

export

AGENTRA_ARGS ?= $(ARGS)

COMPOSE_DEV := docker compose -f docker-compose.yml -f docker-compose.dev.yml

define REQUIRE_ENV
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing env file: $(ENV_FILE)"; \
		echo "Run 'make bootstrap-env', or run 'make worktree-env' for a worktree."; \
		exit 1; \
	fi
endef

# ---------- One-click commands ----------

bootstrap-env:
	@bash scripts/bootstrap-env.sh "$(MAIN_ENV_FILE)"

env-check:
	$(REQUIRE_ENV)
	@bash scripts/bootstrap-env.sh --check "$(ENV_FILE)"

ensure-env:
	@if [ -f "$(ENV_FILE)" ]; then \
		exit 0; \
	elif [ "$(ENV_FILE)" = "$(MAIN_ENV_FILE)" ]; then \
		bash scripts/bootstrap-env.sh "$(ENV_FILE)"; \
	else \
		echo "Missing env file: $(ENV_FILE)"; \
		echo "Run 'make worktree-env' for a worktree environment."; \
		exit 1; \
	fi

# First-time setup: install deps, start DB, run migrations
setup: ensure-env
	@echo "==> Using env file: $(ENV_FILE)"
	@echo "==> Installing dependencies..."
	pnpm install
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@echo "==> Running migrations..."
	@set -a; . "$(ENV_FILE)"; set +a; cd server && go run ./cmd/migrate up
	@echo ""
	@echo "✓ Setup complete! Run 'make start' to launch the app."

# Start all services (backend + frontend)
start: ensure-env
	@echo "Using env file: $(ENV_FILE)"
	@echo "API: $(NEXT_PUBLIC_API_URL)"
	@echo "Frontend: $(FRONTEND_ORIGIN)"
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@echo "Starting backend and frontend..."
	@set -a; . "$(ENV_FILE)"; set +a; \
		trap 'kill 0' EXIT; \
		(cd server && go run ./cmd/server) & \
		pnpm dev:web & \
		wait

# Stop all services
stop:
	$(REQUIRE_ENV)
	@echo "Stopping services..."
	@-lsof -ti:$(PORT) | xargs kill -9 2>/dev/null
	@-lsof -ti:$(FRONTEND_PORT) | xargs kill -9 2>/dev/null
	@echo "✓ App processes stopped. Shared PostgreSQL is still running."

# Full verification: typecheck + unit tests + Go tests + E2E
check: ensure-env
	@ENV_FILE="$(ENV_FILE)" bash scripts/check.sh

db-up: ensure-env
	@$(COMPOSE_DEV) --env-file "$(ENV_FILE)" up -d postgres

db-down:
	@$(COMPOSE_DEV) --env-file "$(ENV_FILE)" down

worktree-env:
	@bash scripts/init-worktree-env.sh .env.worktree

setup-main:
	@$(MAKE) setup ENV_FILE=$(MAIN_ENV_FILE)

start-main:
	@$(MAKE) start ENV_FILE=$(MAIN_ENV_FILE)

stop-main:
	@$(MAKE) stop ENV_FILE=$(MAIN_ENV_FILE)

check-main:
	@ENV_FILE=$(MAIN_ENV_FILE) bash scripts/check.sh

setup-worktree:
	@echo "==> Generating $(WORKTREE_ENV_FILE) with unique ports..."
	@FORCE=1 bash scripts/init-worktree-env.sh $(WORKTREE_ENV_FILE)
	@$(MAKE) setup ENV_FILE=$(WORKTREE_ENV_FILE)

start-worktree:
	@$(MAKE) start ENV_FILE=$(WORKTREE_ENV_FILE)

stop-worktree:
	@$(MAKE) stop ENV_FILE=$(WORKTREE_ENV_FILE)

check-worktree:
	@ENV_FILE=$(WORKTREE_ENV_FILE) bash scripts/check.sh

# ---------- Individual commands ----------

# Go server
dev: ensure-env
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@set -a; . "$(ENV_FILE)"; set +a; cd server && go run ./cmd/server

daemon:
	@$(MAKE) agentra AGENTRA_ARGS="daemon"

cli:
	@$(MAKE) agentra AGENTRA_ARGS="$(AGENTRA_ARGS)"

agentra:
	cd server && go run ./cmd/agentra $(AGENTRA_ARGS)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

build:
	cd server && go build -o bin/server ./cmd/server
	cd server && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/agentra ./cmd/agentra

test: ensure-env
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@set -a; . "$(ENV_FILE)"; set +a; cd server && go test ./...

# Database
migrate-up: ensure-env
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@set -a; . "$(ENV_FILE)"; set +a; cd server && go run ./cmd/migrate up

migrate-down: ensure-env
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@set -a; . "$(ENV_FILE)"; set +a; cd server && go run ./cmd/migrate down

sqlc:
	cd server && sqlc generate

# Cleanup
clean:
	rm -rf server/bin server/tmp
