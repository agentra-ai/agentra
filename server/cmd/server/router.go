package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/pkg/taskgraph"
	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/corsconfig"
	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/handler"
	"github.com/agentra-ai/agentra/server/internal/loop"
	"github.com/agentra-ai/agentra/server/internal/middleware"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	"github.com/agentra-ai/agentra/server/internal/service"
	"github.com/agentra-ai/agentra/server/internal/storage"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/agentra-ai/agentra/server/pkg/redact"
	stripelib "github.com/agentra-ai/agentra/server/pkg/stripe"
)

// allowedOrigins delegates to internal/corsconfig so the resolution logic
// can be unit-tested without booting the database. A nil return means
// "do not enable CORS at all" — empty slice would silently allow all
// origins in go-chi/cors.
func allowedOrigins() []string {
	return corsconfig.AllowedOrigins()
}

// NewRouter creates the fully-configured Chi router with all middleware and routes.
func NewRouter(pool *pgxpool.Pool, hub *realtime.Hub, bus *events.Bus, stripeClient *stripelib.Client) chi.Router {
	return newRouter(pool, hub, bus, nil, stripeClient)
}

// newRouter is the internal form that accepts an optional loop Coordinator
// to wire into the Handler. The Handler falls back to a nil coordinator
// (CreateLoop then no-ops the StartLoop call) when nil is passed, which is
// what unit tests want.
func newRouter(pool *pgxpool.Pool, hub *realtime.Hub, bus *events.Bus, loopCoord *loop.Coordinator, stripeClient *stripelib.Client) chi.Router {
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
	graphStore := taskgraph.NewGraphStore(pool)
	plannerSvc := service.NewPlannerService(queries, graphStore)
	h := handler.New(queries, pool, hub, bus, graphStore, plannerSvc, emailSvc, fileStorage, cfSigner)
	projectsHandler := handler.NewProjectHandler(queries)
	billingHandler := handler.NewBillingHandler(queries, stripeClient)
	memoryHandler := handler.NewMemoryHandler(queries)
	metricsHandler := handler.NewMetricsHandler(queries)
	if loopCoord != nil {
		h.SetLoopCoordinator(loopCoord)
	}

	// Wire up GatewayHub callbacks to TaskService
	setGatewayCallbacks(hub, h)

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Recoverer)
	if origins := allowedOrigins(); origins != nil {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   origins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID", "X-Request-ID", "X-Agent-ID", "X-Task-ID"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	} else {
		slog.Warn("CORS not configured: set CORS_ALLOWED_ORIGINS or FRONTEND_ORIGIN; cross-origin browser requests will fail")
	}

	readiness := newReadinessDependencies(pool, fileStorage, loopCoord != nil)

	// /health remains the lightweight endpoint used by the CLI.
	// Deployments should use /livez for liveness and /readyz for traffic gates.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeHealthJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/livez", livenessHandler)
	r.Get("/readyz", readinessHandler(readiness))
	r.Get("/api/version", versionHandler)

	// WebSocket
	mc := &membershipChecker{queries: queries}
	wa := &websocketAuthenticator{queries: queries}
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleWebSocket(hub, wa, mc, w, r)
	})

	// Cloud Runtime Gateway WebSocket
	r.Get("/api/gateway/connect", func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleGatewayWebSocket(hub, wa, mc, w, r)
	})

	// Auth (public)
	r.Post("/auth/send-code", h.SendCode)
	r.Post("/auth/verify-code", h.VerifyCode)

	// GitHub OAuth (public)
	githubOAuth := handler.NewGitHubOAuthHandler(
		os.Getenv("GITHUB_CLIENT_ID"),
		os.Getenv("GITHUB_CLIENT_SECRET"),
		os.Getenv("GITHUB_REDIRECT_URL"),
	)
	r.Route("/github/oauth", func(r chi.Router) {
		githubOAuth.RegisterRoutes(r)
	})

	// Stripe webhook — PUBLIC (no JWT). Must live outside the protected group
	// so Stripe can POST without credentials. Signature verification is performed
	// inside the handler using the configured STRIPE_WEBHOOK_SECRET.
	r.Post("/webhooks/stripe", billingHandler.StripeWebhook)

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
		r.Post("/tasks/{taskId}/session", h.CheckpointTaskSession)
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
	})

	// Public file access — the 128-bit key IS the access token (unguessable).
	// <img> tags can't send Authorization headers, so this route lives
	// outside the auth group. GetPublicFile still enforces workspace
	// membership when a userID is supplied (programmatic API clients).
	r.Get("/api/files/{key}", h.GetPublicFile)
	r.Get("/files/{key}", h.GetPublicFile)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(queries))
		r.Use(middleware.RefreshCloudFrontCookies(cfSigner))

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
					r.Post("/seed-specialists", h.SeedSpecialists)
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
				// Owner/Admin manage SSO config (Issue #24)
				r.Route("/sso", func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner", "admin"))
					r.Get("/", h.GetSSOConfig)
					r.Put("/", h.SetSSOConfig)
				})
				// Goal-first execute endpoint
				r.Post("/execute", h.ExecuteGoal)

				// Billing (workspace-scoped, owner/admin). Must live inside the
				// {id} subrouter so chi.URLParam(r, "id") resolves to the workspace
				// id from the matched path — registering it as a sibling of {id}
				// captures the literal segment "billing" as the id param instead.
				r.Route("/billing", func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner", "admin"))
					r.Get("/subscription", billingHandler.GetSubscription)
					r.Get("/checkout", billingHandler.CreateCheckoutSession)
					r.Get("/portal", billingHandler.CreatePortalSession)
					r.Get("/invoices", billingHandler.ListInvoices)
					r.Get("/usage", billingHandler.GetUsage)
				})

				// Memories (workspace-scoped, member access). Team memories are
				// shared across the workspace; the workspace id comes from {id}.
				r.Route("/memories", func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "id"))
					r.Get("/", memoryHandler.ListTeamMemories)
					r.Get("/search", memoryHandler.SearchMemories)
					r.Post("/", memoryHandler.CreateTeamMemory)
					r.Delete("/{memoryId}", memoryHandler.DeleteTeamMemory)
				})
			})

		})

		// Agent memories resolve their workspace from the agent URL parameter.
		// Register from the protected API root so the handler paths are not
		// accidentally prefixed with /api/workspaces.
		memoryHandler.RegisterAgentRoutes(r)

		r.Route("/api/tokens", func(r chi.Router) {
			r.Get("/", h.ListPersonalAccessTokens)
			r.Post("/", h.CreatePersonalAccessToken)
			r.Delete("/{id}", h.RevokePersonalAccessToken)
		})

		// Issues (workspace-scoped; workspace_id comes from X-Workspace-ID header or query param)
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
				r.Get("/traces", h.ListTracesByIssue)
				r.Post("/reactions", h.AddIssueReaction)
				r.Delete("/reactions", h.RemoveIssueReaction)
				r.Get("/attachments", h.ListAttachments)
				r.Post("/auto-decompose", h.AutoDecomposeIssue)
			})
		})

		// Task graph reads and node mutations are workspace-scoped. The
		// handlers additionally verify that the issue or node belongs to the
		// authorized workspace.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireWorkspaceMember(queries))
			r.Get("/api/issues/{id}/graph", h.GetTaskGraph)
			r.Patch("/api/graph/nodes/{id}", h.UpdateTaskGraphNode)
			r.Delete("/api/graph/nodes/{id}", h.DeleteTaskGraphNode)
		})

		// Metrics are restricted to workspace owners and admins. The
		// handler only accepts the workspace authorized and injected by the
		// middleware instead of independently trusting a query parameter.
		r.Route("/api/admin/metrics", func(r chi.Router) {
			r.Use(middleware.RequireWorkspaceRole(queries, "owner", "admin"))
			metricsHandler.RegisterRoutes(r)
		})

		// Attachments
		r.Get("/api/attachments/{id}", h.GetAttachmentByID)
		r.Delete("/api/attachments/{id}", h.DeleteAttachment)

		// Git hooks API
		r.Route("/api/git", func(r chi.Router) {
			r.Post("/link-commit", h.LinkCommit)
			r.Post("/link-pr", h.LinkPR)
			r.Post("/link-branch", h.LinkBranch)
			r.Get("/active-task", h.GetActiveTask)
			r.Get("/issue-links/{issueId}", h.GetIssueLinks)
		})

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

		// Projects (workspace-scoped, member-level reads / owner-admin writes)
		r.Route("/api/workspaces/{id}/projects", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "id"))
				r.Get("/", projectsHandler.ListProjects)
				r.Get("/unassigned", projectsHandler.ListUnassignedIssues)
				r.Get("/{projectId}", projectsHandler.GetProject)
				r.Get("/{projectId}/issues", projectsHandler.ListProjectIssues)
				r.Get("/{projectId}/milestones", projectsHandler.ListMilestones)
			})
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner", "admin"))
				r.Post("/", projectsHandler.CreateProject)
				r.Put("/{projectId}", projectsHandler.UpdateProject)
				r.Delete("/{projectId}", projectsHandler.DeleteProject)
				r.Post("/{projectId}/issues/{issueId}", projectsHandler.AssignOrRemoveIssue)
				r.Post("/{projectId}/milestones", projectsHandler.CreateMilestone)
				r.Patch("/{projectId}/milestones/{milestoneId}", projectsHandler.UpdateMilestone)
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

		// Traces
		r.Get("/api/traces/{taskId}", h.GetTraceByTask)
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

		// Engineering loops
		r.Route("/api/loops", func(r chi.Router) {
			r.Get("/", h.ListLoops)
			r.Post("/", h.CreateLoop)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.GetLoop)
				r.Post("/pause", h.PauseLoop)
				r.Post("/resume", h.ResumeLoop)
				r.Post("/cancel", h.CancelLoop)
			})
		})
	})

	return r
}

