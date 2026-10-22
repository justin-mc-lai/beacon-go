// P2d orchestration-extension state types: program plans (G-12),
// hotfix plans (G-13), multi-version plans (G-14), goal memory
// indexes (G-15) and main axes (G-16). Like every other goal state,
// these live inside runs.metadata_json — the single store snapshot
// owned by the engine — never in package-owned JSON files or extra
// tables (charter rule 7, frozen AC-016; the frozen nine-table schema
// is untouched by P2d).
package orchestrator

// ---- Program (G-12) ----

// ProgramLakeInput is one lake of a program plan request: its slug
// (a feature slug in the host run's project version), its one-line
// goal, the lakes it depends on, and the surfaces it declares it
// will touch (kept for parity with the parallel-plan input shape).
type ProgramLakeInput struct {
	Slug      string   `json:"slug"`
	Goal      string   `json:"goal,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
	Touches   []string `json:"touches,omitempty"`
}

// LakeAdmissionItem is one per-lake admission check.
type LakeAdmissionItem struct {
	Check      string `json:"check"`
	OK         bool   `json:"ok"`
	ReasonCode string `json:"reason_code,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// LakeAdmission is the recorded admission verdict of one lake
// (program_admission.py's per-lake evaluation, persisted on the plan
// so route can tell "admitted" from "never judged" without a second
// state store).
type LakeAdmission struct {
	Lake        string              `json:"lake"`
	Admitted    bool                `json:"admitted"`
	ReasonCodes []string            `json:"reason_codes,omitempty"`
	Items       []LakeAdmissionItem `json:"items"`
	EvaluatedAt string              `json:"evaluated_at"`
}

// ProgramLake is one lake of a recorded program plan.
type ProgramLake struct {
	Slug      string         `json:"slug"`
	Goal      string         `json:"goal,omitempty"`
	DependsOn []string       `json:"depends_on,omitempty"`
	Touches   []string       `json:"touches,omitempty"`
	Admission *LakeAdmission `json:"admission,omitempty"`
}

// ProgramChange is one recorded change intent against a (possibly
// frozen) program plan (program_change_intent.py's append-only log,
// folded into the plan snapshot + the lifecycle event trail).
type ProgramChange struct {
	At          string   `json:"at"`
	Note        string   `json:"note"`
	LakesBefore []string `json:"lakes_before,omitempty"`
	LakesAfter  []string `json:"lakes_after,omitempty"`
}

// ProgramState is the program plan bound to a host goal run. The
// binding is the run itself: lakes are feature slugs in the host
// run's project version, lake progress is always derived live from
// each lake feature's own runs/lifecycle (ProgramRoute), never
// cached here — this snapshot holds only the plan, its freeze state
// and its admission records.
type ProgramState struct {
	Slug      string          `json:"slug"`
	Goal      string          `json:"goal,omitempty"`
	Lakes     []ProgramLake   `json:"lakes"`
	Frozen    bool            `json:"frozen"`
	FrozenAt  string          `json:"frozen_at,omitempty"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	Changes   []ProgramChange `json:"changes,omitempty"`
}

func (p *ProgramState) lake(slug string) *ProgramLake {
	if p == nil {
		return nil
	}
	for i := range p.Lakes {
		if p.Lakes[i].Slug == slug {
			return &p.Lakes[i]
		}
	}
	return nil
}

func (p *ProgramState) lakeSlugs() []string {
	out := []string{}
	if p == nil {
		return out
	}
	for _, l := range p.Lakes {
		out = append(out, l.Slug)
	}
	return out
}

// lakeAdmitted reports whether the lake carries a passing admission
// record on the plan.
func (p *ProgramState) lakeAdmitted(slug string) bool {
	l := p.lake(slug)
	return l != nil && l.Admission != nil && l.Admission.Admitted
}

// ProgramAdmissionView is the surface of one admission evaluation.
type ProgramAdmissionView struct {
	RunID       string          `json:"run_id"`
	ProgramSlug string          `json:"program_slug"`
	AllAdmitted bool            `json:"all_admitted"`
	Lakes       []LakeAdmission `json:"lakes"`
}

// LakeRoute is one lake's live row in a program route view.
type LakeRoute struct {
	Lake       string   `json:"lake"`
	Goal       string   `json:"goal,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"`
	Status     string   `json:"status"` // completed | admitted | ready | waiting | blocked
	ReasonCode string   `json:"reason_code,omitempty"`
	FeatureID  string   `json:"feature_id,omitempty"`
	GoalRunID  string   `json:"goal_run_id,omitempty"`
	DriverHint string   `json:"driver_hint,omitempty"`
}

// ProgramRouteView is the read-only orchestration view (G-12): the
// topological next batch plus every lake's live status. Nothing in
// it is cached state — it is recomputed from the store on each call
// (program_orchestrator.py's build_program_orchestrator_state,
// re-based from files to the store).
type ProgramRouteView struct {
	RunID       string      `json:"run_id"`
	ProgramSlug string      `json:"program_slug"`
	Frozen      bool        `json:"frozen"`
	Lakes       []LakeRoute `json:"lakes"`
	NextBatch   []string    `json:"next_batch"`
	AllComplete bool        `json:"all_complete"`
}

// Lake route statuses.
const (
	LakeCompleted = "completed"
	LakeAdmitted  = "admitted"
	LakeReady     = "ready"
	LakeWaiting   = "waiting"
	LakeBlocked   = "blocked"
)

// ---- Hotfix (G-13) ----

// HotfixPlanInput is the build request for a hotfix plan.
type HotfixPlanInput struct {
	Description                string   // the incident / fault description
	FromVersion                string   // baseline: the version to fix from
	TargetVersion              string   // forward-port target the fix must also land on
	FixPoints                  []string // the concrete points the fix touches
	RollbackPlan               string   // how the fix is rolled back (required)
	RollbackDeclaredExecutable bool     // caller declares the rollback executable
}

// HotfixPlan is one recorded hotfix plan (prod_ops_v168's plan dict,
// folded onto the run): baseline, forward target, fix points and the
// rollback. RequiresHumanRelease is structurally always true — the
// fast lane never skips the human release confirmation, and the
// normal Engine.Complete path is unchanged by a hotfix plan.
type HotfixPlan struct {
	Schema                     string   `json:"schema"`
	RunID                      string   `json:"run_id"`
	Description                string   `json:"description"`
	FromVersion                string   `json:"from_version"`
	BaselineRef                string   `json:"baseline_ref"`
	TargetVersion              string   `json:"target_version"`
	ForwardPortTarget          string   `json:"forward_port_target"`
	FixPoints                  []string `json:"fix_points"`
	RollbackPlan               string   `json:"rollback_plan"`
	RollbackDeclaredExecutable bool     `json:"rollback_declared_executable"`
	ReleaseProfile             string   `json:"release_profile"`
	RequiredGates              []string `json:"required_gates"`
	RequiresHumanRelease       bool     `json:"requires_human_release"`
	CreatedAt                  string   `json:"created_at"`
}

// HotfixRequiredGates mirrors prod_ops_v168.HOTFIX_REQUIRED_GATES.
var HotfixRequiredGates = []string{
	"evidence_manifest", "hotfix_baseline", "forward_port_plan",
	"rollback_notes", "minimal_qa", "human_release",
}

// HotfixValidation is the release-readiness verdict of a recorded
// hotfix plan (validate_hotfix_for_release + the run's qa gate).
type HotfixValidation struct {
	RunID                string   `json:"run_id"`
	OK                   bool     `json:"ok"`
	Blocked              bool     `json:"blocked"`
	ReasonCodes          []string `json:"reason_codes,omitempty"`
	ReleaseProfile       string   `json:"release_profile"`
	RequiresHumanRelease bool     `json:"requires_human_release"`
	QAGateVerdict        string   `json:"qa_gate_verdict,omitempty"`
}

// ---- Multi-version (G-14) ----

// VersionSegmentInput is one segment of a multi-version request.
type VersionSegmentInput struct {
	Version   string   `json:"version"`
	Items     []string `json:"items"` // lake/run slugs this segment carries
	DependsOn []string `json:"depends_on,omitempty"`
}

// MultiVersionInput is the PlanMultiVersion request. Completed and
// Dropped are keyed by version: the items of that segment already
// done, and the items explicitly dropped (an unfinished item that is
// neither carried into the next segment nor dropped is a problem —
// work must never silently vanish between versions).
type MultiVersionInput struct {
	Segments  []VersionSegmentInput
	Completed map[string][]string
	Dropped   map[string][]string
}

// VersionSegment is one recorded segment of a multi-version plan.
type VersionSegment struct {
	Version   string   `json:"version"`
	Items     []string `json:"items"`
	Completed []string `json:"completed,omitempty"`
	Dropped   []string `json:"dropped,omitempty"`
	CarriedIn []string `json:"carried_in,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// MVProblem is one multi-version validation problem.
type MVProblem struct {
	ReasonCode string `json:"reason_code"`
	Detail     string `json:"detail"`
}

// MultiVersionPlan is the recorded segmentation of one goal across
// versions (plan_multi_version's tracks, sequential-segment form).
type MultiVersionPlan struct {
	Schema    string           `json:"schema"`
	RunID     string           `json:"run_id"`
	Segments  []VersionSegment `json:"segments"`
	Valid     bool             `json:"valid"`
	Problems  []MVProblem      `json:"problems,omitempty"`
	CreatedAt string           `json:"created_at"`
}

// ---- Goal memory (G-15) ----

// MemoryEntry is one indexed text unit of a run: a lifecycle event,
// an artifact pointer or a checkpoint, reduced to searchable text.
// The index is a derived cache — it can be rebuilt from the
// authoritative tables at any time and is never evidence.
type MemoryEntry struct {
	SourceKind string `json:"source_kind"` // event | artifact | checkpoint
	SourceID   string `json:"source_id"`
	RunID      string `json:"run_id"`
	Label      string `json:"label"` // event type / artifact kind / "checkpoint"
	Text       string `json:"text"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// MemoryIndex is the run's inverted index over its own trail,
// stored in the run snapshot. Storage argument (p2d report): the
// frozen nine-table schema has no memory table and adding one — or
// an FTS5 virtual table, which modernc.org/sqlite does support —
// would change the frozen schema; a sidecar DB file (the v1 shape)
// would violate charter rule 7. The inverted index therefore lives
// in runs.metadata_json like all other goal state, and search is a
// posting-list intersection over Terms.
type MemoryIndex struct {
	IndexedAt string           `json:"indexed_at"`
	Entries   []MemoryEntry    `json:"entries"`
	Terms     map[string][]int `json:"terms"`
}

// MemoryReindexView is the reindex surface (goal_memory_reindex's
// payload shape, store-based).
type MemoryReindexView struct {
	RunID         string `json:"run_id"`
	Status        string `json:"status"`
	Indexed       int    `json:"indexed"`
	Authority     string `json:"authority"` // always support_advisory
	ReferenceOnly bool   `json:"reference_only"`
	Note          string `json:"note"`
}

// MemoryHit is one search hit. ReferenceOnly is always true: a hit
// is something to look at, never proof of completion.
type MemoryHit struct {
	SourceKind    string `json:"source_kind"`
	SourceID      string `json:"source_id"`
	RunID         string `json:"run_id"`
	Label         string `json:"label"`
	Snippet       string `json:"snippet"`
	CreatedAt     string `json:"created_at,omitempty"`
	ReferenceOnly bool   `json:"reference_only"`
}

// MemorySearchView is the search surface (goal_memory_search's
// payload shape): support_advisory, can_complete structurally false.
type MemorySearchView struct {
	RunID         string      `json:"run_id"`
	Status        string      `json:"status"` // ok | empty
	Query         string      `json:"query"`
	Hits          []MemoryHit `json:"hits"`
	Authority     string      `json:"authority"` // always support_advisory
	ReferenceOnly bool        `json:"reference_only"`
	CanComplete   bool        `json:"can_complete"` // always false
	Note          string      `json:"note,omitempty"`
}

// ---- Main axis (G-16) ----

// AxisStage is one stage of a main axis: the stage name plus the
// criterion that says it is done. Criteria are statements of the
// existing gate surfaces (exit gates / admission / complete), not a
// second rule set — the axis describes the spine, the gates judge it.
type AxisStage struct {
	Stage     string `json:"stage"`
	Criterion string `json:"criterion"`
}

// MainAxis is a run's (or its program's) spine description:
// objective in one line, the stage order with done-criteria, and —
// for program hosts — the lakes in dependency (topological) order.
// Derived from the run's stage plan and program plan at generation
// time; ValidateMainAxis re-derives and reports any drift.
type MainAxis struct {
	Schema      string      `json:"schema"`
	RunID       string      `json:"run_id"`
	ProgramSlug string      `json:"program_slug,omitempty"`
	Objective   string      `json:"objective"`
	Stages      []AxisStage `json:"stages"`
	Lakes       []string    `json:"lakes,omitempty"`
	GeneratedAt string      `json:"generated_at"`
}

// AxisDrift is one way the stored axis no longer matches reality.
type AxisDrift struct {
	Kind   string `json:"kind"` // stage_mismatch | lake_missing | lake_extra | order_conflict | ref_missing
	Detail string `json:"detail"`
}

// AxisValidation is the validate surface (validate_main_axis,
// re-based to the store): ok only when there is no drift.
type AxisValidation struct {
	RunID       string      `json:"run_id"`
	OK          bool        `json:"ok"`
	ReasonCode  string      `json:"reason_code,omitempty"`
	Drifts      []AxisDrift `json:"drifts,omitempty"`
	ProgramSlug string      `json:"program_slug,omitempty"`
}
