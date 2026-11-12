// Implement work-gate tests (implement-work-gate AC-001..006) at the
// JUDGEMENT layer: facts in, item out. No store, no filesystem, no
// git — the git-backed behaviour lives in the orchestrator's
// TestImplementGate* tests.
package gate

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// workPass are facts that satisfy AC-001 (a dirty or moved work tree).
func workPass() ImplementWorkFacts {
	return ImplementWorkFacts{GateEnabled: true, BaselineCommit: "aaaa", HeadCommit: "bbbb", HeadMoved: true}
}

func findItem(ev Evaluation, check string) (CheckItem, bool) {
	for _, c := range ev.Checks {
		if c.Check == check {
			return c, true
		}
	}
	return CheckItem{}, false
}

func checkNames(ev Evaluation) []string {
	names := make([]string, 0, len(ev.Checks))
	for _, c := range ev.Checks {
		names = append(names, c.Check)
	}
	return names
}

// ---- AC-001: the check exists and passes when work exists ----

func TestImplementWorkRecordedPassesOnMovement(t *testing.T) {
	ev := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: true, Work: workPass()}, "qa-bot")
	if ev.Verdict != VerdictPass {
		t.Fatalf("moved HEAD must pass, got %+v", ev)
	}
	item, ok := findItem(ev, CheckImplementWorkRecorded)
	if !ok || !item.Applicable || !item.OK {
		t.Fatalf("implement_work_recorded must be an applicable ok item, got %+v (ok=%v)", item, ok)
	}
	dirty := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: true, Work: ImplementWorkFacts{
		GateEnabled: true, BaselineCommit: "aaaa", HeadCommit: "aaaa", Dirty: true, DirtyCount: 2,
	}}, "qa-bot")
	if dirty.Verdict != VerdictPass {
		t.Fatalf("dirty work tree must pass even with HEAD unmoved, got %+v", dirty)
	}
}

// ---- AC-002: clean + unmoved is blocked with implement_no_work ----

func TestImplementWorkRecordedBlocksEmptyStage(t *testing.T) {
	ev := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: true, Work: ImplementWorkFacts{
		GateEnabled: true, BaselineCommit: "aaaa", HeadCommit: "aaaa",
	}}, "qa-bot")
	if ev.Verdict != VerdictBlocked {
		t.Fatalf("empty implement stage must block, got %+v", ev)
	}
	if len(ev.ReasonCodes) == 0 || ev.ReasonCodes[0] != core.ReasonImplementNoWork {
		t.Fatalf("want implement_no_work as the headline, got %v", ev.ReasonCodes)
	}
	item, _ := findItem(ev, CheckImplementWorkRecorded)
	if item.OK || !item.Applicable || item.ReasonCode != core.ReasonImplementNoWork {
		t.Fatalf("empty-stage item must be an applicable failure, got %+v", item)
	}
}

// ---- AC-003 / AC-004: not_applicable legs never block, never pass-by-accident ----

func TestImplementWorkRecordedNotApplicableLegs(t *testing.T) {
	cases := []struct {
		name string
		work ImplementWorkFacts
		code string
	}{
		{"allow-empty", ImplementWorkFacts{GateEnabled: true, EmptyAllowed: true}, core.ReasonImplementEmptyAllowed},
		{"gate-off", ImplementWorkFacts{GateEnabled: false}, core.ReasonImplementWorkGateOff},
		{"unverified", ImplementWorkFacts{GateEnabled: true, Unverified: true, UnverifiedDetail: "not a git repository"}, core.ReasonImplementWorkUnverified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: true, Work: tc.work}, "qa-bot")
			if ev.Verdict != VerdictPass {
				t.Fatalf("%s must not block, got %+v", tc.name, ev)
			}
			if len(ev.ReasonCodes) != 0 {
				t.Fatalf("%s must not add reason codes to the gate, got %v", tc.name, ev.ReasonCodes)
			}
			item, _ := findItem(ev, CheckImplementWorkRecorded)
			if item.Applicable || !item.OK || item.ReasonCode != tc.code {
				t.Fatalf("%s item = %+v, want not_applicable + %s", tc.name, item, tc.code)
			}
			if item.Detail == "" {
				t.Fatalf("%s item must explain itself", tc.name)
			}
		})
	}
}

// ---- AC-005: only the implement gate gains an item ----

// TestOtherGatesUnchanged pins the per-item sets of the other gates:
// the implement work-evidence item appears in the implement gate ONLY.
func TestOtherGatesUnchanged(t *testing.T) {
	truthFreeze := EvaluateTruthFreeze(TruthFreezeFacts{
		TruthPresent: true, TruthNonEmpty: true, TestsPresent: true, TestsNonEmpty: true, FrozenMarker: true,
	}, "qa-bot")
	if got := strings.Join(checkNames(truthFreeze), ","); got != "truth_file_present,truth_file_nonempty,tests_file_present,tests_file_nonempty,frozen_marker,lifecycle_corroboration,no_pending_change" {
		t.Fatalf("truth_freeze check set changed: %s", got)
	}
	design := EvaluateDesign(DesignFacts{HasRun: true, Required: false}, "qa-bot")
	if got := strings.Join(checkNames(design), ","); got != "design_required" {
		t.Fatalf("design check set changed: %s", got)
	}
	qa := EvaluateQA(QAFacts{QAArtifactCount: 1, QAStageExecuted: true}, "qa-bot")
	// qa-surface-e2e AC-008/AC-009 deliberately extend the QA gate with
	// two fail-closed doors; the other three gates below stay pinned.
	if got := strings.Join(checkNames(qa), ","); got != "qa_artifacts_present,qa_stage_executed,qa_artifact_refs_valid,qa_autotest_recorded,qa_surface_evidence" {
		t.Fatalf("qa check set changed: %s", got)
	}
	passFact := GateFact{Present: true, Verdict: string(VerdictPass)}
	release := EvaluateRelease(ReleaseFacts{
		TruthFreeze: passFact, Design: passFact, Implement: passFact, QA: passFact,
		DesignApplicable: true, HumanConfirmed: true,
	}, "qa-bot")
	if got := strings.Join(checkNames(release), ","); got != "gate_truth_freeze_passed,gate_design_passed,gate_implement_passed,gate_qa_passed,truth_materials_fresh,human_confirmation" {
		t.Fatalf("release check set changed: %s", got)
	}
	for _, ev := range []Evaluation{truthFreeze, design, qa, release} {
		if _, found := findItem(ev, CheckImplementWorkRecorded); found {
			t.Fatalf("gate %s must not carry implement_work_recorded: %v", ev.GateType, checkNames(ev))
		}
	}

	// The implement gate gains EXACTLY one item, appended after the
	// admission items (their names and order are untouched).
	impl := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: true,
		Items: []CheckItem{okItem("truth_frozen", "frozen")}, Work: workPass()}, "qa-bot")
	if got := strings.Join(checkNames(impl), ","); got != "truth_frozen,"+CheckImplementWorkRecorded {
		t.Fatalf("implement check set = %s, want truth_frozen,%s", got, CheckImplementWorkRecorded)
	}
}
