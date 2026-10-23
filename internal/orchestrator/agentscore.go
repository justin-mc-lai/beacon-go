// Agent evaluation surface (R2d, parity audit M1): the storage
// and engine half of the agent-eval line whose pure judgement
// lives in internal/core/agenteval.go. Layout mirrors Python's
// agent_eval_root (.beacon/state/agent-eval/) with scorecards/
// and grades/ subdirectories — files, like Python, because an
// eval record belongs to the project, not to one machine's DB.
//
// The catalog is rebuilt from Go facts: the stage-skill names
// the loop axis actually routes to (core.LoopSkillForStage),
// resolved to their embedded host cores; the CLI's real top
// level commands; and Python's four boundary-negative entries
// verbatim.
package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/skills"
)

// AgentEvalVersion is agent_eval_kit.AGENT_EVAL_VERSION.
const AgentEvalVersion = "v1.6.1"

// EventAgentEvalGraded / EventAgentEvalScored trail the two
// write actions when a run context exists (grade --run-id).
const (
	EventAgentEvalGraded = "agent_eval_graded"
	EventAgentEvalScored = "agent_eval_scored"
)

func agentEvalRoot(projectRoot string) string {
	return filepath.Join(projectRoot, ".beacon", "state", "agent-eval")
}

func compactTS() string {
	return strings.ToLower(time.Now().UTC().Format("20060102T150405Z"))
}

// uniqueStamp is compactTS at nanosecond resolution: grade and
// scorecard files are named by it, and two records written in
// the same second must not overwrite each other.
func uniqueStamp() string {
	return strings.ToLower(time.Now().UTC().Format("20060102T150405.000000000Z"))
}

// ---- catalog ----

// AgentCatalogEntry is one capability-catalog row.
type AgentCatalogEntry struct {
	ID           string `json:"id"`
	Harness      string `json:"harness"` // pln | gen | eval | gov
	Surface      string `json:"surface"`
	Command      string `json:"command"`
	P0           bool   `json:"p0"`
	LiveRequired bool   `json:"live_required"`
	SkillID      string `json:"skill_id,omitempty"`
	SkillPresent *bool  `json:"skill_present,omitempty"`
}

// AgentCatalog is the materialized capability catalog view.
type AgentCatalog struct {
	SchemaVersion    string              `json:"schema_version"`
	Version          string              `json:"version"`
	EntryCount       int                 `json:"entry_count"`
	Entries          []AgentCatalogEntry `json:"entries"`
	MissingHarnesses []string            `json:"missing_harnesses,omitempty"`
	Authority        map[string]bool     `json:"authority"`
}

// hostCoreForSkill ports agent_eval_kit's _resolve_host_core_for_skill:
// legacy/merged skill ids map to their 1+6 host core.
var hostCoreForSkill = map[string]string{
	"beacon-eval-qa": "beacon-qa", "beacon-eval-qa-deep": "beacon-qa",
	"beacon-eval-real-project": "beacon-qa", "beacon-eval-release": "beacon-release",
	"beacon-gen-change": "beacon-truth", "beacon-gen-refreeze": "beacon-truth",
	"beacon-gen-truth": "beacon-truth", "beacon-gen-truth-init": "beacon-truth",
	"beacon-gen-design": "beacon-design", "beacon-gen-design-explore": "beacon-design",
	"beacon-gen-design-init": "beacon-design", "beacon-gen-design-polish": "beacon-design",
	"beacon-gen-design-system": "beacon-design", "beacon-gen-implement": "beacon-implement",
	"beacon-gen-prototype":  "beacon-design",
	"beacon-pln-brainstorm": "beacon-plan", "beacon-pln-friction": "beacon-plan",
	"beacon-pln-review": "beacon-plan", "beacon-pln-source": "beacon-plan",
	"beacon-goal": "beacon-goal",
}

// harnessShortFromSkillID ports _harness_short_from_skill_id.
func harnessShortFromSkillID(skillID string) string {
	if skillID == "beacon-loop-goal" {
		return "gov"
	}
	parts := strings.Split(skillID, "-")
	if len(parts) < 3 || parts[0] != "beacon" {
		return ""
	}
	switch parts[1] {
	case "pln", "gen", "eval", "gov":
		return parts[1]
	}
	return ""
}

func boolPtr(b bool) *bool { return &b }

