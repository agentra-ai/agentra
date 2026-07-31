package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	response := httptest.NewRecorder()

	livenessHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
}

func TestReadinessHandlerReady(t *testing.T) {
	ok := func(context.Context) error { return nil }
	handler := readinessHandler(readinessDependencies{
		database:  ok,
		migration: ok,
		storage:   ok,
		scheduler: true,
	})
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ready" {
		t.Fatalf("status body = %q, want ready", body.Status)
	}
	for _, name := range []string{"database", "migration", "storage", "scheduler"} {
		if body.Checks[name].Status != "ok" {
			t.Errorf("check %s = %q, want ok", name, body.Checks[name].Status)
		}
	}
}

func TestReadinessHandlerReportsDependencyFailures(t *testing.T) {
	ok := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errors.New("unavailable") }
	handler := readinessHandler(readinessDependencies{
		database:  ok,
		migration: fail,
		storage:   fail,
		scheduler: false,
	})
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "not_ready" {
		t.Fatalf("status body = %q, want not_ready", body.Status)
	}
	for _, name := range []string{"migration", "storage", "scheduler"} {
		if body.Checks[name].Status != "error" {
			t.Errorf("check %s = %q, want error", name, body.Checks[name].Status)
		}
	}
}

func TestReadinessHandlerAllowsDisabledOptionalStorage(t *testing.T) {
	ok := func(context.Context) error { return nil }
	handler := readinessHandler(readinessDependencies{
		database:  ok,
		migration: ok,
		scheduler: true,
	})
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Checks["storage"].Status != "disabled" {
		t.Fatalf("storage check = %q, want disabled", body.Checks["storage"].Status)
	}
}
