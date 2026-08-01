package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentra-ai/agentra/server/internal/buildinfo"
)

func TestVersionHandlerReturnsNormalizedPublicBuildInfo(t *testing.T) {
	originalVersion, originalCommit := buildinfo.Version, buildinfo.Commit
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
	})
	buildinfo.Version = "v0.6.0"
	buildinfo.Commit = "abc123"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	versionHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var got buildinfo.Info
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Version != "0.6.0" || got.Commit != "abc123" {
		t.Fatalf("version response = %+v", got)
	}
}
