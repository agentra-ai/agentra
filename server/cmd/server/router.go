package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/cli"
	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/handler"
	"github.com/agentra-ai/agentra/server/internal/middleware"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	"github.com/agentra-ai/agentra/server/internal/service"
	"github.com/agentra-ai/agentra/server/internal/storage"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	}
	if raw == "" {
		raw = cli.ResolveSiteURLFromEnv()
	}
	if raw == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{}
	}
	return origins
}

// NewRouter creates the fully-configured Chi router with all middleware and routes.
func NewRouter(pool *pgxpool.Pool, hub *realtime.Hub, bus *events.Bus) chi.Router {
	queries := db.New(pool)
	emailSvc := service.NewEmailService()

	// STORAGE_DRIVER controls which backend to use: "minio" (default) or "s3".
	// Both backends satisfy the storage.FileStorage interface.
	var fileStorage storage.FileStorage
	driver := os.Getenv("STORAGE_DRIVER")
	if driver == "" {
		driver = "minio" // MinIO is the default for self-hosted deployments
	}
	switch driver {
	case "minio":
		if minio := storage.NewMinIOStorageFromEnv(); minio != nil {
			fileStorage = minio
		} else {
			slog.Warn("MinIO driver selected but not configured, falling back to S3")
			if s3 := storage.NewS3StorageFromEnv(); s3 != nil {
				fileStorage = s3
			}
		}
	case "s3":
		if s3 := storage.NewS3StorageFromEnv(); s3 != nil {
			fileStorage = s3
		} else {
			slog.Warn("S3 driver selected but not configured")
		}
	default:
		slog.Warn("unknown STORAGE_DRIVER, trying MinIO then S3", "driver", driver)
		if minio := storage.NewMinIOStorageFromEnv(); minio != nil {
			fileStorage = minio
		} else if s3 := storage.NewS3StorageFromEnv(); s3 != nil {
			fileStorage = s3
		}
	}
	if fileStorage == nil {
		slog.Info("no file storage configured, uploads disabled")
	}

	cfSigner := auth.NewCloudFrontSignerFromEnv()
	h := handler.New(queries, pool, hub, bus, emailSvc, fileStorage, cfSigner)

	// Wire up GatewayHub callbacks to TaskService
	setGatewayCallbacks(hub, h)

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID", "X-Request-ID", "X-Agent-ID", "X-Task-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// WebSocket
	mc := &membershipChecker{queries: queries}
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleWebSocket(hub, mc, w, r)
	})

	// Cloud Runtime Gateway WebSocket
	r.Get("/api/gateway/connect", func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleGatewayWebSocket(hub, w, r)
	})

	// Auth (public)
	r.Post("/auth/send-code", h.SendCode)
	r.Post("/auth/verify-code", h.VerifyCode)
	r.Get("/api/files/{key}", h.GetPublicFile)

	// Daemon API routes (all require a valid token)
	r.Route("/api/daemon", func(r chi.Router) {
		r.Use(middleware.Auth(queries))

		r.Post("/register", h.DaemonRegister)
		r.Post("/deregister", h.DaemonDeregister)
		r.Post("/heartbeat", h.DaemonHeartbeat)

		r.Post("/runtimes/{runtimeId}/tasks/claim", h.ClaimTaskByRuntime)
		r.Get("/runtimes/{runtimeId}/tasks/pending", h.ListPendingTasksByRuntime)
		r.Post("/runtimes/{runtimeId}/usage", h.ReportRuntimeUsage)
		r.Post("/runtimes/{runtimeId}/ping/{pingId}/result", h.ReportPingResult)
		r.Post("/runtimes/{runtimeId}/update/{updateId}/result", h.ReportUpdateResult)

		r.Get("/tasks/{taskId}/status", h.GetTaskStatus)
		r.Post("/tasks/{taskId}/start", h.StartTask)
		r.Post("/tasks/{taskId}/progress", h.ReportTaskProgress)
		r.Post("/tasks/{taskId}/stage", h.ReportAgentStage)
		r.Post("/tasks/{taskId}/complete", h.CompleteTask)
		r.Post("/tasks/{taskId}/fail", h.FailTask)
		r.Post("/tasks/{taskId}/messages", h.ReportTaskMessages)
		r.Get("/tasks/{taskId}/messages", h.ListTaskMessages)
	})

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(queries))
		r.Use(middleware.RefreshCloudFrontCookies(cfSigner))

		// --- User-scoped routes (no workspace context required) ---
		r.Get("/api/me", h.GetMe)
		r.Patch("/api/me", h.UpdateMe)
		r.Post("/api/upload-file", h.UploadFile)

		r.Route("/api/workspaces", func(r chi.Router) {
			r.Get("/", h.ListWorkspaces)
			r.Post("/", h.CreateWorkspace)
			r.Route("/{id}", func(r chi.Router) {
				// Member-level access
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "id"))
					r.Get("/", h.GetWorkspace)
					r.Get("/members", h.ListMembersWithUser)
					r.Post("/leave", h.LeaveWorkspace)
				})
				// Admin-level access
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner", "admin"))
					r.Put("/", h.UpdateWorkspace)
					r.Patch("/", h.UpdateWorkspace)
					r.Post("/members", h.CreateMember)
					r.Route("/members/{memberId}", func(r chi.Router) {
						r.Patch("/", h.UpdateMember)
						r.Delete("/", h.DeleteMember)
					})
				})
				// Owner-only access
				r.With(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner")).Delete("/", h.DeleteWorkspace)
			})
		})

		r.Route("/api/tokens", func(r chi.Router) {
			r.Get("/", h.ListPersonalAccessTokens)
			r.Post("/", h.CreatePersonalAccessToken)
			r.Delete("/{id}", h.RevokePersonalAccessToken)
		})

		// --- Workspace-scoped routes (all require workspace membership) ---
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireWorkspaceMember(queries))

			// Issues
			r.Route("/api/issues", func(r chi.Router) {
				r.Get("/", h.ListIssues)
				r.Post("/", h.CreateIssue)
				r.Post("/batch-update", h.BatchUpdateIssues)
				r.Post("/batch-delete", h.BatchDeleteIssues)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetIssue)
					r.Put("/", h.UpdateIssue)
					r.Delete("/", h.DeleteIssue)
					r.Post("/comments", h.CreateComment)
					r.Get("/comments", h.ListComments)
					r.Get("/timeline", h.ListTimeline)
					r.Get("/subscribers", h.ListIssueSubscribers)
					r.Post("/subscribe", h.SubscribeToIssue)
					r.Post("/unsubscribe", h.UnsubscribeFromIssue)
					r.Get("/active-task", h.GetActiveTaskForIssue)
					r.Post("/tasks/{taskId}/cancel", h.CancelTask)
					r.Get("/task-runs", h.ListTasksByIssue)
					r.Post("/reactions", h.AddIssueReaction)
					r.Delete("/reactions", h.RemoveIssueReaction)
					r.Get("/attachments", h.ListAttachments)
				})
			})

			// Attachments
			r.Get("/api/attachments/{id}", h.GetAttachmentByID)
			r.Delete("/api/attachments/{id}", h.DeleteAttachment)

			// Comments
			r.Route("/api/comments/{commentId}", func(r chi.Router) {
				r.Put("/", h.UpdateComment)
				r.Delete("/", h.DeleteComment)
				r.Post("/reactions", h.AddReaction)
				r.Delete("/reactions", h.RemoveReaction)
			})

			// Agents
			r.Route("/api/agents", func(r chi.Router) {
				r.Get("/", h.ListAgents)
				r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/", h.CreateAgent)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetAgent)
					r.Put("/", h.UpdateAgent)
					r.Post("/archive", h.ArchiveAgent)
					r.Post("/restore", h.RestoreAgent)
					r.Get("/tasks", h.ListAgentTasks)
					r.Get("/skills", h.ListAgentSkills)
					r.Put("/skills", h.SetAgentSkills)
				})
			})

			// Skills
			r.Route("/api/skills", func(r chi.Router) {
				r.Get("/", h.ListSkills)
				r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/", h.CreateSkill)
				r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/import", h.ImportSkill)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetSkill)
					r.Put("/", h.UpdateSkill)
					r.Delete("/", h.DeleteSkill)
					r.Get("/files", h.ListSkillFiles)
					r.Put("/files", h.UpsertSkillFile)
					r.Delete("/files/{fileId}", h.DeleteSkillFile)
				})
			})

			// Runtimes
			r.Route("/api/runtimes", func(r chi.Router) {
				r.Get("/", h.ListAgentRuntimes)
				r.Get("/{runtimeId}/usage", h.GetRuntimeUsage)
				r.Get("/{runtimeId}/activity", h.GetRuntimeTaskActivity)
				r.Post("/{runtimeId}/ping", h.InitiatePing)
				r.Get("/{runtimeId}/ping/{pingId}", h.GetPing)
				r.Post("/{runtimeId}/update", h.InitiateUpdate)
				r.Get("/{runtimeId}/update/{updateId}", h.GetUpdate)
			})

			// Cloud Runtime (admin-only)
			r.Route("/api/cloud-runtime", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceRole(queries, "owner", "admin"))
				r.Post("/", h.RegisterCloudRuntime)
				r.Get("/", h.GetCloudRuntime)
				r.Delete("/", h.DeleteCloudRuntime)
				r.Post("/validate", h.ValidateAPIKey)
			})

			// Inbox
			r.Route("/api/inbox", func(r chi.Router) {
				r.Get("/", h.ListInbox)
				r.Get("/unread-count", h.CountUnreadInbox)
				r.Post("/mark-all-read", h.MarkAllInboxRead)
				r.Post("/archive-all", h.ArchiveAllInbox)
				r.Post("/archive-all-read", h.ArchiveAllReadInbox)
				r.Post("/archive-completed", h.ArchiveCompletedInbox)
				r.Post("/{id}/read", h.MarkInboxRead)
				r.Post("/{id}/archive", h.ArchiveInboxItem)
			})
		})
	})

	return r
}

