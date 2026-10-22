// Topology S3 (AC-171/172/173): the GitScopeGraph -> BranchContext
// machine.
//
// AC-171: a v1-shaped GitScopeGraph (schema git-scope-graph.v1, as
// produced by v1.6.7's beacon.workspace.topology-resolve — real
// fixture: docs/beacon/archive/v1.6.7/.machine/schemas/
// git-scope-graph-v167.fixture.json) loads into the kernel and
// projects onto BranchContext.v1 with at least the four frozen
// fields topology_kind / scopes[] / topology_gate_status /
// topology_gate_ref. A graph that cannot be loaded or is missing
// key fields fails closed with gitscope_branch_context_parity_failed.
// The v1 ref was a state-file path (.beacon/state/git-scope-graph/
// ...); the Go kernel keeps state only in the store (charter rule
// 7), so its ref is content-addressed instead: bc_<sha256 prefix of
// the normalised graph> — the same graph always yields the same
// ref, any graph change yields a new one.
//
// AC-172: the topology gate judges every topology — single_repo,
// monorepo, multi_git_workspace. A WRITABLE scope is blocked when
// its working tree is dirty under the adopted caliber: primary
// scopes use the S2 dirty policy (requirement-package paths and
// protected paths violate; ordinary code may be dirty before
// implement), while satellite and service scopes — delivery
// scopes — are judged by raw dirtiness of their territory, exactly
// as v1.6.7 judged them. Any blocked scope — in particular any
// satellite of a multi-git workspace — blocks the aggregate gate.
// Scope codes are
// v1.6.7-verbatim where v1 has them (satellite_worktree_dirty,
// monorepo_service_dirty:<key>, multi_git_primary_missing); the
// primary scope's dirty violation reuses the frozen R11
// unscoped_dirty_state.
//
// AC-173 consumption lives in admission.go (item 8), bundle.go
// (branch-governance's topology_gate sub-result) and CreateRun
// (the run fact layer). The projection itself is recorded on an
// anchor feature's trail (branch_context_projected event — the
// store is the only state; no schema change) and resolved
// project-wide from there.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
)

// Topology kinds (frozen UD-GSG-002 spellings).
const (
	TopologySingleRepo = "single_repo"
	TopologyMonorepo   = "monorepo"
	TopologyMultiGit   = "multi_git_workspace"
)

// Schemas and event types of this slice.
const (
	GitScopeGraphSchema = "git-scope-graph.v1"
	BranchContextSchema = "BranchContext.v1"

	EventBranchContextProjected = "branch_context_projected"
	EventBranchContextConfirmed = "branch_context_confirmed"
)

// Topology gate / fact statuses.
const (
	TopologyGatePass      = "pass"
	TopologyGateBlocked   = "blocked"
	TopologyGateMissing   = "missing"
	TopologyNotApplicable = "not_applicable"
	TopologyUnevaluated   = "unevaluated"
)

// Projection sources: a graph file the human explicitly supplied
// (confirmed by the act of projecting it) vs a generated draft
// (detect — needs an explicit confirm before admission consumes it).
const (
	TopologySourceFile  = "file"
	TopologySourceDraft = "generated_draft"
)

// GitScope is one scope of a GitScopeGraph, in the v1.6.7 emitted
// shape (git_scope_graph.py::_scope_from_binding): scope_id,
// git_root, role, branch_resolution_mode, target_branch,
// worktree_path, admission_artifact_ref, dirty_policy, plus the
// binding-level fields the v1 builder consumed (writable — the
// monorepo writable-service flag; path — the in-repo pathspec v1
// keyed by service_key). Writable is a pointer so "absent" is
// distinguishable from an explicit false.
type GitScope struct {
	ScopeID              string `json:"scope_id"`
	GitRoot              string `json:"git_root"`
	Role                 string `json:"role"` // primary | satellite | service
	WorktreePath         string `json:"worktree_path,omitempty"`
	Path                 string `json:"path,omitempty"`
	Writable             *bool  `json:"writable,omitempty"`
	BranchResolutionMode string `json:"branch_resolution_mode,omitempty"`
	TargetBranch         string `json:"target_branch,omitempty"`
	AdmissionArtifactRef string `json:"admission_artifact_ref,omitempty"`
	DirtyPolicy          string `json:"dirty_policy,omitempty"`
}