// BuildAgentCatalog assembles the capability catalog from Go
// facts (no file writes; the scorecard builder persists what
// it needs).
func BuildAgentCatalog() AgentCatalog {
	embedded := map[string]bool{}
	for _, id := range skills.IDs() {
		embedded["beacon-"+id] = true
	}
	var entries []AgentCatalogEntry
	// Skill entries: every distinct skill name the loop axis
	// routes to, presence = its host core is embedded.
	seenSkill := map[string]bool{}
	for _, stage := range core.AllLoopStages() {
		name := core.LoopSkillForStage(stage)
		if name == "" || strings.HasPrefix(name, "CLI:") || seenSkill[name] {
			continue
		}
		seenSkill[name] = true
		coreName := hostCoreForSkill[name]
		if coreName == "" {
			coreName = name
		}
		present := embedded[coreName]
		harness := harnessShortFromSkillID(name)
		if harness == "" {
			harness = "eval"
		}
		entries = append(entries, AgentCatalogEntry{
			ID: "skill." + name, Harness: harness, Surface: "host-visible-skill",
			Command: name, P0: true, LiveRequired: true,
			SkillID: name, SkillPresent: boolPtr(present),
		})
	}
	// CLI entries: the kernel's real top-level faces, mapped to
	// the harness that drives them (the Python catalog's public
	// CLI rows name Python commands; these name Go's).
	for _, row := range []struct{ id, surface, command, harness string }{
		{"goal.run", "goal", "beacon goal", "pln"},
		{"truth.package", "truth", "beacon truth", "pln"},
		{"implement.lanes", "implement", "beacon implement", "gen"},
		{"qa.evaluate", "qa", "beacon qa evaluate", "eval"},
		{"qa.autotest", "qa", "beacon qa autotest run", "eval"},
		{"release.check", "release", "beacon release check", "eval"},
		{"gate.evaluate", "gate", "beacon gate evaluate", "eval"},
		{"skill.surface", "skills", "beacon skills", "eval"},
		{"doctor.rebuild", "doctor", "beacon doctor rebuild", "gov"},
		{"hooks.run", "hooks", "beacon hooks run", "gov"},
		{"archive.lineage", "archive", "beacon archive lineage", "gov"},
		{"eval.agent", "eval", "beacon qa agent-eval", "eval"},
	} {
		entries = append(entries, AgentCatalogEntry{
			ID: row.id, Harness: row.harness, Surface: row.surface,
			Command: row.command, P0: true,
		})
	}
	// Boundary-negative entries, verbatim from
	// _boundary_negative_catalog_entries.
	for _, row := range []struct{ id, harness, command string }{
		{"boundary.planner.truth-write", "pln", "beacon-pln-review"},
		{"boundary.generator.self-certification", "gen", "beacon-gen-implement"},
		{"boundary.evaluator.implementation-repair", "eval", "beacon-eval-qa"},
		{"boundary.governor.release-verdict", "gov", "beacon-gov-status"},
	} {
		entries = append(entries, AgentCatalogEntry{
			ID: row.id, Harness: row.harness, Surface: "boundary-negative",
			Command: row.command, P0: true,
		})
	}
	harnesses := map[string]bool{}
	for _, e := range entries {
		if e.Harness != "" {
			harnesses[e.Harness] = true
		}
	}
	var missing []string
	for _, h := range []string{"pln", "gen", "eval", "gov"} {
		if !harnesses[h] {
			missing = append(missing, h)
		}
	}
	sort.Strings(missing)
	return AgentCatalog{
		SchemaVersion: "1.0", Version: AgentEvalVersion,
		EntryCount: len(entries), Entries: entries,
		MissingHarnesses: missing, Authority: core.AgentEvalAuthority(),
	}
}

// CatalogCoverage counts p0 entries and how many carry a
// present skill surface (non-skill entries count as covered,
// matching the source's .get("skill_present", True)).
func (c AgentCatalog) CatalogCoverage() (total, covered int) {
	for _, e := range c.Entries {
		if !e.P0 {
			continue
		}
		total++
		if e.SkillPresent == nil || *e.SkillPresent {
			covered++
		}
	}
	return total, covered
}

// ---- transcript grading (persisted) ----

// AgentGradeRecord is one graded transcript, persisted under
// grades/. Source is "transcript" (a file/stdin the caller
// supplied) or "run" (assembled from a run's event trail).
type AgentGradeRecord struct {
	SchemaVersion string               `json:"schema_version"`
	Agent         string               `json:"agent"`
	GradedAt      string               `json:"graded_at"`
	Source        string               `json:"source"`
	RunID         string               `json:"run_id,omitempty"`
	ExpectedRoute string               `json:"expected_route_surface"`
	Grade         core.TranscriptGrade `json:"grade"`
	Path          string               `json:"path,omitempty"`
}