// membershipChecker implements realtime.MembershipChecker using database queries.
type membershipChecker struct {
	queries *db.Queries
}

func (mc *membershipChecker) IsMember(ctx context.Context, userID, workspaceID string) bool {
	_, err := mc.queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(userID),
		WorkspaceID: parseUUID(workspaceID),
	})
	return err == nil
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}
	}
	return u
}

// setGatewayCallbacks wires up the GatewayHub callbacks to TaskService methods.
func setGatewayCallbacks(hub *realtime.Hub, h *handler.Handler) {
	hub.GatewayHub.OnTaskComplete = func(gatewayID, taskID string, exitCode int, output string) {
		ctx := context.Background()
		taskUUID := util.ParseUUID(taskID)
		if !taskUUID.Valid {
			slog.Error("gateway complete: invalid task ID", "task_id", taskID)
			return
		}
		// Exit code 0 = success, non-zero = failure
		if exitCode == 0 {
			_, err := h.TaskService.CompleteTask(ctx, taskUUID, []byte(output), "", "")
			if err != nil {
				slog.Error("gateway complete: failed", "task_id", taskID, "error", err)
			}
		} else {
			_, err := h.TaskService.FailTask(ctx, taskUUID, output)
			if err != nil {
				slog.Error("gateway fail: failed", "task_id", taskID, "error", err)
			}
		}
	}

	hub.GatewayHub.OnTaskFail = func(gatewayID, taskID string, errorMsg string, retryable bool) {
		ctx := context.Background()
		taskUUID := util.ParseUUID(taskID)
		if !taskUUID.Valid {
			slog.Error("gateway fail: invalid task ID", "task_id", taskID)
			return
		}

		// If the failure is retryable, attempt to retry the task
		if retryable {
			if task, retried, err := h.TaskService.RetryTask(ctx, taskUUID); err != nil {
				slog.Error("gateway fail: retry failed", "task_id", taskID, "error", err)
				// Fall through to mark as failed
			} else if retried {
				slog.Info("gateway fail: task re-queued for retry", "task_id", taskID,
					"retry_count", task.RetryCount, "max_retries", task.MaxRetries)
				return
			} else {
				slog.Warn("gateway fail: retry not possible (max retries or invalid state)", "task_id", taskID)
				// Fall through to mark as failed
			}
		}

		_, err := h.TaskService.FailTask(ctx, taskUUID, errorMsg)
		if err != nil {
			slog.Error("gateway fail: failed", "task_id", taskID, "error", err)
		}
	}

	hub.GatewayHub.OnTaskLogs = func(gatewayID, taskID string, logs string) {
		// Broadcast logs to workspace via realtime
		// TODO: Implement log streaming to web clients
		slog.Debug("gateway logs", "gateway_id", gatewayID, "task_id", taskID, "logs_len", len(logs))
	}
}
