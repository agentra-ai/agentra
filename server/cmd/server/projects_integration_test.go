package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func requireStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("expected status %d, got %d", want, response.StatusCode)
	}
}

func TestProjectLifecycle(t *testing.T) {
	requireIntegrationDB(t)

	slug := fmt.Sprintf("integration-project-%d", time.Now().UnixNano())
	createResponse := authRequest(t, http.MethodPost,
		"/api/workspaces/"+testWorkspaceID+"/projects",
		map[string]string{"title": "Integration Project", "slug": slug},
	)
	if createResponse.StatusCode != http.StatusCreated {
		requireStatus(t, createResponse, http.StatusCreated)
	}
	var project struct {
		ID string `json:"id"`
	}
	readJSON(t, createResponse, &project)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, project.ID)
	})

	issueID := createTestIssue(t, testWorkspaceID, testUserID)
	t.Cleanup(func() { cleanupTestIssue(t, issueID) })
	assignResponse := authRequest(t, http.MethodPost,
		fmt.Sprintf("/api/workspaces/%s/projects/%s/issues/%s", testWorkspaceID, project.ID, issueID),
		map[string]string{"action": "assign"},
	)
	requireStatus(t, assignResponse, http.StatusOK)

	issuesResponse := authRequest(t, http.MethodGet,
		fmt.Sprintf("/api/workspaces/%s/projects/%s/issues", testWorkspaceID, project.ID), nil,
	)
	if issuesResponse.StatusCode != http.StatusOK {
		requireStatus(t, issuesResponse, http.StatusOK)
	}
	var issues []struct {
		ID string `json:"id"`
	}
	readJSON(t, issuesResponse, &issues)
	if len(issues) != 1 || issues[0].ID != issueID {
		t.Fatalf("expected assigned issue %s, got %#v", issueID, issues)
	}

	milestoneResponse := authRequest(t, http.MethodPost,
		fmt.Sprintf("/api/workspaces/%s/projects/%s/milestones", testWorkspaceID, project.ID),
		map[string]string{"title": "Integration Milestone"},
	)
	if milestoneResponse.StatusCode != http.StatusCreated {
		requireStatus(t, milestoneResponse, http.StatusCreated)
	}
	var milestone struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	readJSON(t, milestoneResponse, &milestone)
	if milestone.Status != "active" {
		t.Fatalf("expected active milestone, got %q", milestone.Status)
	}

	updateMilestoneResponse := authRequest(t, http.MethodPatch,
		fmt.Sprintf("/api/workspaces/%s/projects/%s/milestones/%s", testWorkspaceID, project.ID, milestone.ID),
		map[string]string{"status": "completed"},
	)
	if updateMilestoneResponse.StatusCode != http.StatusOK {
		requireStatus(t, updateMilestoneResponse, http.StatusOK)
	}
	readJSON(t, updateMilestoneResponse, &milestone)
	if milestone.Status != "completed" {
		t.Fatalf("expected completed milestone, got %q", milestone.Status)
	}

	removeResponse := authRequest(t, http.MethodPost,
		fmt.Sprintf("/api/workspaces/%s/projects/%s/issues/%s", testWorkspaceID, project.ID, issueID),
		map[string]string{"action": "remove"},
	)
	requireStatus(t, removeResponse, http.StatusOK)

	deleteResponse := authRequest(t, http.MethodDelete,
		fmt.Sprintf("/api/workspaces/%s/projects/%s", testWorkspaceID, project.ID), nil,
	)
	requireStatus(t, deleteResponse, http.StatusNoContent)
}

func TestProjectsRejectCrossWorkspaceEntityAccess(t *testing.T) {
	requireIntegrationDB(t)

	ctx := context.Background()
	foreignSlug := fmt.Sprintf("foreign-project-workspace-%d", time.Now().UnixNano())
	var foreignWorkspaceID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug)
		VALUES ('Foreign Project Workspace', $1)
		RETURNING id
	`, foreignSlug).Scan(&foreignWorkspaceID); err != nil {
		t.Fatalf("create foreign workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, foreignWorkspaceID)
	})

	var foreignProjectID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, title, slug, owner_id)
		VALUES ($1, 'Foreign Project', 'foreign-project', $2)
		RETURNING id
	`, foreignWorkspaceID, testUserID).Scan(&foreignProjectID); err != nil {
		t.Fatalf("create foreign project: %v", err)
	}

	requests := []struct {
		method string
		path   string
		body   any
	}{
		{
			method: http.MethodGet,
			path:   fmt.Sprintf("/api/workspaces/%s/projects/%s", testWorkspaceID, foreignProjectID),
		},
		{
			method: http.MethodPut,
			path:   fmt.Sprintf("/api/workspaces/%s/projects/%s", testWorkspaceID, foreignProjectID),
			body:   map[string]string{"title": "Cross-workspace mutation"},
		},
		{
			method: http.MethodPost,
			path:   fmt.Sprintf("/api/workspaces/%s/projects/%s/milestones", testWorkspaceID, foreignProjectID),
			body:   map[string]string{"title": "Cross-workspace milestone"},
		},
	}

	for _, request := range requests {
		response := authRequest(t, request.method, request.path, request.body)
		requireStatus(t, response, http.StatusNotFound)
	}

	var title string
	if err := testPool.QueryRow(ctx, `SELECT title FROM projects WHERE id = $1`, foreignProjectID).Scan(&title); err != nil {
		t.Fatalf("load foreign project: %v", err)
	}
	if title != "Foreign Project" {
		t.Fatalf("foreign project was mutated: %q", title)
	}
}