func writeJSONFile(abs string, v any) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(abs, append(raw, '\n'), 0o644)
}

// GradeAgentTranscript grades one transcript and persists the
// record. agent names whose behavior this is (an executor
// profile name, a skill id, a free label).
func GradeAgentTranscript(projectRoot, agent, transcript, expectedRoute, source, runID string) (AgentGradeRecord, error) {
	if strings.TrimSpace(agent) == "" {
		agent = "unknown-agent"
	}
	rec := AgentGradeRecord{
		SchemaVersion: "1.0", Agent: agent, GradedAt: nowISO(),
		Source: source, RunID: runID, ExpectedRoute: expectedRoute,
		Grade: core.GradeTranscript(transcript, expectedRoute),
	}
	name := fmt.Sprintf("%s-%s.json", uniqueStamp(), agent)
	abs := filepath.Join(agentEvalRoot(projectRoot), "grades", name)
	if err := writeJSONFile(abs, rec); err != nil {
		return AgentGradeRecord{}, err
	}
	rec.Path = filepath.ToSlash(filepath.Join(".beacon", "state", "agent-eval", "grades", name))
	return rec, nil
}

// GradeRunTranscript assembles a transcript from one run's event
// trail (event types + payloads, in order) and grades it. The
// expected route defaults to the skill the run's current loop
// stage routes to. Boundary phrases anywhere in the trail — an
// executor writing a "release verdict" into its output, say —
// fail the grade exactly like a supplied transcript.
func (e *Engine) GradeRunTranscript(runID, agent, expectedRoute string) (AgentGradeRecord, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return AgentGradeRecord{}, err
	}
	fctx, err := e.featureContext(run.FeatureID)
	if err != nil {
		return AgentGradeRecord{}, err
	}
	events, err := e.store.ListEvents(run.FeatureID)
	if err != nil {
		return AgentGradeRecord{}, err
	}
	var lines []string
	for _, ev := range events {
		if ev.RunID.Valid && ev.RunID.String != runID {
			continue
		}
		if !ev.RunID.Valid {
			continue
		}
		lines = append(lines, ev.EventType+" "+string(ev.Payload))
	}
	if agent == "" {
		agent = "run:" + runID
	}
	if expectedRoute == "" && m != nil {
		expectedRoute = core.LoopSkillForStage(core.LoopStage(m.LoopStage))
	}
	rec, err := GradeAgentTranscript(fctx.Project.RootPath, agent,
		strings.Join(lines, "\n"), expectedRoute, "run", runID)
	if err != nil {
		return AgentGradeRecord{}, err
	}
	_ = e.appendFeatureEvent(run.FeatureID, EventAgentEvalGraded, map[string]any{
		"run_id": runID, "agent": agent, "passed": rec.Grade.Passed,
		"reason_code": rec.Grade.ReasonCode, "path": rec.Path,
	})
	return rec, nil
}

