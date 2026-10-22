// P2c delegation-and-parallel state types: implement lanes (G-22),
// ralph delegation (G-10), parallel-lake plans (G-11), design
// primitive bindings (G-18) and the implement admission result
// (G-04). Like every other goal state, these live inside
// runs.metadata_json — the single store snapshot owned by the engine —
// never in package-owned JSON files (charter rule 7, frozen AC-016).
package orchestrator

// ---- Implement lanes (G-22) ----

// Lane names, mirroring implement/commands.py's mode vocabulary
// (_normalize_mode accepts exactly these three).
const (
	LaneSingle = "single"
	LaneTeam   = "team"
	LaneRalph  = "ralph"
)

// Lane sources: how a lane decision was reached.
const (
	LaneSourceExplicit = "explicit"
	LaneSourceScale    = "scale"
	LaneSourceLakes    = "lakes"
	LaneSourceDefault  = "default"
)

// LaneRequest is the routing input: an explicit lane ask plus the
// run's scale field ("" when unset).
type LaneRequest struct {
	Explicit string `json:"explicit,omitempty"`
	Scale    string `json:"scale,omitempty"`
}

// LaneDecision is the outcome of RouteLane.
type LaneDecision struct {
	Lane   string `json:"lane"`
	Source string `json:"source"` // explicit | scale | lakes | default
	Reason string `json:"reason"`
}

// ImplementState is the run's implement-lane record: the routed lane,
// the bound worktree ref (admission consults it) and the delegations.
type ImplementState struct {
	Lane        string       `json:"lane,omitempty"`
	LaneSource  string       `json:"lane_source,omitempty"`
	WorktreeRef string       `json:"worktree_ref,omitempty"`
	Delegations []Delegation `json:"delegations,omitempty"`
	// P3c: the REAL worktrees created from this run's parallel plan
	// (nil until `goal worktree create` runs) and the plan the
	// planner review last consumed.
	Worktrees     *WorktreeState       `json:"worktrees,omitempty"`
	PlannerReview *PlannerReviewRecord `json:"planner_review,omitempty"`
}

// activeDelegation returns the delegation currently out (status
// delegated), or nil.
func (s *ImplementState) activeDelegation() *Delegation {
	if s == nil {
		return nil
	}
	for i := range s.Delegations {
		if s.Delegations[i].Status == DelegationDelegated {
			return &s.Delegations[i]
		}
	}
	return nil
}

func (s *ImplementState) findDelegation(id string) *Delegation {
	if s == nil {
		return nil
	}
	for i := range s.Delegations {
		if s.Delegations[i].ID == id {
			return &s.Delegations[i]
		}
	}
	return nil
}

// ImplementView is the status-surface summary of the lane state.
type ImplementView struct {
	Lane               string `json:"lane,omitempty"`
	LaneSource         string `json:"lane_source,omitempty"`
	WorktreeRef        string `json:"worktree_ref,omitempty"`
	ActiveDelegationID string `json:"active_delegation_id,omitempty"`
	DelegationCount    int    `json:"delegation_count"`
}

func (s *ImplementState) view() *ImplementView {
	if s == nil {
		return nil
	}
	v := &ImplementView{
		Lane: s.Lane, LaneSource: s.LaneSource, WorktreeRef: s.WorktreeRef,
		DelegationCount: len(s.Delegations),
	}
	if d := s.activeDelegation(); d != nil {
		v.ActiveDelegationID = d.ID
	}
	if v.Lane == "" && v.WorktreeRef == "" && v.DelegationCount == 0 {
		return nil
	}
	return v
}

// ---- Team handoff (G-22, team lane) ----

// TeamHandoff is the implement-to-QA handoff of the team lane
// (team_surface.write_implement_to_qa_handoff payload, minimum set):
// who hands over, who receives, and which artifacts change hands.
// All three are required — a handoff missing any of them is refused.
type TeamHandoff struct {
	From         string   `json:"from"`
	To           string   `json:"to"`
	ArtifactRefs []string `json:"artifact_refs"`
	SummaryRef   string   `json:"summary_ref,omitempty"`
}

