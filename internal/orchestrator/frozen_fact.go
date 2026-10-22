// The shared frozen determination (P4b fix of the P4a observation
// item): ONE function answers "is this feature's truth frozen right
// now", and every consumer — implement admission (G-04), the truth
// stage exit gate (G-03) and the program lake facts — reads its
// answer from here. Until this fix each consumer derived frozenness
// from the lifecycle snapshot (Freeze/Completed) or a formal
// truth_freeze gate pass only, so a package whose truth.md was
// frozen on disk stopped counting as frozen the moment CreateRun
// advanced the lifecycle to Prd/InProgress, and admission reported
// freeze_ack_required for a package that was, in fact, frozen.
//
// The determination order (the P3a primary-source caliber, applied
// uniformly):
//
//  1. change_pending — an unrefrozen change request is on the
//     trail: NOT frozen, whatever anything else says. A pending
//     change means the frozen text is under authorised edit.
//  2. frontmatter — the truth document itself speaks: status ==
//     frozen means frozen (package-authoritative, AC/P3a primary
//     source); any other non-empty status means NOT frozen — the
//     document outranks a stale gate verdict minted earlier.
//  3. formal gate — only when the document is silent (file
//     missing/unreadable, no frontmatter, no status scalar) may
//     the latest formal truth_freeze verdict corroborate: pass
//     means frozen.
//  4. otherwise NOT frozen (fail-closed). The lifecycle snapshot
//     is carried on the fact as reference information only; it
//     never decides — after a freeze the lifecycle legitimately
//     moves on to Implement/QA, and before a freeze it can be
//     parked at Freeze by flows that never froze the document.
//
// Why this is not circular with the truth_freeze GATE: the gate is
// the write side — it itemises the package's own declarations
// (four-piece files, the frozen marker, no pending change) and
// mints a verdict. This function is the read side — it answers
// from the document first and consults the minted verdict only
// when the document cannot speak. The gate never consults this
// function, so no loop exists: "the package declares itself
// frozen" (gate's frozen_marker item) and "the engine treats the
// package as frozen" (this fact) are two faces of the same
// declaration, checked on the write side, consumed on the read
// side.
package orchestrator

