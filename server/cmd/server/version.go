package main

import (
	"encoding/json"
	"net/http"

	"github.com/agentra-ai/agentra/server/internal/buildinfo"
)

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildinfo.Current()); err != nil {
		http.Error(w, "failed to encode version", http.StatusInternalServerError)
	}
}