// ---- Harvest / receive (shared lane collection rule) ----

// ArtifactInput is one artifact pointer a lane result brings back.
// The pointer is registered in artifact_refs on receipt (AbsPath must
// be absolute — the store enforces AC-083).
type ArtifactInput struct {
	Kind        string `json:"kind"`
	AbsPath     string `json:"abs_path"`
	ContentHash string `json:"content_hash,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

// LaneResultInput is what a lane (or a returned delegation) hands
// back: a status, a summary, artifact pointers and — for the team
// lane — the handoff record.
type LaneResultInput struct {
	Status    string          `json:"status"` // done | failed
	Summary   string          `json:"summary,omitempty"`
	Artifacts []ArtifactInput `json:"artifacts,omitempty"`
	Handoff   *TeamHandoff    `json:"handoff,omitempty"`
}

// HarvestView is the verdict of the shared collection rule: a lane
// result counts as collected only when its artifact pointers landed
// in artifact_refs AND a stage_executed evidence event for the
// implement stage exists on the run.
type HarvestView struct {
	RunID         string   `json:"run_id"`
	Lane          string   `json:"lane"`
	Accepted      bool     `json:"accepted"`
	ReasonCode    string   `json:"reason_code,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
	Missing       []string `json:"missing,omitempty"`
	ArtifactIDs   []string `json:"artifact_ids,omitempty"`
	EvidenceFound bool     `json:"evidence_found"`
	Simulated     bool     `json:"simulated,omitempty"`
}

// ---- Ralph delegation (G-10) ----

// Delegation statuses.
const (
	DelegationDelegated = "delegated"
	DelegationReturned  = "returned"
	DelegationFailed    = "failed"
)

// DelegationPackage is the whole implement stage packed for the
// delegatee: requirement package pointers, the stage plan, the
// admission verdict it was packed under, the acceptance rule the
// result will be collected by, and the rollback requirements.
type DelegationPackage struct {
	FeatureSlug string           `json:"feature_slug"`
	PackageRoot string           `json:"package_root"`
	TruthPath   string           `json:"truth_path"`
	TestsPath   string           `json:"tests_path,omitempty"`
	StagePlan   []StagePlanEntry `json:"stage_plan"`
	Lane        string           `json:"lane"`
	WorktreeRef string           `json:"worktree_ref,omitempty"`
	Admission   AdmissionResult  `json:"admission"`
	Acceptance  []string         `json:"acceptance"`
	Rollback    []string         `json:"rollback"`
}

// Delegation is one implement-stage delegation record on the run.
type Delegation struct {
	ID            string            `json:"id"`
	Lane          string            `json:"lane"`
	Status        string            `json:"status"` // delegated | returned | failed
	Simulated     bool              `json:"simulated"`
	Package       DelegationPackage `json:"package"`
	ResultSummary string            `json:"result_summary,omitempty"`
	ReasonCodes   []string          `json:"reason_codes,omitempty"`
	CreatedAt     string            `json:"created_at"`
	ReturnedAt    string            `json:"returned_at,omitempty"`
}

// DelegationView is the CLI/engine surface of one delegation.
type DelegationView struct {
	RunID      string     `json:"run_id"`
	Delegation Delegation `json:"delegation"`
	Run        RunView    `json:"run"`
}

// ---- Parallel lakes (G-11) ----

// LakeInput is one lake's plan input: its slug and the surfaces
// (files/modules) it declares it will touch.
type LakeInput struct {
	Slug    string   `json:"slug"`
	Touches []string `json:"touches,omitempty"`
}

// MainStarPlan is the hub of the pool: satellites merge into it, never
// the reverse (build_parallel_plan's main_star record).
type MainStarPlan struct {
	Role        string `json:"role"`
	Worktree    string `json:"worktree"`
	MergePolicy string `json:"merge_policy"`
}

// SatellitePlan is one lake's workstation in the pool.
type SatellitePlan struct {
	Lake                string   `json:"lake"`
	Role                string   `json:"role"`
	Worktree            string   `json:"worktree"`
	MergeTarget         string   `json:"merge_target"`
	MayWriteTruthOnMain bool     `json:"may_write_truth_on_main"`
	Constraints         []string `json:"constraints"`
}

