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
//
// Returns the Coordinator so callers (e.g. main) can hand it to the Handler
// for the synchronous StartLoop path that the CreateLoop handler drives.
// Without this wiring, freshly created loops would sit forever in 'pending'
// status because the event-driven state machine has no preceding task to
// fire on for the plan stage.
func runLoopCoordinator(ctx context.Context, queries *db.Queries, bus *events.Bus) *looppkg.Coordinator {
	coord := looppkg.NewCoordinator(queries, bus)
	coord.RestoreOnStartup(ctx)

	bus.Subscribe(protocol.EventTaskCompleted, coord.HandleTaskCompleted)
	bus.Subscribe(protocol.EventTaskFailed, coord.HandleTaskFailed)

	slog.Info("loop coordinator: subscribed",
		"completed", protocol.EventTaskCompleted,
		"failed", protocol.EventTaskFailed)
	return coord
}
