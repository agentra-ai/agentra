package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	"github.com/go-chi/chi/v5"
)

func TestRouterExposesFrontendContracts(t *testing.T) {
	router := newRouter(nil, realtime.NewHub(), events.New(), nil, nil)
	routes := routeInventory(t, router)

	want := []string{
		"GET /livez",
		"GET /readyz",
		"GET /api/issues/{id}/graph",
		"PATCH /api/graph/nodes/{id}",
		"DELETE /api/graph/nodes/{id}",
		"GET /api/admin/metrics/summary",
		"GET /api/agents/{agentId}/memories",
		"POST /api/agents/{agentId}/memories",
		"DELETE /api/agents/{agentId}/memories/{memoryId}",
		"GET /api/workspaces/{id}/memories/search",
		"GET /api/workspaces/{id}/projects",
		"POST /api/workspaces/{id}/projects",
		"GET /api/workspaces/{id}/projects/{projectId}",
		"POST /api/workspaces/{id}/projects/{projectId}/issues/{issueId}",
		"POST /api/workspaces/{id}/projects/{projectId}/milestones",
	}
	for _, contract := range want {
		if _, ok := routes[contract]; !ok {
			t.Errorf("frontend API contract %q is not mounted", contract)
		}
	}
}

func TestRouterHasNoAccidentallyNestedAPIPaths(t *testing.T) {
	router := newRouter(nil, realtime.NewHub(), events.New(), nil, nil)
	routes := routeInventory(t, router)

	for contract := range routes {
		if strings.Contains(contract, "/api/workspaces/api/") {
			t.Errorf("route is mounted under an accidental duplicate API prefix: %s", contract)
		}
	}
}

func routeInventory(t *testing.T, router chi.Router) map[string]struct{} {
	t.Helper()

	routes := make(map[string]struct{})
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		contract := fmt.Sprintf("%s %s", method, strings.TrimSuffix(route, "/"))
		routes[contract] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	return routes
}