// WritableScope resolves the scope's writability: the explicit
// flag wins; otherwise DERIVED default — satellites and services
// are the writable work scopes, and a primary scope is writable
// exactly when it is the only scope there is (single_repo).
func (s GitScope) WritableScope(topology string) bool {
	if s.Writable != nil {
		return *s.Writable
	}
	if s.Role == "primary" {
		return topology == TopologySingleRepo
	}
	return true
}

// GitScopeGraph is the v1-shaped input document (AC-GSG-001:
// schema git-scope-graph.v1 with scopes[] and topology_gate_status).
type GitScopeGraph struct {
	SchemaVersion      string     `json:"schema_version"`
	ProjectTopology    string     `json:"project_topology"`
	Scopes             []GitScope `json:"scopes"`
	TopologyGateStatus string     `json:"topology_gate_status,omitempty"`
	Feature            string     `json:"feature,omitempty"`
	FeatureSlug        string     `json:"feature_slug,omitempty"`
	Version            string     `json:"version,omitempty"`
	ProducedBy         string     `json:"produced_by,omitempty"`
}

// LoadGitScopeGraph parses and validates a GitScopeGraph document.
// Every structural defect is a parity failure (AC-171): the v2 side
// must not half-load a graph it cannot project faithfully.
func LoadGitScopeGraph(data []byte) (GitScopeGraph, error) {
	var g GitScopeGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return GitScopeGraph{}, core.NewError(core.ReasonGitScopeParityFailed, "topology.LoadGitScopeGraph",
			"graph is not valid JSON: "+err.Error())
	}
	if err := ValidateGitScopeGraph(g); err != nil {
		return GitScopeGraph{}, err
	}
	return g, nil
}

// ValidateGitScopeGraph is the structural parity check shared by
// the loader and the projection entry (AC-171 + AC-GSG-002/004
// shape rules).
func ValidateGitScopeGraph(g GitScopeGraph) error {
	fail := func(detail string) error {
		return core.NewError(core.ReasonGitScopeParityFailed, "topology.ValidateGitScopeGraph", detail)
	}
	if g.SchemaVersion != GitScopeGraphSchema {
		return fail("schema_version " + orMissing(g.SchemaVersion) + " != " + GitScopeGraphSchema)
	}
	switch g.ProjectTopology {
	case TopologySingleRepo, TopologyMonorepo, TopologyMultiGit:
	default:
		return fail("project_topology " + orMissing(g.ProjectTopology) + " is not one of single_repo/monorepo/multi_git_workspace")
	}
	if len(g.Scopes) == 0 {
		return fail("scopes[] is empty; a GitScopeGraph must declare at least one scope")
	}
	seen := map[string]bool{}
	var primaries, satellites, services int
	for _, s := range g.Scopes {
		if s.ScopeID == "" {
			return fail("a scope is missing scope_id")
		}
		if seen[s.ScopeID] {
			return fail("duplicate scope_id " + s.ScopeID)
		}
		seen[s.ScopeID] = true
		if s.GitRoot == "" {
			return fail("scope " + s.ScopeID + " is missing git_root")
		}
		switch s.Role {
		case "primary":
			primaries++
		case "satellite":
			satellites++
		case "service":
			services++
		default:
			return fail("scope " + s.ScopeID + " has role " + orMissing(s.Role) + " (want primary|satellite|service)")
		}
	}
	switch g.ProjectTopology {
	case TopologySingleRepo:
		// AC-GSG-002: single_repo means exactly one primary scope.
		if len(g.Scopes) != 1 || primaries != 1 {
			return fail("single_repo must declare exactly one scope with role=primary")
		}
	case TopologyMonorepo:
		if services == 0 {
			return fail("monorepo must declare at least one service scope")
		}
	case TopologyMultiGit:
		// AC-GSG-004: a multi-git workspace contains satellite scopes.
		if satellites == 0 {
			return fail("multi_git_workspace must declare at least one satellite scope")
		}
	}
	return nil
}

func orMissing(s string) string {
	if s == "" {
		return "(missing)"
	}
	return s
}

// BranchScope is one projected scope of a BranchContext.
type BranchScope struct {
	ScopeID      string `json:"scope_id"`
	GitRoot      string `json:"git_root"`
	WorktreePath string `json:"worktree_path"`
	Path         string `json:"path,omitempty"`
	Role         string `json:"role"`
	Writable     bool   `json:"writable"`
	DirtyPolicy  string `json:"dirty_policy"`
}