import (
	"database/sql"
	"errors"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// Frozen sources: which step of the determination decided.
const (
	FrozenSourceChangePending = "change_pending"
	FrozenSourceFrontmatter   = "frontmatter"
	FrozenSourceFormalGate    = "formal_gate"
	FrozenSourceNone          = "none"
)

// FrozenFact is the shared frozen determination for one feature:
// the verdict, which source decided it, and every signal that fed
// it (so surfaces can explain the answer without re-deriving it).
// When the package is frozen it also carries the S1 dimensions:
// the branch fact of its project (AC-136) and the baseline fact
// (AC-139 — whether the frozen body is a committed baseline on
// the canonical branch). The baseline is a SEPARATE dimension:
// Frozen still answers "is it frozen", consumers that need a
// consumable baseline (implement admission, the truth exit gate)
// additionally require BaselineStatus == committed.
type FrozenFact struct {
	Frozen            bool     `json:"frozen"`
	Source            string   `json:"source"`
	ChangePending     bool     `json:"change_pending"`
	PendingChangeIDs  []string `json:"pending_change_ids,omitempty"`
	FrontmatterStatus string   `json:"frontmatter_status,omitempty"` // "" = document silent
	FormalGatePass    bool     `json:"formal_gate_pass"`
	FormalGateVerdict string   `json:"formal_gate_verdict,omitempty"`
	LifecycleFrozen   bool     `json:"lifecycle_frozen"` // reference only, never decides
	LifecycleStage    string   `json:"lifecycle_stage,omitempty"`
	LifecycleStatus   string   `json:"lifecycle_status,omitempty"`
	// S1 (AC-136/139), populated only when Frozen:
	BranchCheck     string `json:"branch_check,omitempty"`
	CurrentBranch   string `json:"current_branch,omitempty"`
	CanonicalBranch string `json:"canonical_branch,omitempty"`
	BaselineStatus  string `json:"baseline_status,omitempty"` // committed | uncommitted | not_a_git_repo | no_document
	BaselineCommit  string `json:"baseline_commit,omitempty"`
	BaselineDetail  string `json:"baseline_detail,omitempty"`
}

// frontmatterStatusOf extracts the `status` scalar from a package
// file's frontmatter. parsed=false means the document carries no
// readable frontmatter (missing file, no --- block) — the document
// is silent, which is NOT the same as declaring a non-frozen
// status.
func frontmatterStatusOf(pf gate.PackageFile) (status string, parsed bool) {
	if !pf.Present {
		return "", false
	}
	scalars, _, ok := gate.ParseFrontmatter(pf.Text)
	if !ok {
		return "", false
	}
	return scalars["status"], true
}

// featureContextByID assembles the full feature context from a
// feature id alone (the exit-gate / program call sites hold ids,
// not contexts).
func (e *Engine) featureContextByID(featureID string) (store.FeatureContext, error) {
	feat, err := e.store.Features().GetByID(featureID)
	if err != nil {
		return store.FeatureContext{}, err
	}
	ver, err := e.store.Projects().GetVersionByID(feat.ProjectVersionID)
	if err != nil {
		return store.FeatureContext{}, err
	}
	proj, err := e.store.Projects().GetByID(ver.ProjectID)
	if err != nil {
		return store.FeatureContext{}, err
	}
	return store.FeatureContext{Project: proj, Version: ver, Feature: feat}, nil
}

// FrozenFact runs the shared determination for the feature of ctx.
func (e *Engine) FrozenFact(ctx store.FeatureContext) (FrozenFact, error) {
	feat := ctx.Feature
	var f FrozenFact
	pending, err := e.pendingChangeIDs(feat.ID)
	if err != nil {
		return FrozenFact{}, err
	}
	f.ChangePending = len(pending) > 0
	f.PendingChangeIDs = pending
	truth := readPkgFile(ctx.Project.RootPath, feat.TruthPath)
	if status, parsed := frontmatterStatusOf(truth); parsed {
		f.FrontmatterStatus = status
	}
	if g, err := e.store.Gates().Latest(feat.ID, string(gate.GateTruthFreeze)); err == nil {
		f.FormalGateVerdict = g.Verdict
		f.FormalGatePass = g.Verdict == string(gate.VerdictPass)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return FrozenFact{}, err
	}
	if lc, err := e.store.Features().GetLifecycle(feat.ID); err == nil {
		f.LifecycleStage = string(lc.Stage)
		f.LifecycleStatus = string(lc.Status)
		f.LifecycleFrozen = lc.Stage == core.StageFreeze && lc.Status == core.StatusCompleted
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return FrozenFact{}, err
	}
	switch {
	case f.ChangePending:
		f.Frozen, f.Source = false, FrozenSourceChangePending
	case f.FrontmatterStatus == "frozen":
		f.Frozen, f.Source = true, FrozenSourceFrontmatter
	case f.FrontmatterStatus != "":
		// The document speaks and does not declare frozen; a stale
		// formal pass cannot resurrect it.
		f.Frozen, f.Source = false, FrozenSourceFrontmatter
	case f.FormalGatePass:
		f.Frozen, f.Source = true, FrozenSourceFormalGate
	default:
		f.Frozen, f.Source = false, FrozenSourceNone
	}
	if f.Frozen {
		// S1 dimensions (branch.go): the branch fact of the
		// project and the committed-baseline state of the frozen
		// body. `truth` here is the package file FrozenFact
		// already read for the frontmatter — the recorded-
		// baseline verification hashes exactly those bytes.
		bi := e.branchInfo(ctx.Project.RootPath)
		f.BranchCheck, f.CurrentBranch, f.CanonicalBranch = bi.BranchCheck, bi.CurrentBranch, bi.CanonicalBranch
		bl := e.factBaseline(ctx, truth)
		f.BaselineStatus, f.BaselineCommit, f.BaselineDetail = bl.Status, bl.Commit, bl.Detail
	}
	return f, nil
}

// frozenFactByID is FrozenFact for call sites holding only a
// feature id. A feature that cannot be resolved at all yields the
// fail-closed zero fact (not frozen, source none) rather than an
// error: the callers (exit gates, admission) must still be able
// to say "blocked", never crash open.
func (e *Engine) frozenFactByID(featureID string) (FrozenFact, error) {
	if featureID == "" {
		return FrozenFact{Source: FrozenSourceNone}, nil
	}
	ctx, err := e.featureContextByID(featureID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FrozenFact{Source: FrozenSourceNone}, nil
		}
		return FrozenFact{}, err
	}
	return e.FrozenFact(ctx)
}