// ListAgentGrades returns one agent's grade records, oldest
// first (all agents when agent == "").
func ListAgentGrades(projectRoot, agent string) []AgentGradeRecord {
	dir := filepath.Join(agentEvalRoot(projectRoot), "grades")
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	sort.Strings(files)
	var out []AgentGradeRecord
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var rec AgentGradeRecord
		if json.Unmarshal(raw, &rec) != nil {
			continue
		}
		if agent != "" && rec.Agent != agent {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// ---- scorecard ----

// AgentScorecard is build_agent_eval_scorecard's payload,
// persisted under scorecards/. It is a DIFFERENT object from
// orchestrator.Scorecard (the skill score): this one grades an
// agent's behavior; that one grades a skill package.
type AgentScorecard struct {
	SchemaVersion  string             `json:"schema_version"`
	ScorecardID    string             `json:"scorecard_id"`
	Agent          string             `json:"agent"`
	Version        string             `json:"version"`
	ExecutionMode  string             `json:"execution_mode"`
	Degraded       bool               `json:"degraded"`
	Authority      map[string]bool    `json:"authority"`
	Dimensions     map[string]float64 `json:"dimensions"`
	Weights        map[string]float64 `json:"composite_weights"`
	CompositeScore float64            `json:"composite_score"`
	Status         string             `json:"status"` // pass | blocked
	CaseCount      int                `json:"case_count"`
	FailureCount   int                `json:"failure_count"`
	CatalogP0Total int                `json:"catalog_p0_total"`
	CatalogP0Cover int                `json:"catalog_p0_covered"`
	GeneratedAt    string             `json:"generated_at"`
	Path           string             `json:"path,omitempty"`
}

// BuildAgentScorecard computes and persists one agent's
// scorecard from its grade history + the live catalog. The
// runner is "replay": grades are recorded evidence replayed
// through the grader, exactly the source's default mode.
func BuildAgentScorecard(projectRoot, agent string) (AgentScorecard, error) {
	if strings.TrimSpace(agent) == "" {
		agent = "unknown-agent"
	}
	catalog := BuildAgentCatalog()
	total, covered := catalog.CatalogCoverage()
	grades := ListAgentGrades(projectRoot, agent)
	failures := 0
	for _, g := range grades {
		if !g.Grade.Passed {
			failures++
		}
	}
	dims := core.ComputeAgentDimensions(core.AgentDimensionInputs{
		CatalogP0Total: total, CatalogP0Covered: covered,
		FailureCount: failures, CaseCount: len(grades),
		Runner: "replay",
	})
	composite := core.AgentComposite(dims, "replay")
	status := "pass"
	if composite < core.AgentEvalCompositeThreshold {
		status = "blocked"
	}
	card := AgentScorecard{
		SchemaVersion: "1.0",
		ScorecardID:   "scorecard-" + uniqueStamp(),
		Agent:         agent, Version: AgentEvalVersion,
		ExecutionMode: "replay",
		// Degraded when nothing was ever measured: Python marks
		// a card degraded when its evidence is not live-admitted;
		// the Go replay card's equivalent honesty flag is "no
		// grades on file".
		Degraded:       len(grades) == 0,
		Authority:      core.AgentEvalAuthority(),
		Dimensions:     dims,
		Weights:        core.AgentEvalWeights,
		CompositeScore: composite,
		Status:         status,
		CaseCount:      len(grades),
		FailureCount:   failures,
		CatalogP0Total: total, CatalogP0Cover: covered,
		GeneratedAt: nowISO(),
	}
	name := card.ScorecardID + "-" + agent + ".json"
	abs := filepath.Join(agentEvalRoot(projectRoot), "scorecards", name)
	if err := writeJSONFile(abs, card); err != nil {
		return AgentScorecard{}, err
	}
	card.Path = filepath.ToSlash(filepath.Join(".beacon", "state", "agent-eval", "scorecards", name))
	return card, nil
}

// ListAgentScorecards returns persisted scorecards, oldest
// first (one agent, or all when agent == "").
func ListAgentScorecards(projectRoot, agent string) []AgentScorecard {
	dir := filepath.Join(agentEvalRoot(projectRoot), "scorecards")
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	sort.Strings(files)
	var out []AgentScorecard
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var card AgentScorecard
		if json.Unmarshal(raw, &card) != nil {
			continue
		}
		if agent != "" && card.Agent != agent {
			continue
		}
		out = append(out, card)
	}
	return out
}

// LatestAgentScorecard returns the newest card for one agent.
func LatestAgentScorecard(projectRoot, agent string) (AgentScorecard, bool) {
	cards := ListAgentScorecards(projectRoot, agent)
	if len(cards) == 0 {
		return AgentScorecard{}, false
	}
	return cards[len(cards)-1], true
}

// AgentAdmission is the profile-selection disposition for one
// agent/profile name, derived from its latest scorecard and
// latest grade (see core.AgentAdmissionFromRecord for the
// disposition rules and their Python basis).
func AgentAdmission(projectRoot, agent string) core.AgentAdmissionVerdict {
	card, ok := LatestAgentScorecard(projectRoot, agent)
	if !ok {
		return core.AgentAdmissionFromRecord(false, 0, false)
	}
	boundaryFailed := false
	if grades := ListAgentGrades(projectRoot, agent); len(grades) > 0 {
		latest := grades[len(grades)-1]
		boundaryFailed = latest.Grade.ReasonCode == core.ReasonBoundaryForbiddenPhrase ||
			len(latest.Grade.ForbiddenHits) > 0
	}
	return core.AgentAdmissionFromRecord(true, card.CompositeScore, boundaryFailed)
}

// CheckProfileAdmission is the executor-selection hook: it
// refuses (typed error) when the profile's latest agent-eval
// record blocks selection, and lets warn/admitted through. It
// runs at profile resolution time — the one moment the kernel
// knows which agent is about to do the work.
func (e *Engine) CheckProfileAdmission(projectRoot, profileName, op string) error {
	v := AgentAdmission(projectRoot, profileName)
	if v.State == "blocked" {
		return core.NewError(v.ReasonCode, op,
			"executor profile "+profileName+" is blocked by its agent-eval record: "+v.Detail)
	}
	return nil
}
