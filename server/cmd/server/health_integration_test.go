package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessWithMigratedIntegrationDatabase(t *testing.T) {
	requireIntegrationDB(t)

	handler := readinessHandler(newReadinessDependencies(testPool, nil, true))
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Checks["database"].Status != "ok" {
		t.Fatalf("database check = %q, want ok", body.Checks["database"].Status)
	}
	if body.Checks["migration"].Status != "ok" {
		t.Fatalf("migration check = %q, want ok", body.Checks["migration"].Status)
	}
}