// BranchContext is the BranchContext.v1 projection (AC-171): the
// four frozen fields plus the graph hash, the projection source
// and its confirmation state.
type BranchContext struct {
	Schema             string        `json:"schema"`
	TopologyKind       string        `json:"topology_kind"`
	Scopes             []BranchScope `json:"scopes"`
	TopologyGateStatus string        `json:"topology_gate_status"`
	TopologyGateRef    string        `json:"topology_gate_ref"`
	GraphHash          string        `json:"graph_hash"`
	Source             string        `json:"source"`
	Confirmed          bool          `json:"confirmed"`
	ProjectedAt        string        `json:"projected_at,omitempty"`
}

// ProjectBranchContext projects a validated graph onto a
// BranchContext (pure function). The ref is content-addressed over
// the normalised graph, so it is stable per graph and changes with
// the graph. The gate status snapshot starts from the graph's own
// declared status (the v1 fixture carries one) or "unevaluated";
// engine projection and admission re-evaluate it live.
func ProjectBranchContext(g GitScopeGraph) BranchContext {
	scopes := make([]BranchScope, 0, len(g.Scopes))
	for _, s := range g.Scopes {
		wt := s.WorktreePath
		if wt == "" {
			wt = s.GitRoot
		}
		policy := s.DirtyPolicy
		if policy == "" {
			policy = "fail_closed"
		}
		scopes = append(scopes, BranchScope{
			ScopeID: s.ScopeID, GitRoot: s.GitRoot, WorktreePath: wt,
			Path: s.Path, Role: s.Role, Writable: s.WritableScope(g.ProjectTopology),
			DirtyPolicy: policy,
		})
	}
	norm, _ := json.Marshal(struct {
		Topology string        `json:"project_topology"`
		Scopes   []BranchScope `json:"scopes"`
	}{g.ProjectTopology, scopes})
	sum := sha256.Sum256(norm)
	hash := hex.EncodeToString(sum[:])
	status := g.TopologyGateStatus
	if status != TopologyGatePass && status != TopologyGateBlocked {
		status = TopologyUnevaluated
	}
	return BranchContext{
		Schema: BranchContextSchema, TopologyKind: g.ProjectTopology,
		Scopes: scopes, TopologyGateStatus: status,
		TopologyGateRef: "bc_" + hash[:16], GraphHash: hash,
	}
}

// ScopeVerdict is one scope's topology-gate outcome.
type ScopeVerdict struct {
	ScopeID     string   `json:"scope_id"`
	Role        string   `json:"role"`
	Status      string   `json:"status"` // pass | blocked
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Detail      string   `json:"detail,omitempty"`
}

// TopologyGateVerdict is the aggregate gate outcome (AC-172).
type TopologyGateVerdict struct {
	Status      string         `json:"status"` // pass | blocked
	ReasonCodes []string       `json:"reason_codes,omitempty"`
	Scopes      []ScopeVerdict `json:"scopes"`
	Ref         string         `json:"topology_gate_ref,omitempty"`
}

// ScopeWorkState is the observed working-tree state of one scope:
// Judged=false when the scope is not writable (the dirty policy is
// not applied to it); NotGit when its worktree cannot be inspected
// as a git repository at all; Violations are the S2 dirty-policy
// violation paths relative to the scope's judged base.
type ScopeWorkState struct {
	ScopeID    string
	NotGit     bool
	Violations []string
}

