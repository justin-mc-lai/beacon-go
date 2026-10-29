package main

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/agentapi"
)

// newTestApp wires the real binding surface over temp stores, so the
// Wails-facing methods are exercised exactly as the UI calls them.
func newTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "beacon", "v2.0.0"), 0o755); err != nil {
		t.Fatalf("seed docs: %v", err)
	}
	app, err := newApp(agentapi.Options{
		Root:        root,
		StorePath:   filepath.Join(t.TempDir(), "ledger.db"),
		SessionPath: filepath.Join(t.TempDir(), "sessions.db"),
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(func() { _ = app.api.Close() })
	return app
}

// TestAppSessionAPIs covers TC-STREAM-003: the frozen test case that
// the shipped build listed but never implemented.
func TestAppSessionAPIs(t *testing.T) {
	app := newTestApp(t)

	sess, err := app.CreateSession("退款方案")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("CreateSession returned an empty id")
	}

	list, err := app.GetSessions()
	if err != nil {
		t.Fatalf("GetSessions: %v", err)
	}
	if len(list) != 1 || list[0].ID != sess.ID {
		t.Fatalf("GetSessions = %+v, want the created session", list)
	}

	msgs, err := app.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("new session must have no messages, got %d", len(msgs))
	}
}

func TestAppStartGoalCreatesRealRun(t *testing.T) {
	app := newTestApp(t)
	sess, err := app.CreateSession("demo")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	run, err := app.StartGoal(sess.ID, "demo-feature", "add a demo feature")
	if err != nil {
		t.Fatalf("StartGoal: %v", err)
	}
	if run.RunID == "" || len(run.StagePlan) == 0 {
		t.Fatalf("StartGoal returned no ledger facts: %+v", run)
	}

	msgs, err := app.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	found := false
	for _, m := range msgs {
		if m.CardType == "goal_run" {
			found = true
		}
	}
	if !found {
		t.Fatal("the goal_run card must be persisted for replay")
	}

	// The release gate on a fresh run cannot pass, and the binding must
	// report that instead of claiming a release.
	res, err := app.ReleaseGoal(sess.ID, run.RunID)
	if err != nil {
		t.Fatalf("ReleaseGoal: %v", err)
	}
	if res.Completed || res.Verdict == "pass" {
		t.Fatalf("fresh run must not release: %+v", res)
	}
}
