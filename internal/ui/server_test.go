package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsoleThreeColumnLayout(t *testing.T) {
	srv := NewServer(":0")
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	// Verify Three-column OpenMuse + DS-Bot components
	if !strings.Contains(body, "workspace-layout") {
		t.Errorf("missing workspace-layout container")
	}
	if !strings.Contains(body, "nav-pane") {
		t.Errorf("missing Col 1: nav-pane (Rooms)")
	}
	if !strings.Contains(body, "step-plan-container") {
		t.Errorf("missing Col 2: step-plan-container (OpenMuse Step Plan)")
	}
	if !strings.Contains(body, "inspector-pane") {
		t.Errorf("missing Col 3: inspector-pane (Truth/Tests/Evidence)")
	}
}

func TestWorkspaceFeaturesAPI(t *testing.T) {
	// Create temporary workspace with a mock feature
	tmpDir := t.TempDir()
	featDir := filepath.Join(tmpDir, "docs", "beacon", "v2.0.0", "features", "mock-feature")
	_ = os.MkdirAll(featDir, 0755)
	_ = os.WriteFile(filepath.Join(featDir, "truth.md"), []byte("# Mock Truth L0"), 0644)
	_ = os.WriteFile(filepath.Join(featDir, "tests.md"), []byte("# Mock Tests TC"), 0644)

	srv := NewServer(":0")
	srv.SetWorkdir(tmpDir)
	handler := srv.Handler()

	// 1. Test /api/workspace/features list
	req := httptest.NewRequest(http.MethodGet, "/api/workspace/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res map[string]any
	_ = json.NewDecoder(w.Body).Decode(&res)
	if res["count"].(float64) < 1 {
		t.Fatalf("expected at least 1 feature, got %v", res["count"])
	}

	// 2. Test /api/workspace/feature?slug=mock-feature detail
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/workspace/feature?slug=mock-feature", nil)
	wDetail := httptest.NewRecorder()
	handler.ServeHTTP(wDetail, reqDetail)

	if wDetail.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wDetail.Code)
	}
	var detail FeatureDetail
	_ = json.NewDecoder(wDetail.Body).Decode(&detail)
	if !strings.Contains(detail.TruthMD, "Mock Truth L0") {
		t.Errorf("unexpected TruthMD content: %s", detail.TruthMD)
	}
	if !strings.Contains(detail.TestsMD, "Mock Tests TC") {
		t.Errorf("unexpected TestsMD content: %s", detail.TestsMD)
	}
}

func TestWailsConfigValid(t *testing.T) {
	// Verify wails.json exists and parses correctly
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, "..", "..", "wails.json")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(cwd, "wails.json")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg map[string]any
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("invalid wails.json format: %v", err)
		}
		if cfg["name"] != "beacon-desktop" {
			t.Errorf("expected app name beacon-desktop, got %v", cfg["name"])
		}
	}
}
