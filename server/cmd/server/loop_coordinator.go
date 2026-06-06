package main

import (
	"context"
	"log/slog"

	"github.com/agentra-ai/agentra/server/internal/events"
	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

// runLoopCoordinator subscribes the loop Coordinator to task lifecycle events
// and restores any loops that were running when the server last stopped.
// The actual state machine lives in server/internal/loop (unit-tested there);
// this is a thin wiring layer that mirrors runRuntimeSweeper.
func runLoopCoordinator(ctx context.Context, queries *db.Queries, bus *events.Bus) {
	coord := looppkg.NewCoordinator(queries, bus)
	coord.RestoreOnStartup(ctx)

	bus.Subscribe(protocol.EventTaskCompleted, coord.HandleTaskCompleted)
	bus.Subscribe(protocol.EventTaskFailed, coord.HandleTaskFailed)

	slog.Info("loop coordinator: subscribed",
		"completed", protocol.EventTaskCompleted,
		"failed", protocol.EventTaskFailed)
}
