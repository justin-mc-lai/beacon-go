package core

import "fmt"

// Loop failure-mode catalog (P4c; v1.6.5 beacon-loop-failure-mode
// -catalog-v165). The v1 catalog is a governed markdown artifact
// (docs/beacon/archive/v1.6.5/execution/loop-failure-modes.md); the
// Go port keeps the same six modes as versioned data in the single
// reason-code home, each mode carrying the v1 required fields (id,
// severity, symptom, cause, mitigation, route, evidence
// expectation — v1 AC-002) plus the Go reason codes that signal it,
// so existing surfaces (longrun-doctor) can EXPLAIN a sick run in
// catalog terms instead of inventing a second vocabulary.
//
// The catalog does not replace reason codes (v1 Non-Goal) and never
// marks QA or release pass: it is explanatory data only.

// FailureMode severities (v1 UD-002).
const (
	SeverityS1Annoying = "S1"
	SeverityS2Harmful  = "S2"
	SeverityS3Critical = "S3"
)

// FailureMode is one catalog entry.
type FailureMode struct {
	ID                  string   `json:"id"`
	Severity            string   `json:"severity"`
	Symptom             string   `json:"symptom"`
	Cause               string   `json:"cause"`
	Mitigation          string   `json:"mitigation"`
	Route               string   `json:"route"`
	EvidenceExpectation string   `json:"evidence_expectation"`
	ReasonCodes         []string `json:"reason_codes,omitempty"`
}

// failureModeCatalog is the governed list; content mirrors the v1
// catalog file, with routes kept as the v1 skill-route names (they
// name the responsible surface, they are not Go commands).
var failureModeCatalog = []FailureMode{
	{
		ID: "stale-evidence-manifest", Severity: SeverityS1Annoying,
		Symptom:             "Evidence manifest does not reflect the latest generated artifacts.",
		Cause:               "A runtime artifact changed without refreshing the manifest projection.",
		Mitigation:          "Rebuild or refresh evidence manifest before QA or release.",
		Route:               "beacon-gov-doctor",
		EvidenceExpectation: "Manifest diff or refreshed manifest path.",
		ReasonCodes:         []string{ReasonStageExitQAEvidenceMiss},
	},
	{
		ID: "truth-drift", Severity: SeverityS2Harmful,
		Symptom:             "Implementation or generated projections disagree with frozen feature truth.",
		Cause:               "Post-freeze edits bypassed change/refreeze or legacy truth was treated as authority.",
		Mitigation:          "Route through change/refreeze or repair projections back to feature-package truth.",
		Route:               "beacon-gen-change",
		EvidenceExpectation: "Truth-map resolve output and repaired package references.",
		ReasonCodes:         []string{ReasonStageExitTruthNotFrozen, ReasonTruthChangePending, ReasonFreezeAckRequired},
	},
	{
		ID: "git-alignment-blocked", Severity: SeverityS2Harmful,
		Symptom:             "Implementation, QA, or release cannot enter the governed branch/worktree.",
		Cause:               "Dirty worktree, wrong branch, missing admission artifact, or invalid worktree lane.",
		Mitigation:          "Run workspace admission and repair the branch/worktree state before writes.",
		Route:               "beacon-gov-branch",
		EvidenceExpectation: "Passing workspace admission JSON.",
		ReasonCodes: []string{
			ReasonWorktreeMainDirty, ReasonWorktreeMergeDirection, ReasonWorktreeMissing,
			ReasonAdmissionWorktreeMiss, ReasonAdmissionWorktreeMism, ReasonGoalImplementAdmissBlock,
		},
	},
	{
		ID: "infinite-fix-loop", Severity: SeverityS2Harmful,
		Symptom:             "The loop repeats repairs without new evidence or failing test movement.",
		Cause:               "No attempt cap, no failure classification, or no route escalation.",
		Mitigation:          "Stop after capped attempts and route to planner/evaluator finding review.",
		Route:               "beacon-eval-qa",
		EvidenceExpectation: "Attempt ledger with unchanged failure signature.",
		ReasonCodes:         []string{ReasonMaxRepairLoopsExceeded},
	},
	{
		ID: "protected-path-attempt", Severity: SeverityS2Harmful,
		Symptom:             "Automatic write surface attempts to edit secrets, auth, payment, migration, or environment files.",
		Cause:               "Broad implementation scope touched a protected path without explicit human handling.",
		Mitigation:          "Fail closed and ask for a scoped human decision or manual edit route.",
		Route:               "human-override",
		EvidenceExpectation: "Block payload with protected_path_denylist reason code.",
		ReasonCodes:         []string{ReasonProtectedPathDenylist},
	},
	{
		ID: "unattended-release-without-qa", Severity: SeverityS3Critical,
		Symptom:             "Release is attempted without layered QA evidence.",
		Cause:               "Dashboard or status projection was mistaken for release proof.",
		Mitigation:          "Block release and complete QA evidence chain first.",
		Route:               "beacon-eval-qa",
		EvidenceExpectation: "QA verdict artifacts and release scorecard inputs.",
		ReasonCodes:         []string{ReasonReleaseRequiresHuman, ReasonReleaseAwaitingComplete},
	},
}

// FailureModeCatalog returns the governed catalog (a copy).
func FailureModeCatalog() []FailureMode {
	out := make([]FailureMode, len(failureModeCatalog))
	copy(out, failureModeCatalog)
	return out
}

// ValidateFailureModeCatalog mirrors the v1 catalog validator
// (AC-002): every mode must carry all required fields, a governed
// severity, and a unique id.
func ValidateFailureModeCatalog() error {
	seen := map[string]bool{}
	for _, m := range failureModeCatalog {
		if m.ID == "" || m.Symptom == "" || m.Cause == "" || m.Mitigation == "" ||
			m.Route == "" || m.EvidenceExpectation == "" {
			return fmt.Errorf("failure mode %q is missing a required field (id, severity, symptom, cause, mitigation, route, evidence expectation)", m.ID)
		}
		switch m.Severity {
		case SeverityS1Annoying, SeverityS2Harmful, SeverityS3Critical:
		default:
			return fmt.Errorf("failure mode %q has ungoverned severity %q", m.ID, m.Severity)
		}
		if seen[m.ID] {
			return fmt.Errorf("failure mode id %q is duplicated", m.ID)
		}
		seen[m.ID] = true
	}
	return nil
}

// FailureModesForReasons maps observed reason codes to the catalog
// modes they signal, in catalog order, each mode at most once.
func FailureModesForReasons(codes []string) []FailureMode {
	if len(codes) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, c := range codes {
		set[c] = true
	}
	var out []FailureMode
	for _, m := range failureModeCatalog {
		for _, rc := range m.ReasonCodes {
			if set[rc] {
				out = append(out, m)
				break
			}
		}
	}
	return out
}
