package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/agentra-ai/agentra/server/internal/storage"
	"github.com/agentra-ai/agentra/server/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

type readinessDependencies struct {
	database  func(context.Context) error
	migration func(context.Context) error
	storage   func(context.Context) error
	scheduler bool
}

type readinessCheck struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string                    `json:"status"`
	Checks map[string]readinessCheck `json:"checks"`
}

func newReadinessDependencies(
	pool *pgxpool.Pool,
	fileStorage storage.FileStorage,
	scheduler bool,
) readinessDependencies {
	latestMigration, latestMigrationErr := migrations.LatestVersion()
	dependencies := readinessDependencies{
		database: func(ctx context.Context) error {
			if pool == nil {
				return errors.New("database is not configured")
			}
			return pool.Ping(ctx)
		},
		migration: func(ctx context.Context) error {
			if pool == nil {
				return errors.New("database is not configured")
			}
			if latestMigrationErr != nil {
				return latestMigrationErr
			}
			var applied bool
			if err := pool.QueryRow(ctx,
				"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
				latestMigration,
			).Scan(&applied); err != nil {
				return err
			}
			if !applied {
				return errors.New("latest migration is not applied")
			}
			return nil
		},
		scheduler: scheduler,
	}
	if fileStorage != nil {
		dependencies.storage = fileStorage.HealthCheck
	}
	return dependencies
}

func writeHealthJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func livenessHandler(w http.ResponseWriter, _ *http.Request) {
	writeHealthJSON(w, http.StatusOK, map[string]string{"status": "live"})
}

func readinessHandler(dependencies readinessDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		response := readinessResponse{
			Status: "ready",
			Checks: map[string]readinessCheck{},
		}
		ready := true

		runRequiredCheck := func(name string, check func(context.Context) error) {
			if check == nil || check(ctx) != nil {
				response.Checks[name] = readinessCheck{Status: "error"}
				ready = false
				return
			}
			response.Checks[name] = readinessCheck{Status: "ok"}
		}

		runRequiredCheck("database", dependencies.database)
		runRequiredCheck("migration", dependencies.migration)
		if dependencies.storage == nil {
			response.Checks["storage"] = readinessCheck{Status: "disabled"}
		} else {
			runRequiredCheck("storage", dependencies.storage)
		}
		if dependencies.scheduler {
			response.Checks["scheduler"] = readinessCheck{Status: "ok"}
		} else {
			response.Checks["scheduler"] = readinessCheck{Status: "error"}
			ready = false
		}

		if !ready {
			response.Status = "not_ready"
			writeHealthJSON(w, http.StatusServiceUnavailable, response)
			return
		}
		writeHealthJSON(w, http.StatusOK, response)
	}
}
