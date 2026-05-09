package service

import (
	"github.com/agentra-ai/agentra/server/pkg/traces"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TraceService wraps the traces package TraceService for use by the handler layer.
type TraceService struct {
	*traces.TraceService
}

// NewTraceServiceFromPool creates a TraceService from the given connection pool.
// The pool must implement the txStarter interface (i.e., *pgxpool.Pool).
func NewTraceServiceFromPool(pool any) *TraceService {
	if p, ok := pool.(*pgxpool.Pool); ok {
		return &TraceService{traces.NewTraceService(p)}
	}
	// If the pool doesn't match, return an uninitialized TraceService.
	// Methods will fail at runtime with nil pointer errors — callers should
	// ensure they pass a valid *pgxpool.Pool.
	return &TraceService{}
}