// TouchConflict is one surface declared by two or more lakes: those
// lakes may not silently run in parallel over it.
type TouchConflict struct {
	Surface string   `json:"surface"`
	Lakes   []string `json:"lakes"`
}

// ParallelPlan is the plan-only output of BuildParallelPlan. P2c
// never creates worktrees (real creation is P3), so PlannedOnly is
// always true and every surface says so.
type ParallelPlan struct {
	Schema              string          `json:"schema"`
	Mode                string          `json:"mode"` // single_worktree | main_star_satellite_pool
	Lakes               []string        `json:"lakes"`
	MainStar            MainStarPlan    `json:"main_star"`
	Satellites          []SatellitePlan `json:"satellites,omitempty"`
	OneLakeOneSatellite bool            `json:"one_lake_one_satellite"`
	Conflicts           []TouchConflict `json:"conflicts,omitempty"`
	RequiresDecision    bool            `json:"requires_decision"`
	PlannedOnly         bool            `json:"planned_only"`
	IntentSource        string          `json:"intent_source"` // explicit_flag | lake_count | single_default
}

// Parallel modes, verbatim from build_parallel_plan.
const (
	ParallelSingle = "single_worktree"
	ParallelPool   = "main_star_satellite_pool"
)

// ---- Design primitive binding (G-18) ----

// DesignBinding is the run's record that a design primitive set (the
// frozen visual rules: colours, spacing, components) is bound to this
// run. The binding lives in the run snapshot — it is a property of
// the run, and gate_results stay reserved for gate evaluations (P3
// write-back). P2c records bindings only; it generates no prototype
// files (generation is the host bridge's job, charter 3.4).
type DesignBinding struct {
	PrimitiveSet string `json:"primitive_set"`
	Version      string `json:"version,omitempty"`
	SourceRef    string `json:"source_ref,omitempty"`
	BoundBy      string `json:"bound_by,omitempty"`
	BoundAt      string `json:"bound_at"`
}

// DesignStageView is the design-stage surface: whether design is
// required for this run, whether the primitives are bound, and the
// bridge note for prototype generation.
type DesignStageView struct {
	RunID        string         `json:"run_id"`
	Required     bool           `json:"required"`
	StagePresent bool           `json:"stage_present"`
	Bound        bool           `json:"bound"`
	Binding      *DesignBinding `json:"binding,omitempty"`
	StageStatus  string         `json:"stage_status,omitempty"`
	ReasonCode   string         `json:"reason_code,omitempty"`
	BridgeNote   string         `json:"bridge_note"`
}

// DesignBridgeNote is stated on every design surface: prototype
// generation is delegated to the host agent, never done here.
const DesignBridgeNote = "prototype generation is delegated to the host agent via the design bridge (P3); P2c records primitive bindings only and generates no prototype files"

// ---- Implement admission (G-04) ----

// AdmissionItem is one admission check with its own verdict.
type AdmissionItem struct {
	Check      string `json:"check"`
	OK         bool   `json:"ok"`
	Applicable bool   `json:"applicable"`
	ReasonCode string `json:"reason_code,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// AdmissionResult mirrors goal_implement_admission's gate shape
// (schema goal-implement-admission.v1): one verdict, per-item results,
// and the first failing reason code as the headline.
type AdmissionResult struct {
	Schema           string          `json:"schema"`
	RunID            string          `json:"run_id"`
	FeatureSlug      string          `json:"feature_slug"`
	AllowImplement   bool            `json:"allow_implement"`
	Status           string          `json:"status"` // pass | blocked
	Blocked          bool            `json:"blocked"`
	ReasonCode       string          `json:"reason_code"`
	ReasonCodes      []string        `json:"reason_codes,omitempty"`
	Items            []AdmissionItem `json:"items"`
	ExpectedWorktree string          `json:"expected_worktree"`
	WorktreeRef      string          `json:"worktree_ref,omitempty"`
	Lane             string          `json:"lane,omitempty"`
	LaneSource       string          `json:"lane_source,omitempty"`
}