// EvaluateTopologyGate is the pure AC-172 judgement: writable
// scopes with violations block; any blocked scope blocks the
// aggregate (multi-git satellites included by construction).
func EvaluateTopologyGate(bc BranchContext, states []ScopeWorkState) TopologyGateVerdict {
	byID := map[string]ScopeWorkState{}
	for _, st := range states {
		byID[st.ScopeID] = st
	}
	v := TopologyGateVerdict{Status: TopologyGatePass, Ref: bc.TopologyGateRef}
	for _, sc := range bc.Scopes {
		sv := ScopeVerdict{ScopeID: sc.ScopeID, Role: sc.Role, Status: TopologyGatePass}
		st, seen := byID[sc.ScopeID]
		switch {
		case !sc.Writable:
			sv.Detail = "read-only scope: the dirty policy is not applied to it"
		case !seen:
			sv.Status = TopologyGateBlocked
			sv.ReasonCodes = []string{core.ReasonTopologyScopeUnreadable}
			sv.Detail = "no work state was collected for this writable scope; the gate does not guess pass"
		case st.NotGit:
			sv.Status = TopologyGateBlocked
			if sc.Role == "primary" && bc.TopologyKind == TopologyMultiGit {
				sv.ReasonCodes = []string{core.ReasonMultiGitPrimaryMissing}
				sv.Detail = "the primary scope's worktree is not a git repository (v1: multi_git_primary_missing)"
			} else {
				sv.ReasonCodes = []string{core.ReasonTopologyScopeUnreadable}
				sv.Detail = "the scope's worktree is not a git repository; its dirty state cannot be inspected"
			}
		case len(st.Violations) > 0:
			sv.Status = TopologyGateBlocked
			sv.ReasonCodes = []string{topologyDirtyCode(bc.TopologyKind, sc)}
			sv.Detail = "writable scope carries dirty-policy violations: " + strings.Join(st.Violations, ", ")
		default:
			sv.Detail = "writable scope clean under the dirty policy"
		}
		if sv.Status == TopologyGateBlocked {
			v.Status = TopologyGateBlocked
			v.ReasonCodes = append(v.ReasonCodes, sv.ReasonCodes...)
		}
		v.Scopes = append(v.Scopes, sv)
	}
	v.ReasonCodes = dedupStrs(v.ReasonCodes)
	return v
}

// topologyDirtyCode names a dirty writable scope with the v1.6.7
// codes where v1 has them.
func topologyDirtyCode(topology string, sc BranchScope) string {
	switch sc.Role {
	case "satellite":
		return core.ReasonSatelliteWorktreeDirty
	case "service":
		return core.ReasonMonorepoServiceDirty(sc.ScopeID)
	default: // primary
		return core.ReasonUnscopedDirtyState
	}
}

// topologyScopeStates collects the live work state of every scope
// of a recorded context. pkgRel is the feature's requirement
// package directory relative to the project root ("" when none):
// a scope only sees the package leg when the package actually
// lives under its judged base.
func (e *Engine) topologyScopeStates(bc BranchContext, projectRoot, pkgRel string) []ScopeWorkState {
	gctx := context.Background()
	pkgAbs := ""
	if pkgRel != "" {
		pkgAbs = filepath.Join(projectRoot, filepath.FromSlash(pkgRel))
	}
	var states []ScopeWorkState
	for _, sc := range bc.Scopes {
		st := ScopeWorkState{ScopeID: sc.ScopeID}
		if !sc.Writable {
			states = append(states, st)
			continue
		}
		gitDir := sc.GitRoot
		if !harness.IsGitRepo(gctx, gitDir) {
			st.NotGit = true
			states = append(states, st)
			continue
		}
		entries, err := harness.StatusEntries(gctx, gitDir)
		if err != nil {
			st.NotGit = true
			states = append(states, st)
			continue
		}
		// Judge the scope's own slice: its in-repo path, relativised.
		base := gitDir
		if sc.Path != "" {
			prefix := path.Clean(sc.Path) + "/"
			var scoped []string
			for _, p := range entries {
				if strings.HasPrefix(p, prefix) {
					scoped = append(scoped, strings.TrimPrefix(p, prefix))
				}
			}
			entries = scoped
			base = filepath.Join(gitDir, filepath.FromSlash(sc.Path))
		}
		if sc.Role == "primary" {
			// Primary scopes keep the S2 dirty policy: only the
			// requirement package and protected paths violate —
			// ordinary code may be dirty at the pre-implement point.
			relPkg := ""
			if pkgAbs != "" {
				if rel, err := filepath.Rel(base, pkgAbs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					relPkg = filepath.ToSlash(rel)
				}
			}
			st.Violations, _ = dirtyViolations(entries, relPkg)
		} else {
			// Satellites and services are delivery scopes: v1 judged
			// them by raw dirtiness of their territory (any dirty
			// entry in the scope's slice blocks — git_scope_graph's
			// satellite_worktree_dirty / monorepo_service_dirty).
			st.Violations = append(st.Violations, entries...)
		}
		states = append(states, st)
	}
	return states
}

// evaluateTopologyLive runs the gate over a recorded context
// against the live working trees.
func (e *Engine) evaluateTopologyLive(bc BranchContext, projectRoot, pkgRel string) TopologyGateVerdict {
	return EvaluateTopologyGate(bc, e.topologyScopeStates(bc, projectRoot, pkgRel))
}
