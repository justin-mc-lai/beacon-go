// intent-contract-bridge (plan stage): executable acceptance seeds.
//
// These tests are the feature's acceptance seed, not its proof of
// delivery: they FAIL on main today and must be turned green by the
// implement round without editing what they assert.
//
//	AC-002  a kernel-driven run without a confirmed intent is refused
//	AC-003  resume keeps the utterance and lands it in a run event
//
// They use only behaviour that already exists (CreateRun, Resume,
// Trace), so they compile against main and fail by assertion.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// Strings the contract speaks. They are local to this seed file so the
// red tests do not depend on production symbols that do not exist yet.
const (
	seedReasonIntentMissing = "goal_intent_missing"
	seedEventIntentRecorded = "intent_recorded"
	seedEventGoalResumed    = "goal_resumed"
	seedIntentSchema        = "beacon-intent-v1"
)

// seedWriteIntent writes the intent file the kernel is expected to
// read: sibling of truth.md, schema beacon-intent-v1, with the given
// confirmation state (AC-002 judges a confirmed, non-empty intent).
func seedWriteIntent(t *testing.T, dir, slug, restated string, confirmed bool) string {
	t.Helper()
	doc := map[string]any{
		"schema":       seedIntentSchema,
		"feature_slug": slug,
		"version":      "v2.0.0",
		"truth_path":   "truth.md",
		"recorded_at":  "2026-11-11T00:00:00Z",
		"recorded_by":  "seed",
		"confirmed":    confirmed,
		"briefing": map[string]any{
			"original_prompt":  "seed the intent contract",
			"restated_intent":  restated,
			"identified_risks": []string{"automation without intent cannot be reviewed"},
			"boundaries":       []string{"no release change"},
			"proposed_slugs":   []string{slug},
			"confirmed":        confirmed,
			"degraded":         false,
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "intent.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedConfirmedIntent writes the intent the kernel is expected to
// accept: sibling of truth.md, schema beacon-intent-v1, confirmed.
func seedConfirmedIntent(t *testing.T, dir, slug, restated string) string {
	t.Helper()
	return seedWriteIntent(t, dir, slug, restated, true)
}

// seedRunCount is the ListByFeature snapshot a refusal must not grow.
func seedRunCount(t *testing.T, env testEnv) int {
	t.Helper()
	runs, err := env.st.Runs().ListByFeature(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	return len(runs)
}

func seedExecutorOpts(root, mode string, exec *ExecutorConfig) CreateRunOptions {
	return CreateRunOptions{
		ProjectRoot: root, FeatureSlug: "feat-a",
		ExecutorID: "maker", ExecutorMode: mode, Exec: exec,
	}
}

// seedTracePayloads returns the decoded payload of every event of one
// type, in trace order.
func seedTracePayloads(t *testing.T, trace TraceBundle, eventType string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ev := range trace.Events {
		if ev.EventType != eventType {
			continue
		}
		p := map[string]any{}
		if len(ev.Payload) > 0 {
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Fatalf("%s payload is not JSON: %v (%s)", eventType, err, ev.Payload)
			}
		}
		out = append(out, p)
	}
	return out
}

// seedAssertResumePayload checks the contract fields of one
// goal_resumed payload. The empty-utterance key absence is checked by
// the caller.
func seedAssertResumePayload(t *testing.T, p map[string]any, rv ResumeView, wantUtterance string, wantRecorded bool) {
	t.Helper()
	if action, ok := p["resume_action"].(string); !ok || action == "" {
		t.Fatalf("%s payload has no resume_action: %v", seedEventGoalResumed, p)
	}
	recorded, ok := p["utterance_recorded"].(bool)
	if !ok || recorded != wantRecorded {
		t.Fatalf("%s utterance_recorded = %v (bool ok=%v), want %v: %v", seedEventGoalResumed, p["utterance_recorded"], ok, wantRecorded, p)
	}
	if wantRecorded {
		if got, ok := p["utterance"].(string); !ok || got != wantUtterance {
			t.Fatalf("%s utterance = %v, want the exact resume text %q", seedEventGoalResumed, p["utterance"], wantUtterance)
		}
	}
	if got, ok := p["stage"].(string); !ok || got == "" || got != rv.CurrentStage {
		t.Fatalf("%s stage = %v, want %q", seedEventGoalResumed, p["stage"], rv.CurrentStage)
	}
	if got, ok := p["loop_stage"].(string); !ok || got == "" || got != rv.LoopStage {
		t.Fatalf("%s loop_stage = %v, want %q", seedEventGoalResumed, p["loop_stage"], rv.LoopStage)
	}
}

// AC-002: a run whose stages the kernel drives itself must not exist
// without a confirmed intent. Every auto-driven selector is covered;
// manual and external stay open; an empty or unconfirmed intent is not
// an intent.
func TestKernelDrivenRunWithoutIntentIsRefused(t *testing.T) {
	driven := []struct {
		name string
		opts func(root string) CreateRunOptions
	}{
		{"recording", func(root string) CreateRunOptions {
			return seedExecutorOpts(root, ExecutorModeRecording, nil)
		}},
		{"subprocess", func(root string) CreateRunOptions {
			return seedExecutorOpts(root, ExecutorModeSubprocess, nil)
		}},
		{"subprocess_profile", func(root string) CreateRunOptions {
			return seedExecutorOpts(root, ExecutorModeSubprocess+":codex", nil)
		}},
		{"ralph", func(root string) CreateRunOptions {
			return seedExecutorOpts(root, ExecutorModeRalph, nil)
		}},
		{"exec_config", func(root string) CreateRunOptions {
			return seedExecutorOpts(root, "", &ExecutorConfig{Argv: []string{"true"}})
		}},
	}
	for _, tc := range driven {
		tc := tc
		t.Run("refused_"+tc.name, func(t *testing.T) {
			env := newTestEnv(t, "active")
			before := seedRunCount(t, env)
			if _, err := env.eng.CreateRun(tc.opts(env.root)); err == nil {
				t.Fatalf("kernel-driven %s run created without a confirmed intent; want refusal %s before any run row exists", tc.name, seedReasonIntentMissing)
			} else {
				if rc, ok := core.ReasonCodeOf(err); !ok || rc != seedReasonIntentMissing {
					t.Fatalf("kernel-driven %s refusal reason = %v (code %q, ok=%v); want %s", tc.name, err, rc, ok, seedReasonIntentMissing)
				}
				if !strings.Contains(err.Error(), "truth intent record") {
					t.Fatalf("refusal for %s has no repair guidance (want mention of truth intent record): %v", tc.name, err)
				}
			}
			if after := seedRunCount(t, env); after != before {
				t.Fatalf("refused %s creation left a run row: before=%d after=%d", tc.name, before, after)
			}

			// Control: the same options are admitted once a confirmed
			// intent exists next to truth.md.
			seedConfirmedIntent(t, env.root, "feat-a", "drive the run from a recorded intent")
			run, err := env.eng.CreateRun(tc.opts(env.root))
			if err != nil {
				t.Fatalf("confirmed intent must admit the %s run: %v", tc.name, err)
			}
			if run.CurrentStage != StagePlan {
				t.Fatalf("admitted %s run stage = %q, want %q", tc.name, run.CurrentStage, StagePlan)
			}
		})
	}

	// Controls: manual and external do not auto-drive, so they stay
	// possible without intent.json.
	for _, mode := range []string{ExecutorModeManual, ExecutorModeExternal} {
		mode := mode
		t.Run("control_"+mode+"_without_intent", func(t *testing.T) {
			env := newTestEnv(t, "active")
			run, err := env.eng.CreateRun(seedExecutorOpts(env.root, mode, nil))
			if err != nil {
				t.Fatalf("%s run without intent must stay possible: %v", mode, err)
			}
			if run.CurrentStage != StagePlan {
				t.Fatalf("%s run stage = %q, want %q", mode, run.CurrentStage, StagePlan)
			}
		})
	}

	// Negatives: a file that is parseable is still not a confirmed
	// intent when it is unsigned or says nothing.
	negative := []struct {
		name  string
		write func(t *testing.T, root string)
	}{
		{"empty_restated_intent", func(t *testing.T, root string) {
			seedWriteIntent(t, root, "feat-a", "   ", true)
		}},
		{"unconfirmed_intent", func(t *testing.T, root string) {
			seedWriteIntent(t, root, "feat-a", "recorded but not signed", false)
		}},
		{"malformed_json", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "intent.json"), []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range negative {
		tc := tc
		t.Run("refused_"+tc.name, func(t *testing.T) {
			env := newTestEnv(t, "active")
			tc.write(t, env.root)
			before := seedRunCount(t, env)
			if _, err := env.eng.CreateRun(seedExecutorOpts(env.root, ExecutorModeRalph, nil)); err == nil {
				t.Fatalf("kernel-driven run created with %s; want refusal %s before any run row exists", tc.name, seedReasonIntentMissing)
			} else if rc, ok := core.ReasonCodeOf(err); !ok || rc != seedReasonIntentMissing {
				t.Fatalf("refusal with %s reason = %v (code %q, ok=%v); want %s", tc.name, err, rc, ok, seedReasonIntentMissing)
			}
			if after := seedRunCount(t, env); after != before {
				t.Fatalf("refused creation with %s left a run row: before=%d after=%d", tc.name, before, after)
			}
		})
	}
}

// AC-003: resume must not drop the utterance. Every resume appends a
// goal_resumed event; the text is exact when present, and an empty
// utterance is still recorded with utterance_recorded=false and no
// utterance key. The original goal_run_created utterance is untouched.
func TestResumeKeepsUtteranceInRunEvent(t *testing.T) {
	env := newTestEnv(t, "active")
	const createdUtterance = "先建意图桥"
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		Utterance: createdUtterance, ExecutorID: "host-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	const resumeUtterance = "继续：把 intent.json 写进 run 事件，别丢这句话"
	rv, err := env.eng.Resume(run.RunID, resumeUtterance)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if rv.ResumeAction == "" {
		t.Fatalf("resume action is empty")
	}
	if rv.ResumeAction == core.ResumeReplaySubstep {
		t.Fatalf("resume action = %q; this run is not interrupted, so the seed must prove goal_resumed lands even outside replay_substep", rv.ResumeAction)
	}
	trace, err := env.eng.Trace(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	resumed := seedTracePayloads(t, trace, seedEventGoalResumed)
	if len(resumed) != 1 {
		t.Fatalf("run %s has %d %s events after one Resume(%q); want exactly one", run.RunID, len(resumed), seedEventGoalResumed, resumeUtterance)
	}
	seedAssertResumePayload(t, resumed[0], rv, resumeUtterance, true)

	created := seedTracePayloads(t, trace, "goal_run_created")
	if len(created) == 0 {
		t.Fatalf("run %s has no goal_run_created event", run.RunID)
	}
	if got, _ := created[0]["utterance"].(string); got != createdUtterance {
		t.Fatalf("goal_run_created utterance = %q, want the original text untouched", got)
	}

	// Empty utterance: the event is still appended, marked as not
	// recorded, and carries no utterance key at all.
	rv2, err := env.eng.Resume(run.RunID, "")
	if err != nil {
		t.Fatalf("Resume with empty utterance: %v", err)
	}
	trace2, err := env.eng.Trace(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	resumed2 := seedTracePayloads(t, trace2, seedEventGoalResumed)
	if len(resumed2) != 2 {
		t.Fatalf("run %s has %d %s events after two Resume calls; want one per call", run.RunID, len(resumed2), seedEventGoalResumed)
	}
	seedAssertResumePayload(t, resumed2[1], rv2, "", false)
	if _, ok := resumed2[1]["utterance"]; ok {
		t.Fatalf("%s empty-utterance payload must not carry an utterance key: %v", seedEventGoalResumed, resumed2[1])
	}
	created2 := seedTracePayloads(t, trace2, "goal_run_created")
	if len(created2) == 0 {
		t.Fatalf("run %s lost its goal_run_created event", run.RunID)
	}
	if got, _ := created2[0]["utterance"].(string); got != createdUtterance {
		t.Fatalf("goal_run_created utterance = %q after resumes, want the original text untouched", got)
	}
}
