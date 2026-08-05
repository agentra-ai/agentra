package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	stripelib "github.com/agentra-ai/agentra/server/pkg/stripe"

	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/buildinfo"
	"github.com/agentra-ai/agentra/server/internal/corsconfig"
	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/logger"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	"github.com/agentra-ai/agentra/server/internal/service"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func main() {
	logger.Init()

	port := os.Getenv("PORT")
	if port == "" {
		slog.Error("PORT is required")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Fail fast on a weak or missing JWT secret. The auth package panics
	// on misconfiguration; do the same here with a friendly message so the
	// operator sees the cause before the first request.
	_ = auth.JWTSecret()

	// Restrict WebSocket upgrades to the same origins configured for CORS.
	realtime.SetWSAllowedOrigins(corsconfig.AllowedOrigins())

	bus := events.New()
	hub := realtime.NewHub()
	go hub.Run()
	registerListeners(bus, hub)

	queries := db.New(pool)
	// Order matters: subscriber listeners must register BEFORE notification listeners.
	// The notification listener queries the subscriber table to determine recipients,
	// so subscribers must be written first within the same synchronous event dispatch.
	registerSubscriberListeners(bus, queries)
	registerActivityListeners(bus, queries)
	registerNotificationListeners(bus, queries)

	// Background sweepers and the loop coordinator. runLoopCoordinator returns
	// the Coordinator it constructs so we can hand it to the router/handler
	// for the synchronous CreateLoop -> StartLoop path. The coordinator
	// runs RestoreOnStartup eagerly inside the constructor's caller, so
	// building the router afterwards guarantees the DB restore completes
	// before any HTTP request can hit CreateLoop.
	sweepCtx, sweepCancel := context.WithCancel(context.Background())
	runLifecycle := service.NewRunLifecycle(pool, queries)
	go runRuntimeSweeper(sweepCtx, queries, bus, runLifecycle)
	loopCoord := runLoopCoordinator(sweepCtx, pool, queries)
	lifecycleWorker := service.NewLifecycleOutboxWorker(queries, bus, service.NewTraceServiceFromPool(pool))
	go lifecycleWorker.Run(sweepCtx)
	taskDerivedProjector := service.NewTaskDerivedLifecycleProjector(pool, queries, bus)
	go taskDerivedProjector.Run(sweepCtx)

	stripeClient := stripelib.NewClient(
		os.Getenv("STRIPE_SECRET_KEY"),
		os.Getenv("STRIPE_WEBHOOK_SECRET"),
		os.Getenv(stripelib.PriceBaseSeat),
		os.Getenv(stripelib.PriceAgentRuntime),
	)

	r := newRouter(pool, hub, bus, loopCoord, stripeClient)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		info := buildinfo.Current()
		slog.Info("server starting", "port", port, "version", info.Version, "commit", info.Commit)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	sweepCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
