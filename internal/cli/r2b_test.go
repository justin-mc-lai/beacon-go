package cli

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// R2b CLI surfaces: release check (H7), goal quota/decide/gate/
// fork/spend/error-recovery (H6/M8/M9), truth map (M16), review
// (M15) — one seeded project, thin wrappers over the engine.
func TestR2BSurfaces(t *testing.T) {
	root, _ := seedGoalProject(t)

	// release check: the seeded feature has no QA evidence and no
	// human confirmation -> not release-ready, with those checks
	// named; an invalid profile is refused with its code.
	out, err := runCLI(t, "release", "check", "--project", root, "--json")
	if err != nil {
		t.Fatalf("release check: %v (out %s)", err, out)
	}
	doc := parseDoc(t, out)
	if doc["release_ready"] != false {
		t.Fatalf("release check doc = %v", doc)
	}
	failed, _ := doc["failed_checks"].([]any)
	joined := ""
	for _, f := range failed {
		joined += f.(string) + ","
	}
	if !strings.Contains(joined, "qa_evidence_present") || !strings.Contains(joined, "human_release_confirmed") {
		t.Errorf("failed checks = %s", joined)
	}
	if _, err := runCLI(t, "release", "check", "--project", root, "--profile", "banana", "--json"); err == nil {
		t.Error("invalid profile must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonReleaseProfileInvalid {
		t.Errorf("profile reason = %q", rc)
	}

	// truth map verify: no on-disk package -> blocked, unresolved.
	out, err = runCLI(t, "truth", "map", "verify", "--project", root, "--feature", "feat-a", "--json")
	if err != nil {
		t.Fatalf("truth map verify: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["status"] != "blocked" || doc["authority_source"] != "cli_static_resolver_plus_markdown_feature_package" {
		t.Errorf("truth map doc = %v", doc)
	}

	// A goal run to hang the records on.
	out, err = runCLI(t, "goal", "run", "build the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	// quota: configure + status.
	out, err = runCLI(t, "goal", "quota", "configure", "--run-id", runID, "--allowed-slots", "3", "--json")
	if err != nil {
		t.Fatalf("quota configure: %v", err)
	}
	cfg, _ := parseDoc(t, out)["config"].(map[string]any)
	if cfg["allowed_slots"].(float64) != 3 {
		t.Errorf("quota config = %v", cfg)
	}
	out, err = runCLI(t, "goal", "quota", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("quota status: %v", err)
	}
	dec, _ := parseDoc(t, out)["decision"].(map[string]any)
	if dec["decision"] != "run" {
		t.Errorf("quota decision = %v", dec)
	}

	// decide + gate: a decided key is never asked twice.
	if _, err := runCLI(t, "goal", "decide", "--run-id", runID, "--key", "ship_it", "--value", "yes", "--json"); err != nil {
		t.Fatalf("decide: %v", err)
	}
	out, err = runCLI(t, "goal", "gate", "card", "--run-id", runID, "--key", "ship_it", "--question", "ship?", "--json")
	if err != nil {
		t.Fatalf("gate card: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["duplicate"] != true || doc["cached_answer"] != "yes" {
		t.Errorf("gate card doc = %v", doc)
	}
	out, err = runCLI(t, "goal", "gate", "check", "--run-id", runID, "--key", "ship_it")
	if err != nil || !strings.Contains(out, "gate resolved") {
		t.Errorf("gate check = %q, %v", out, err)
	}

	// fork + spend guards.
	if _, err := runCLI(t, "goal", "fork", "--run-id", runID, "--stage", "truth", "--decision", "fork_b", "--json"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	if _, err := runCLI(t, "goal", "spend", "--run-id", runID, "--source", "bogus", "--digest", "d1", "--writeback-validated"); err == nil {
		t.Error("bogus spend source must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonSpendSourceNotAllowed {
		t.Errorf("spend reason = %q", rc)
	}
	out, err = runCLI(t, "goal", "spend", "--run-id", runID, "--source", "delivery_completion", "--digest", "d1", "--writeback-validated", "--json")
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if parseDoc(t, out)["spent"].(float64) != 1 {
		t.Errorf("spend doc = %s", out)
	}

	// review quality: fails on the unverified feature and writes
	// back (recorded) through the H5 channel.
	out, err = runCLI(t, "review", "quality", "--project", root, "--feature", "feat-a", "--json")
	if err != nil {
		t.Fatalf("review quality: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["status"] != "fail" || doc["review_recorded"] != true {
		t.Errorf("review doc = %v", doc)
	}

	// error-recovery through the one door (last: it blocks the run).
	out, err = runCLI(t, "goal", "error-recovery", "--run-id", runID, "--reason-code", "executor_died", "--json")
	if err != nil {
		t.Fatalf("error-recovery: %v", err)
	}
	if parseDoc(t, out)["blocked"] != true {
		t.Errorf("error-recovery doc = %s", out)
	}
}
