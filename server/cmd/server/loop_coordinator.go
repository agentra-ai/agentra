package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// runLoopCoordinator starts the durable lifecycle projector and restores any
// loops that were running when the server last stopped.
// The actual state machine lives in server/internal/loop (unit-tested there);
// this is a thin wiring layer that mirrors runRuntimeSweeper.
//
// Returns the Coordinator so callers (e.g. main) can hand it to the Handler
// for the synchronous StartLoop path that the CreateLoop handler drives.
// Pending terminal events are drained before RestoreOnStartup so recovery
// cannot re-enqueue a stage whose Run already finished.
func runLoopCoordinator(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries) *looppkg.Coordinator {
	coord := looppkg.NewCoordinator(queries, pool)
	projector := looppkg.NewLifecycleProjector(pool, queries, coord)
	projector.Drain(ctx)
	coord.RestoreOnStartup(ctx)
	go projector.Run(ctx)

	slog.Info("loop coordinator: durable lifecycle projector started")
	return coord
}
