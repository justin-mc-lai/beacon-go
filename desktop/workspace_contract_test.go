package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC-DESKTOP-003: the Wails project configuration exists and names the
// real build entry. A missing config means the desktop app is not a
// reproducible deliverable, only a local `go run`.
func TestWailsConfigValid(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "wails.json"))
	if err != nil {
		t.Fatalf("wails.json missing: %v", err)
	}
	var cfg struct {
		Name           string `json:"name"`
		OutputFilename string `json:"outputfilename"`
		Version        string `json:"version"`
		Info           struct {
			ProductName string `json:"productName"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("wails.json is not valid JSON: %v", err)
	}
	if cfg.Name == "" || cfg.OutputFilename == "" || cfg.Version == "" {
		t.Fatalf("wails.json is incomplete: %+v", cfg)
	}
	if cfg.Info.ProductName == "" {
		t.Error("wails.json info.productName is empty")
	}
}

// AC-STREAM-001/003/004: the single embedded shell reads sessions from
// the backend (not a hardcoded list), replays history on switch, and
// renders a stream in typing mode from the Wails event channel.
func TestShellSessionAndStreamWiring(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	shell := string(raw)
	for _, want := range []string{
		`invoke("GetSessions")`,
		`invoke("GetSessionMessages"`,
		`EventsOn("chat:stream:chunk"`,
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("shell is missing %s (session/stream wiring)", want)
		}
	}
	if strings.Contains(shell, "fetch(") || strings.Contains(shell, "EventSource(") {
		t.Error("shell must use Wails bindings/events only; fetch/EventSource is the deleted web path")
	}
}
