// Planner review (P3c, AC-102): the multi-agent review of a stage
// plan / truth package. N reviewer profiles run as REAL subprocesses
// (the harness profile layer); each must print ONE JSON verdict line
// {"verdict":"pass"|"block","reasons":[...]} on stdout.
//
// Aggregation (fail-closed, no silent softening):
//   - any reviewer block with a non-empty reason -> overall block
//   - a block with NO reason is invalid input, not a veto and not a
//     pass — the lane counts invalid
//   - a reviewer that times out, exits non-zero, or prints no
//     parseable verdict line is INVALID — never a pass
//   - overall pass requires at least one valid lane and every valid
//     lane passing (with zero valid lanes the review cannot pass)
//
// AC-102 honesty: reviewers run SEQUENTIALLY in this build, so
// execution_mode is "multi_agent_sequential" — parallel execution is
// never claimed. Shape mirrors the v1.6 runtime's fields:
// reviewer_lanes, findings, synthesis, execution_mode,
// fallback_reason, runtime_evidence_only=true. The aggregate lands
// in gate_results as gate_type=planner_review plus a full-evidence
// artifact (the synthesis JSON) in artifact_refs.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// Reviewer lane verdicts.
const (
	LanePass    = "pass"
	LaneBlock   = "block"
	LaneInvalid = "invalid"
)