// membershipChecker implements realtime.MembershipChecker using database queries.
type membershipChecker struct {
	queries *db.Queries
}

type websocketAuthenticator struct {
	queries *db.Queries
}

func (wa *websocketAuthenticator) Authenticate(ctx context.Context, token string) (string, error) {
	identity, err := middleware.AuthenticateUserToken(ctx, wa.queries, token)
	if err != nil {
		return "", err
	}
	return identity.UserID, nil
}

func (mc *membershipChecker) IsMember(ctx context.Context, userID, workspaceID string) bool {
	_, err := mc.queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(userID),
		WorkspaceID: parseUUID(workspaceID),
	})
	return err == nil
}

func (mc *membershipChecker) CanConnectGateway(ctx context.Context, userID, workspaceID string) bool {
	member, err := mc.queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(userID),
		WorkspaceID: parseUUID(workspaceID),
	})
	return err == nil && (member.Role == "owner" || member.Role == "admin")
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
	authorize := func(ctx context.Context, gatewayID, workspaceID, taskID, event string) (db.AgentTaskQueue, bool) {
		task, err := h.TaskService.ValidateCloudGatewayTask(ctx, workspaceID, taskID)
		if err != nil {
			slog.Warn("gateway event rejected", "gateway_id", gatewayID, "workspace_id", workspaceID, "task_id", taskID, "event", event)
			return db.AgentTaskQueue{}, false
		}
		return task, true
	}

	hub.GatewayHub.OnTaskDispatched = func(gatewayID, workspaceID, taskID, containerID string) {
		ctx := context.Background()
		task, ok := authorize(ctx, gatewayID, workspaceID, taskID, protocol.EventTaskDispatched)
		if !ok {
			return
		}
		if task.Status == "running" {
			return
		}
		if task.Status != "dispatched" {
			slog.Warn("gateway dispatched: invalid task state", "gateway_id", gatewayID, "task_id", taskID, "status", task.Status)
			return
		}
		if _, err := h.TaskService.StartTask(ctx, task.ID); err != nil {
			slog.Error("gateway dispatched: failed to start task", "gateway_id", gatewayID, "task_id", taskID, "container_id", containerID, "error", err)
		}
	}

	hub.GatewayHub.OnTaskComplete = func(gatewayID, workspaceID, taskID string, exitCode int, output string) {
		ctx := context.Background()
		task, ok := authorize(ctx, gatewayID, workspaceID, taskID, protocol.EventTaskCompleted)
		if !ok {
			return
		}
		output = boundedGatewayText(output)
		// Exit code 0 = success, non-zero = failure
		if exitCode == 0 {
			result, err := json.Marshal(protocol.TaskCompletedPayload{TaskID: taskID, Output: output})
			if err != nil {
				slog.Error("gateway complete: marshal result failed", "task_id", taskID, "error", err)
				return
			}
			_, err = h.TaskService.CompleteTask(ctx, task.ID, result, "", "")
			if err != nil {
				slog.Error("gateway complete: failed", "task_id", taskID, "error", err)
			}
		} else {
			_, err := h.TaskService.FailTask(ctx, task.ID, output)
			if err != nil {
				slog.Error("gateway fail: failed", "task_id", taskID, "error", err)
			}
		}
	}

	hub.GatewayHub.OnTaskFail = func(gatewayID, workspaceID, taskID string, errorMsg string, retryable bool) {
		ctx := context.Background()
		task, ok := authorize(ctx, gatewayID, workspaceID, taskID, protocol.EventTaskFailed)
		if !ok {
			return
		}
		errorMsg = boundedGatewayText(errorMsg)

		// If the failure is retryable, attempt to retry the task
		if retryable {
			if task, retried, err := h.TaskService.RetryTask(ctx, task.ID); err != nil {
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

		_, err := h.TaskService.FailTask(ctx, task.ID, errorMsg)
		if err != nil {
			slog.Error("gateway fail: failed", "task_id", taskID, "error", err)
		}
	}

	hub.GatewayHub.OnTaskLogs = func(gatewayID, workspaceID, taskID string, seq int, stream, content string) {
		if err := h.RecordGatewayTaskLog(context.Background(), workspaceID, taskID, seq, stream, content); err != nil {
			slog.Warn("gateway logs rejected", "gateway_id", gatewayID, "workspace_id", workspaceID, "task_id", taskID, "seq", seq, "error", err)
		}
	}
}

func boundedGatewayText(value string) string {
	value = redact.Text(strings.ToValidUTF8(value, "\uFFFD"))
	if len(value) <= protocol.GatewayTaskResultBytes {
		return value
	}
	return strings.ToValidUTF8(value[len(value)-protocol.GatewayTaskResultBytes:], "")
}