// ReviewerLane is one reviewer's lane record (AC-102
// reviewer_lanes element): identity, raw verdict, validity, timing.
type ReviewerLane struct {
	Reviewer  string   `json:"reviewer"`
	Verdict   string   `json:"verdict"` // pass | block | invalid
	Reasons   []string `json:"reasons,omitempty"`
	ExitCode  int      `json:"exit_code"`
	TimedOut  bool     `json:"timed_out,omitempty"`
	Detail    string   `json:"detail,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	RawStdout string   `json:"-"`
	// R2b (H5): the optional routing fields of the verdict line —
	// a lane may grade its block (severity P0..P3), recommend a
	// route, and demand a user decision. They feed the review
	// packet that record_review_decision routes on.
	Severity             string `json:"severity,omitempty"`
	RecommendedRoute     string `json:"recommended_route,omitempty"`
	UserDecisionRequired bool   `json:"user_decision_required,omitempty"`
}

// PlannerReviewView is the AC-102-shaped review result.
type PlannerReviewView struct {
	RunID               string         `json:"run_id"`
	Target              string         `json:"target"`
	Verdict             string         `json:"verdict"` // pass | blocked
	ReasonCode          string         `json:"reason_code,omitempty"`
	ReviewerLanes       []ReviewerLane `json:"reviewer_lanes"`
	Findings            []string       `json:"findings"`
	Synthesis           string         `json:"synthesis"`
	ExecutionMode       string         `json:"execution_mode"` // multi_agent_sequential
	FallbackReason      string         `json:"fallback_reason,omitempty"`
	RuntimeEvidenceOnly bool           `json:"runtime_evidence_only"` // always true (AC-102)
	GateResultID        string         `json:"gate_result_id,omitempty"`
	ArtifactID          string         `json:"artifact_id,omitempty"`
	// R2b (H5): the routed decision — the packet's graded facts and
	// where record_review_decision sent the run.
	HighestSeverity      string `json:"highest_severity,omitempty"`
	RecommendedRoute     string `json:"recommended_route,omitempty"`
	UserDecisionRequired bool   `json:"user_decision_required,omitempty"`
	RouteStage           string `json:"route_stage,omitempty"`
	RouteReasonCode      string `json:"route_reason_code,omitempty"`
}

// reviewerVerdict is the JSON contract a reviewer prints. The
// verdict/reasons pair is the original AC-102 contract; severity,
// recommended_route and user_decision_required are the optional
// R2b routing fields (a verdict line without them still parses —
// an ungraded block grades P0 at packet assembly, fail-closed).
type reviewerVerdict struct {
	Verdict              string   `json:"verdict"`
	Reasons              []string `json:"reasons"`
	Severity             string   `json:"severity,omitempty"`
	RecommendedRoute     string   `json:"recommended_route,omitempty"`
	UserDecisionRequired bool     `json:"user_decision_required,omitempty"`
}

// parseReviewerVerdict extracts the verdict: the LAST stdout line
// that parses as a JSON object with a verdict field wins (reviewers
// may narrate before answering). A block without reasons is invalid.
func parseReviewerVerdict(stdout string) (verdict string, reasons []string, ok bool) {
	v, parsed := parseReviewerVerdictFull(stdout)
	return v.Verdict, v.Reasons, parsed
}

// parseReviewerVerdictFull is parseReviewerVerdict with the full
// verdict line (routing fields included).
func parseReviewerVerdictFull(stdout string) (reviewerVerdict, bool) {
	lines := strings.Split(stdout, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var v reviewerVerdict
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			continue
		}
		switch v.Verdict {
		case LanePass:
			return v, true
		case LaneBlock:
			var nonempty []string
			for _, r := range v.Reasons {
				if strings.TrimSpace(r) != "" {
					nonempty = append(nonempty, r)
				}
			}
			if len(nonempty) == 0 {
				v.Verdict = LaneInvalid // answered, but an empty veto is not a veto
				v.Reasons = nil
				return v, true
			}
			v.Reasons = nonempty
			return v, true
		}
	}
	return reviewerVerdict{Verdict: LaneInvalid}, false
}

// reviewInputText assembles what the reviewers are shown: the run's
// stage plan (always) plus the target package files when the target
// names them. The text travels as the {prompt} of the reviewer
// profile and inside the stage-context file.
func (e *Engine) reviewInputText(run store.Run, m *runMeta, slug, summary string) (string, string, error) {
	planRaw, err := json.MarshalIndent(m.StagePlan, "", "  ")
	if err != nil {
		return "", "", err
	}
	var b strings.Builder
	b.WriteString("Review the following beacon goal stage plan for run " + run.ID + ".\n")
	fmt.Fprintf(&b, "Feature: %s\nUtterance: %s\n", slug, m.Utterance)
	if summary != "" {
		fmt.Fprintf(&b, "Author summary: %s\n", summary)
	}
	b.WriteString("\nStage plan:\n")
	b.Write(planRaw)
	target := "stage_plan"
	if fc, ok, err := e.featureContextOf(run.FeatureID); err == nil && ok {
		pkgRoot := filepath.Join(fc.Project.RootPath, fc.Version.DocsRoot, "features", fc.Feature.Slug)
		for _, name := range []string{"truth.md", "tests.md"} {
			raw, rerr := os.ReadFile(filepath.Join(pkgRoot, name))
			if rerr != nil {
				continue
			}
			target = "truth_package"
			fmt.Fprintf(&b, "\n--- %s ---\n%s\n", name, string(raw))
		}
	}
	b.WriteString("\nAnswer with ONE JSON line: {\"verdict\":\"pass\"|\"block\",\"reasons\":[\"...\"]}\n")
	return b.String(), target, nil
}

// PlannerReview runs the review for a run: reviewer profile +
// reviewer command come from cfg (falling back to the run's
// recorded Exec). Reviewer count N is len of the SAME profile run N
// times? No — N distinct slots are not simulated: the review runs
// the configured reviewer profile once per --reviewer occurrence;
// the engine API takes the resolved profiles list directly
// ([]ReviewerSpec), so each lane is an explicitly configured
// reviewer, never a copy dressed as a crowd.
type ReviewerSpec struct {
	Name    string   // reviewer identity (defaults to the profile name)
	Profile string   // profile name (defaults: codex-style)
	Argv    []string // explicit command override (argv, no shell)
}

// PlannerReviewerSpecs is the API input type (kept as an alias for
// call-site readability).
type PlannerReviewerSpecs = []ReviewerSpec

// RunPlannerReview executes the review and persists the aggregate:
// one gate_results row (gate_type=planner_review) + one
// planner_review lane event + the synthesis artifact.
func (e *Engine) RunPlannerReview(ctx context.Context, runID string, specs PlannerReviewerSpecs, summary string, timeoutSec int) (PlannerReviewView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return PlannerReviewView{}, err
	}
	if len(specs) == 0 {
		return PlannerReviewView{}, core.NewError(core.ReasonPlannerReviewerUnconfig, "goal.RunPlannerReview",
			"no reviewer configured (set goal run --reviewer-cmd or planner-review --reviewer-cmd); a review with no reviewers cannot pass")
	}
	inputText, target, err := e.reviewInputText(run, m, slug, summary)
	if err != nil {
		return PlannerReviewView{}, err
	}
	workdir, err := e.executorWorkdir(run, m)
	if err != nil {
		return PlannerReviewView{}, err
	}
	dir, err := e.artifactDirFor(run)
	if err != nil {
		return PlannerReviewView{}, err
	}
	sc := harness.StageContext{
		RunID: run.ID, FeatureID: run.FeatureID, FeatureSlug: slug,
		Stage: "planner_review", Workdir: workdir, StagePlan: m.StagePlan,
		Prompt: inputText,
	}
	ctxPath, err := harness.WriteStageContext(dir, sc)
	if err != nil {
		return PlannerReviewView{}, err
	}
	view := PlannerReviewView{
		RunID: runID, Target: target, ExecutionMode: "multi_agent_sequential",
		RuntimeEvidenceOnly: true, Verdict: "blocked",
	}
	var findings []string
	var packetFindings []core.ReviewFinding
	valid, blocks, invalid := 0, 0, 0
	for i, spec := range specs {
		lane := e.runReviewerLane(ctx, run, workdir, ctxPath, inputText, spec, i+1, timeoutSec)
		view.ReviewerLanes = append(view.ReviewerLanes, lane)
		switch lane.Verdict {
		case LanePass:
			valid++
		case LaneBlock:
			valid++
			blocks++
			// An ungraded veto grades P0 at packet assembly
			// (fail-closed: the lane stopped the work but did not
			// say how badly — the routing treats it as blocking).
			sev := strings.ToUpper(strings.TrimSpace(lane.Severity))
			if sev == "" {
				sev = "P0"
			}
			for _, r := range lane.Reasons {
				findings = append(findings, lane.Reviewer+": "+r)
				packetFindings = append(packetFindings, core.ReviewFinding{
					Severity: sev, Message: r, Source: lane.Reviewer,
				})
			}
		default:
			invalid++
			findings = append(findings, lane.Reviewer+": invalid ("+lane.Detail+")")
		}
	}
	view.Findings = findings
	switch {
	case blocks > 0:
		view.Verdict = "blocked"
		view.ReasonCode = core.ReasonPlannerReviewGateBlocked
		view.Synthesis = fmt.Sprintf("%d of %d valid reviewer(s) blocked the plan; see findings", blocks, valid)
	case valid == 0:
		view.Verdict = "blocked"
		view.ReasonCode = core.ReasonPlannerReviewerInvalid
		view.Synthesis = "no reviewer produced a valid verdict; a review that did not happen cannot pass"
	case invalid > 0:
		view.Verdict = "pass"
		view.Synthesis = fmt.Sprintf("all %d valid reviewer(s) passed; %d invalid lane(s) recorded and ignored for the verdict (never counted as pass)", valid, invalid)
	default:
		view.Verdict = "pass"
		view.Synthesis = fmt.Sprintf("all %d reviewer(s) passed", valid)
	}
	// Persist: gate_results row + synthesis artifact + lane event.
	codes := []string{}
	if view.ReasonCode != "" {
		codes = []string{view.ReasonCode}
	}
	rec, err := e.store.Gates().Record(store.GateResult{
		FeatureID: run.FeatureID, GateType: "planner_review",
		Verdict: view.Verdict, ReasonCodes: codes,
	})
	if err != nil {
		return PlannerReviewView{}, err
	}
	view.GateResultID = rec.ID
	synRaw, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return PlannerReviewView{}, err
	}
	if ref, err := e.writeCapture(run, dir, "planner_review", "planner-review.json", string(synRaw)); err == nil {
		view.ArtifactID = ref.ID
	} else {
		return PlannerReviewView{}, err
	}
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	m.Implement.PlannerReview = &PlannerReviewRecord{
		Verdict: view.Verdict, ReasonCode: view.ReasonCode,
		Reviewers: len(specs), Invalid: invalid, Mode: view.ExecutionMode,
		At: nowISO(), Summary: summary,
	}
	_ = e.appendEvent(run, "planner_review", map[string]any{
		"verdict": view.Verdict, "reason_code": view.ReasonCode,
		"reviewers": len(specs), "invalid": invalid, "execution_mode": view.ExecutionMode,
		"runtime_evidence_only": true, "gate_result_id": rec.ID, "target": target,
	})
	// H5 (R2b): the conclusion is ROUTED back into the loop state
	// (record_review_decision) — a graded block can stop implement,
	// a drift route sends the run to the truth change flow, a
	// user-decision demand parks it for a human. The packet's route
	// is the one the strongest blocking lane recommended (ties:
	// first declared); the synthesis marker is "at least one lane
	// produced a valid verdict line" — with zero valid lanes the
	// packet is empty and the routing fail-closes to
	// error_recovery, exactly like the source's incomplete test.
	packet := reviewPacketOf(view, packetFindings, rec.ID)
	if rec2, rerr := e.recordReviewDecisionOn(run, m, packet, "planner_review"); rerr == nil {
		view.HighestSeverity = rec2.HighestSeverity
		view.RecommendedRoute = rec2.RecommendedRoute
		view.UserDecisionRequired = rec2.UserDecisionRequired
		view.RouteStage = rec2.NextStage
		view.RouteReasonCode = rec2.ReasonCode
	} else {
		return PlannerReviewView{}, rerr
	}
	if err := e.saveRun(run, m); err != nil {
		return PlannerReviewView{}, err
	}
	// Re-marshal the artifact now that ArtifactID is known? The
	// artifact holds the synthesis; its own id rides in the event —
	// keep the file stable instead of rewriting it (hash honesty).
	return view, nil
}

// runReviewerLane executes ONE reviewer and judges its output.
func (e *Engine) runReviewerLane(ctx context.Context, run store.Run, workdir, ctxPath, prompt string, spec ReviewerSpec, slot, timeoutSec int) ReviewerLane {
	profileName := spec.Profile
	if profileName == "" {
		profileName = "codex-style"
	}
	prof, ok := harness.LookupProfile(profileName)
	if !ok {
		return ReviewerLane{Reviewer: specName(spec, profileName), Verdict: LaneInvalid, ExitCode: -1,
			Detail: core.ReasonExecutorProfileUnknown + ": " + profileName}
	}
	prof = prof.WithCommand(spec.Argv)
	name := specName(spec, prof.Name)
	backend := harness.NewProfileBackend(prof, ctxPath, prompt, harnessTimeout(timeoutSec))
	res, err := backend.Iterate(ctx, harness.IterationRequest{
		RunID: run.ID, FeatureID: run.FeatureID, Stage: "planner_review", Iteration: slot, Workdir: workdir,
	})
	lane := ReviewerLane{Reviewer: name, ExitCode: -1}
	if err != nil {
		lane.Verdict = LaneInvalid
		if rc, ok2 := core.ReasonCodeOf(err); ok2 {
			lane.Detail = rc
		} else {
			lane.Detail = err.Error()
		}
		return lane
	}
	lane.ExitCode = res.ExitCode
	lane.TimedOut = res.TimedOut
	lane.Summary = res.Summary
	lane.RawStdout = res.StdoutText
	switch {
	case res.TimedOut:
		lane.Verdict = LaneInvalid
		lane.Detail = core.ReasonExecutorTimedOut
	case res.ExitCode != 0:
		lane.Verdict = LaneInvalid
		lane.Detail = core.ReasonExecutorExitNonZero
	default:
		full, parsed := parseReviewerVerdictFull(res.StdoutText)
		lane.Verdict = full.Verdict
		lane.Reasons = full.Reasons
		lane.Severity = full.Severity
		lane.RecommendedRoute = full.RecommendedRoute
		lane.UserDecisionRequired = full.UserDecisionRequired
		if !parsed {
			lane.Detail = "no parseable verdict JSON line on stdout"
		} else if full.Verdict == LaneInvalid {
			lane.Detail = "block verdict carried no reasons (an empty veto is invalid, not a pass and not a block)"
		}
	}
	return lane
}

func specName(spec ReviewerSpec, def string) string {
	if spec.Name != "" {
		return spec.Name
	}
	return def
}

// reviewPacketOf assembles the record_review_decision packet from
// the aggregated lanes. Route choice is deterministic: the
// strongest blocking lane's declared route wins (ungraded blocks
// grade P0, as at finding assembly); when no blocking lane declares
// a route, the first route any valid lane declared is used. The
// synthesis marker is "at least one lane answered with a valid
// verdict line" — zero valid lanes leaves an empty packet, which
// the routing fail-closes as incomplete.
func reviewPacketOf(view PlannerReviewView, findings []core.ReviewFinding, reviewID string) core.ReviewPacket {
	rank := map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}
	route := ""
	bestRank := 99
	declaredHighest := ""
	udr := false
	valid := 0
	var firstRoute string
	for _, lane := range view.ReviewerLanes {
		if lane.Verdict == LaneInvalid {
			continue
		}
		valid++
		if lane.UserDecisionRequired {
			udr = true
		}
		if lane.RecommendedRoute != "" && firstRoute == "" {
			firstRoute = lane.RecommendedRoute
		}
		sev := strings.ToUpper(strings.TrimSpace(lane.Severity))
		if r, ok := rank[sev]; ok && (declaredHighest == "" || r < rank[declaredHighest]) {
			declaredHighest = sev
		}
		if lane.Verdict != LaneBlock {
			continue
		}
		eff := sev
		if eff == "" {
			eff = "P0"
		}
		if lane.RecommendedRoute != "" && rank[eff] < bestRank {
			bestRank = rank[eff]
			route = lane.RecommendedRoute
		}
	}
	if route == "" {
		route = firstRoute
	}
	count := len(findings)
	return core.ReviewPacket{
		ReviewID: reviewID, ArtifactRef: view.ArtifactID,
		RecommendedRoute: route, UserDecisionRequired: udr,
		Findings: findings, DeclaredHighest: declaredHighest,
		DeclaredFindingCount: &count, SynthesisMarker: valid > 0,
	}
}
