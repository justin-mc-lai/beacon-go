// Integrated delivery (program-integrated-delivery AC-001..003).
//
// Plan B: a lake may only start once its upstream handoff is a
// verified, merged commit. Handoffs record the contract fingerprint,
// the independent QA evidence and both SHAs. Disjoint lakes run on
// separate worktrees; unknown or shared change surfaces stay serial,
// and main-star merges are serialized. The whole sea is released by
// ONE human signature bound to one candidate SHA + evidence digest.
//
// This whole surface is opt-in: a program keeps its legacy behavior
// until ProgramIntegratedEnable flips Mode to "integrated".
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// Integrated mode + stage vocabulary (truth.md Domain FSM).
const (
	ProgramModeIntegrated = "integrated"

	ProgramStageIntegrating      = "integrating"
	ProgramStageAwaitingApproval = "awaiting_approval"
	ProgramStageFinalizing       = "finalizing"
	ProgramStageDelivered        = "delivered"
)

// ProgramHandoff is one recorded lake→main-star handoff. It is only
// written when the upstream run's independent QA passed AND its
// verified commit is already merged into the main star; the SHAs are
// re-checked against git at recording time.
type ProgramHandoff struct {
	Lake           string   `json:"lake"`
	LakeRunID      string   `json:"lake_run_id"`
	QAEvidence     []string `json:"qa_evidence,omitempty"`
	QADigest       string   `json:"qa_digest,omitempty"`
	SourceSHA      string   `json:"source_sha"`
	IntegrationSHA string   `json:"integration_sha"`
	ContractFP     string   `json:"contract_fingerprint"`
	Valid          bool     `json:"valid"`
	ReasonCode     string   `json:"reason_code,omitempty"`
	RecordedAt     string   `json:"recorded_at"`
}

// ProgramDispatch is one lake's workstation + run binding. It is the
// recovery anchor: a second dispatch reuses it instead of creating a
// second run or worktree.
type ProgramDispatch struct {
	Lake         string `json:"lake"`
	RunID        string `json:"run_id"`
	Branch       string `json:"branch,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
	Batch        int    `json:"batch"`
	StartedAt    string `json:"started_at,omitempty"`
	// FinishedAt closes the lake's active interval at its merge
	// attempt: [StartedAt, FinishedAt) is the concurrency evidence
	// AC-002 asks for (two disjoint lakes overlap, shared/serial
	// ones do not).
	FinishedAt string `json:"finished_at,omitempty"`
	MergedAt   string `json:"merged_at,omitempty"`
	Status     string `json:"status"` // active | merged | conflicted
	ReasonCode string `json:"reason_code,omitempty"`
}

// ProgramCandidate is the single integration candidate bound to the
// host run: one SHA, the lake/run list and the evidence that passed
// total QA.
type ProgramCandidate struct {
	SHA            string   `json:"sha"`
	EvidenceDigest string   `json:"evidence_digest"`
	Evidence       []string `json:"evidence,omitempty"`
	Lakes          []string `json:"lakes"`
	Runs           []string `json:"runs"`
	QAVerdict      string   `json:"qa_verdict,omitempty"`
	BoundAt        string   `json:"bound_at"`
}

// ProgramApproval is the one human signature, bound to the candidate
// SHA and its evidence digest.
type ProgramApproval struct {
	CandidateSHA   string `json:"candidate_sha"`
	EvidenceDigest string `json:"evidence_digest"`
	SignedBy       string `json:"signed_by"`
	SignedAt       string `json:"signed_at"`
}

// ProgramIntegratedView is the integrated surface.
type ProgramIntegratedView struct {
	RunID       string            `json:"run_id"`
	ProgramSlug string            `json:"program_slug"`
	Mode        string            `json:"mode"`
	Stage       string            `json:"stage"`
	Candidate   *ProgramCandidate `json:"candidate,omitempty"`
	Approval    *ProgramApproval  `json:"approval,omitempty"`
	Handoffs    []ProgramHandoff  `json:"handoffs,omitempty"`
	Dispatches  []ProgramDispatch `json:"dispatches,omitempty"`
	Route       ProgramRouteView  `json:"route"`
	Allowed     bool              `json:"allowed"`
	ReasonCode  string            `json:"reason_code,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Ready       []string          `json:"ready,omitempty"`
	Blocked     map[string]string `json:"blocked,omitempty"`
}

// ProgramDispatchResult is the dispatch surface.
type ProgramDispatchResult struct {
	RunID       string            `json:"run_id"`
	ProgramSlug string            `json:"program_slug"`
	Stage       string            `json:"stage"`
	Batch       int               `json:"batch"`
	Parallel    bool              `json:"parallel"`
	Dispatched  []ProgramDispatch `json:"dispatched,omitempty"`
	Recovered   []string          `json:"recovered,omitempty"`
	Serial      []string          `json:"serial,omitempty"`
	Route       ProgramRouteView  `json:"route"`
}

// handoffFor returns the recorded handoff for a lake, or nil.
func (p *ProgramState) handoffFor(lake string) *ProgramHandoff {
	if p == nil {
		return nil
	}
	for i := range p.Handoffs {
		if p.Handoffs[i].Lake == lake {
			return &p.Handoffs[i]
		}
	}
	return nil
}

func (p *ProgramState) dispatchFor(lake string) *ProgramDispatch {
	if p == nil {
		return nil
	}
	for i := range p.Dispatches {
		if p.Dispatches[i].Lake == lake {
			return &p.Dispatches[i]
		}
	}
	return nil
}

// livePlan returns a COPY of the plan whose handoffs were re-checked
// against git right now (and whose QA digest still matches its
// evidence). Callers use it for start decisions so a stale or tampered
// record in the snapshot can never unlock a lake: the snapshot is a
// cache, git is the truth. The original plan is left untouched.
func (e *Engine) livePlan(ctx context.Context, run store.Run, plan *ProgramState) *ProgramState {
	if plan == nil {
		return nil
	}
	root, err := e.runProjectRoot(run)
	if err != nil || root == "" {
		return plan
	}
	head, herr := harness.HeadCommit(ctx, root)
	if herr != nil {
		return plan
	}
	cp := *plan
	cp.Handoffs = make([]ProgramHandoff, 0, len(plan.Handoffs))
	for i := range plan.Handoffs {
		h := plan.Handoffs[i]
		if !liveHandoffValidity(ctx, root, &h, head) {
			// Keep the row so callers can distinguish "no handoff"
			// from "recorded handoff that no longer holds".
			h.Valid = false
			if h.ReasonCode == "" {
				h.ReasonCode = "handoff_not_live"
			}
		}
		cp.Handoffs = append(cp.Handoffs, h)
	}
	return &cp
}

// liveHandoffValidity re-checks one recorded handoff against real git:
// the source commit must still be an ancestor of the recorded
// integration commit, and that integration commit must still be on the
// main star. The QA digest must still match the recorded evidence.
func liveHandoffValidity(ctx context.Context, root string, h *ProgramHandoff, head string) bool {
	if h == nil || !h.Valid || head == "" {
		return false
	}
	if h.SourceSHA == "" || h.IntegrationSHA == "" || h.QADigest == "" {
		return false
	}
	if h.QADigest != digestOf(h.QAEvidence...) {
		return false
	}
	if ok, err := harness.IsAncestor(ctx, root, h.SourceSHA, h.IntegrationSHA); err != nil || !ok {
		return false
	}
	ok, err := harness.IsAncestor(ctx, root, h.IntegrationSHA, head)
	return err == nil && ok
}

// integratedStartBlock is the ONE shared start judgement: empty when
// the lake may start, else the reason it may not. Route, single-step
// drive and continuous drive all call this, so they cannot disagree.
func integratedStartBlock(p *ProgramState, slug string) string {
	l := p.lake(slug)
	if l == nil {
		return core.ReasonProgramLakeNotInPlan
	}
	for _, dep := range l.DependsOn {
		h := p.handoffFor(dep)
		if h == nil {
			return core.ReasonProgramNotIntegrated
		}
		if !h.Valid {
			return core.ReasonProgramHandoffInvalid
		}
	}
	return ""
}

func (e *Engine) loadIntegrated(hostRunID string) (store.Run, *runMeta, *ProgramState, error) {
	run, m, _, err := e.loadRun(hostRunID)
	if err != nil {
		return store.Run{}, nil, nil, err
	}
	if m.Program == nil {
		return store.Run{}, nil, nil, core.NewError(core.ReasonProgramPlanMissing, "program.integrated",
			"run "+hostRunID+" has no program plan")
	}
	if m.Program.Mode != ProgramModeIntegrated {
		return store.Run{}, nil, nil, core.NewError(core.ReasonProgramIntegratedModeRequired, "program.integrated",
			"program "+m.Program.Slug+" is not in integrated mode; enable it first")
	}
	return run, m, m.Program, nil
}

func (e *Engine) integratedView(run store.Run, m *runMeta, allowed bool, reason, detail string) ProgramIntegratedView {
	plan := m.Program
	route, _ := e.ProgramRoute(run.ID)
	v := ProgramIntegratedView{
		RunID: run.ID, ProgramSlug: plan.Slug, Mode: plan.Mode, Stage: plan.Stage,
		Candidate: plan.Candidate, Approval: plan.Approval,
		Handoffs: plan.Handoffs, Dispatches: plan.Dispatches,
		Route: route, Allowed: allowed, ReasonCode: reason, Detail: detail,
		Blocked: map[string]string{},
	}
	plan = e.livePlan(context.Background(), run, plan)
	for _, l := range plan.Lakes {
		if b := integratedStartBlock(plan, l.Slug); b != "" {
			v.Blocked[l.Slug] = b
		} else {
			v.Ready = append(v.Ready, l.Slug)
		}
	}
	return v
}

// ProgramIntegratedEnable flips an already-frozen program into
// integrated delivery. Legacy plans are untouched until this call.
func (e *Engine) ProgramIntegratedEnable(runID string) (ProgramIntegratedView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	if m.Program == nil {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramPlanMissing, "program.integrated.Enable",
			"run "+runID+" has no program plan")
	}
	if !m.Program.Frozen {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramPlanNotFrozen, "program.integrated.Enable",
			"freeze the program plan before enabling integrated delivery")
	}
	if m.Program.Mode != ProgramModeIntegrated {
		m.Program.Mode = ProgramModeIntegrated
		m.Program.Stage = ProgramStageIntegrating
		m.Program.UpdatedAt = nowISO()
		if err := e.appendEvent(run, "program_integrated_enabled", map[string]any{
			"program_slug": m.Program.Slug, "stage": m.Program.Stage,
		}); err != nil {
			return ProgramIntegratedView{}, err
		}
		if err := e.saveRun(run, m); err != nil {
			return ProgramIntegratedView{}, err
		}
	}
	return e.integratedView(run, m, true, "", ""), nil
}

// ProgramIntegratedHandoff verifies and records one lake handoff.
func (e *Engine) ProgramIntegratedHandoff(ctx context.Context, hostRunID, lake, claimedSource, claimedIntegration string, qaEvidence []string) (ProgramHandoff, error) {
	run, m, plan, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramHandoff{}, err
	}
	pl := plan.lake(lake)
	if pl == nil {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramLakeNotInPlan, "program.integrated.Handoff",
			"lake "+lake+" is not in program "+plan.Slug)
	}
	if pl.RunID == "" {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramHandoffInvalid, "program.integrated.Handoff",
			"lake "+lake+" has no bound run to hand off")
	}
	feat, err := e.lakeFeature(run, lake)
	if err != nil {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramLakeFeatureMiss, "program.integrated.Handoff",
			"no feature package "+lake+" in the host run's project version")
	}
	// Independent QA must have passed on the lake's feature and the
	// lake run must carry a verifier (QA evidence), not the maker's
	// own word.
	if g, gerr := e.store.Gates().Latest(feat.ID, "qa"); gerr != nil || g.Verdict != "pass" {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramHandoffInvalid, "program.integrated.Handoff",
			"lake "+lake+" has no passing independent qa gate")
	}
	lrun, lm, _, err := e.loadRun(pl.RunID)
	if err != nil {
		return ProgramHandoff{}, err
	}
	if lm.Verifier == nil || !lm.Verifier.Present || len(lm.Verifier.Refs) == 0 {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramHandoffInvalid, "program.integrated.Handoff",
			"lake run "+pl.RunID+" has no independent verifier evidence")
	}
	if len(qaEvidence) == 0 {
		qaEvidence = append([]string{}, lm.Verifier.Refs...)
	}
	sort.Strings(qaEvidence)

	rec := dispatchForWorktree(m, lake)
	if rec == nil || rec.Branch == "" {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramHandoffInvalid, "program.integrated.Handoff",
			"lake "+lake+" has no recorded worktree branch")
	}
	src, err := harness.HeadCommit(ctx, rec.Path)
	if err != nil {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramHandoffInvalid, "program.integrated.Handoff",
			"cannot read lake branch tip: "+strings.TrimSpace(err.Error()))
	}
	mainSHA, err := harness.HeadCommit(ctx, mustRoot(e, lrun))
	if err != nil {
		return ProgramHandoff{}, err
	}
	if claimedSource != "" && claimedSource != src {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramCommitMismatch, "program.integrated.Handoff",
			"claimed source SHA "+claimedSource+" != lake branch tip "+src)
	}
	if claimedIntegration != "" && claimedIntegration != mainSHA {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramCommitMismatch, "program.integrated.Handoff",
			"claimed integration SHA "+claimedIntegration+" != main-star tip "+mainSHA)
	}
	merged, err := harness.IsAncestor(ctx, mustRoot(e, lrun), src, mainSHA)
	if err != nil {
		return ProgramHandoff{}, err
	}
	if !merged {
		return ProgramHandoff{}, core.NewError(core.ReasonProgramNotIntegrated, "program.integrated.Handoff",
			"lake commit "+src+" is not merged into the main star "+mainSHA)
	}
	qaDigest := digestOf(qaEvidence...)
	h := ProgramHandoff{
		Lake: lake, LakeRunID: pl.RunID, QAEvidence: qaEvidence, QADigest: qaDigest,
		SourceSHA: src, IntegrationSHA: mainSHA,
		ContractFP: digestOf(lake, src, mainSHA, qaDigest, pl.Goal, strings.Join(pl.DependsOn, ",")),
		Valid:      true, RecordedAt: nowISO(),
	}
	upsertHandoff(plan, h)
	plan.UpdatedAt = nowISO()
	if err := e.appendEvent(run, "program_integrated_handoff", map[string]any{
		"program_slug": plan.Slug, "lake": lake, "source_sha": src,
		"integration_sha": mainSHA, "contract_fingerprint": h.ContractFP,
	}); err != nil {
		return ProgramHandoff{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramHandoff{}, err
	}
	return h, nil
}

func mustRoot(e *Engine, run store.Run) string {
	root, err := e.runProjectRoot(run)
	if err != nil {
		return ""
	}
	return root
}

func dispatchForWorktree(m *runMeta, lake string) *WorktreeRecord {
	if m == nil || m.Implement == nil || m.Implement.Worktrees == nil {
		return nil
	}
	return m.Implement.Worktrees.find(lake)
}

func upsertHandoff(p *ProgramState, h ProgramHandoff) {
	for i := range p.Handoffs {
		if p.Handoffs[i].Lake == h.Lake {
			p.Handoffs[i] = h
			return
		}
	}
	p.Handoffs = append(p.Handoffs, h)
}

func digestOf(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// ProgramIntegratedDispatch starts the next non-conflicting batch.
func (e *Engine) ProgramIntegratedDispatch(ctx context.Context, hostRunID string) (ProgramDispatchResult, error) {
	run, m, plan, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramDispatchResult{}, err
	}
	if plan.Stage != ProgramStageIntegrating {
		return ProgramDispatchResult{}, core.NewError(core.ReasonProgramDispatchStage, "program.integrated.Dispatch",
			"dispatch is only valid while integrating (stage="+plan.Stage+")")
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return ProgramDispatchResult{}, err
	}
	var toStart []string
	var recovered []string
	live := e.livePlan(ctx, run, plan)
	order, _ := TopoOrder(plan.Lakes)
	for _, slug := range order {
		if plan.handoffFor(slug) != nil {
			continue
		}
		if feat, ferr := e.lakeFeature(run, slug); ferr == nil {
			if done, _ := e.featureCompleted(feat.ID); done {
				continue
			}
		}
		if integratedStartBlock(live, slug) != "" {
			continue
		}
		// An existing binding is the recovery anchor: reuse it and
		// never dispatch the lake twice (AC-002). A binding whose
		// worktree is gone (deleted dir) or unusable is NOT reused
		// silently — it falls through so startIntegratedLake rebuilds.
		if d := plan.dispatchFor(slug); d != nil && d.RunID != "" {
			if e.dispatchBindingReady(ctx, root, d) {
				recovered = append(recovered, slug)
				continue
			}
		}
		toStart = append(toStart, slug)
	}
	// Group candidates that are not yet started. Unknown change
	// surfaces and shared change surfaces stay serial; only
	// explicitly declared, pairwise-disjoint lakes form one batch.
	var group []string
	for _, slug := range toStart {
		if len(group) == 0 {
			group = append(group, slug)
			continue
		}
		if parallelCompatible(plan, group, slug) {
			group = append(group, slug)
		}
	}
	serial := diffSlugs(toStart, group)
	res := ProgramDispatchResult{RunID: run.ID, ProgramSlug: plan.Slug, Stage: plan.Stage,
		Parallel: len(group) > 1, Serial: serial, Recovered: recovered}
	if len(group) == 0 {
		res.Route, _ = e.ProgramRoute(hostRunID)
		return res, nil
	}
	plan.BatchSeq++
	batch := plan.BatchSeq
	res.Batch = batch
	// AC-002: a single ready lake reuses the main workstation; only a
	// genuinely parallel batch gets independent satellite worktrees.
	workOnMain := len(group) == 1 && noOtherActiveDispatch(plan, group)
	for _, slug := range group {
		d, wasRecovered, derr := e.startIntegratedLake(ctx, run, m, plan, root, slug, batch, workOnMain)
		if derr != nil {
			return ProgramDispatchResult{}, derr
		}
		res.Dispatched = append(res.Dispatched, d)
		if wasRecovered {
			res.Recovered = append(res.Recovered, slug)
		}
	}
	plan.UpdatedAt = nowISO()
	if err := e.appendEvent(run, "program_integrated_dispatch", map[string]any{
		"program_slug": plan.Slug, "batch": batch, "parallel": res.Parallel,
		"lakes": group, "serial": serial,
	}); err != nil {
		return ProgramDispatchResult{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramDispatchResult{}, err
	}
	res.Route, _ = e.ProgramRoute(hostRunID)
	return res, nil
}

// noOtherActiveDispatch reports whether every other dispatched lake is
// already finished: only then is a lone ready lake truly alone and may
// reuse the main workstation.
func noOtherActiveDispatch(plan *ProgramState, group []string) bool {
	in := map[string]bool{}
	for _, s := range group {
		in[s] = true
	}
	for _, d := range plan.Dispatches {
		if d.Status == "active" && !in[d.Lake] {
			return false
		}
	}
	return true
}

// startIntegratedLake ensures one lake's worktree + run binding and
// returns (recovered=true) when an existing binding was reused.
func (e *Engine) startIntegratedLake(ctx context.Context, run store.Run, m *runMeta, plan *ProgramState, root, slug string, batch int, workOnMain bool) (ProgramDispatch, bool, error) {
	d := plan.dispatchFor(slug)
	if d != nil && d.RunID != "" && d.Status == "active" && e.dispatchBindingReady(ctx, root, d) {
		return *d, true, nil
	}
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	if m.Implement.Worktrees == nil {
		m.Implement.Worktrees = &WorktreeState{}
	}
	book := m.Implement.Worktrees
	rec := book.find(slug)
	branch := "program/" + run.ID + "/" + slug
	path := filepath.Join(root, WorktreeDirName, slug)
	if rec != nil {
		if rec.Branch != "" {
			branch = rec.Branch
		}
		if rec.Path != "" {
			path = rec.Path
		}
	}
	startPoint := ""
	if rec != nil {
		startPoint = rec.BaseRef
	}
	if startPoint == "" {
		startPoint, _ = harness.HeadCommit(ctx, root)
	}
	action := "main_workstation"
	if workOnMain {
		// 单湖复用主工位 (AC-002): no satellite is created when only one
		// lake is ready; its work lands on the host's own workstation.
		mainBranch, berr := harness.CurrentBranch(ctx, root)
		if berr != nil || strings.TrimSpace(mainBranch) == "" {
			mainBranch = "main"
		}
		branch, path = mainBranch, root
	} else {
		// 建工位中断 (AC-002): a binding may point at a worktree that was
		// deleted, or a directory may be left behind without a usable
		// branch. Never reuse either silently: rebuild the worktree.
		var aerr error
		action, aerr = ensureIntegratedWorktree(ctx, root, path, branch, startPoint)
		if aerr != nil {
			return ProgramDispatch{}, false, core.NewError(core.ReasonWorktreeMissing, "program.integrated.Dispatch",
				"worktree restore failed for lake "+slug+": "+strings.TrimSpace(aerr.Error()))
		}
	}
	rebuilt := action == "rebuilt"
	if rec != nil {
		rec.State = WorktreeActive
		rec.Branch, rec.Path = branch, path
		if rebuilt {
			_ = e.appendEvent(run, "worktree_rebuilt", map[string]any{
				"lake": slug, "mode": "integrated", "branch": branch, "path": path,
				"base_ref": startPoint, "reason": "binding or directory was not usable", "real": true,
			})
		}
	} else {
		ref := filepath.Join(WorktreeDirName, slug)
		if workOnMain {
			ref = ""
		}
		book.Records = append(book.Records, WorktreeRecord{
			Lake: slug, Branch: branch, Path: path, Ref: ref,
			BaseRef: startPoint, State: WorktreeActive, CreatedAt: nowISO(),
		})
		eventType := "worktree_created"
		if workOnMain {
			eventType = "worktree_main_reused"
		} else if action == "reused" {
			eventType = "worktree_adopted"
		}
		_ = e.appendEvent(run, eventType, map[string]any{
			"lake": slug, "mode": "integrated", "branch": branch, "path": path, "real": true,
		})
	}
	lakeRunID := ""
	if pl := plan.lake(slug); pl != nil {
		lakeRunID = pl.RunID
	}
	if lakeRunID == "" {
		if feat, ferr := e.lakeFeature(run, slug); ferr == nil {
			if runs, rerr := e.store.Runs().ListByFeature(feat.ID); rerr == nil {
				for _, r := range runs {
					// Never bind the host run as its own lake run.
					if r.ID == run.ID {
						continue
					}
					if r.Status == core.RunRunning || r.Status == core.RunCreated {
						lakeRunID = r.ID
					}
				}
			}
		}
	}
	if lakeRunID == "" {
		fc, ok, ferr := e.featureContextOf(run.FeatureID)
		if ferr != nil {
			return ProgramDispatch{}, false, ferr
		}
		if !ok {
			return ProgramDispatch{}, false, core.NewError(ReasonProjectNotRegistered, "program.integrated.Dispatch",
				"no feature context for host run "+run.ID)
		}
		created, cerr := e.CreateRun(CreateRunOptions{
			ProjectRoot: fc.Project.RootPath, Version: fc.Version.Version,
			FeatureSlug: slug, Utterance: "integrated program: " + slug,
			ExecutorID: m.ExecutorID,
		})
		if cerr != nil {
			return ProgramDispatch{}, false, cerr
		}
		lakeRunID = created.RunID
	}
	if pl := plan.lake(slug); pl != nil {
		pl.RunID = lakeRunID
	}
	if d == nil {
		plan.Dispatches = append(plan.Dispatches, ProgramDispatch{Lake: slug})
		d = plan.dispatchFor(slug)
	}
	d.RunID, d.Branch, d.WorktreePath = lakeRunID, branch, path
	d.Batch, d.StartedAt, d.FinishedAt, d.Status, d.ReasonCode = batch, nowISO(), "", "active", ""
	return *d, rebuilt, nil
}

// dispatchBindingReady reports whether an existing dispatch's run and
// worktree are actually materialized; a deleted directory or a bogus
// branch means the binding must be rebuilt, not silently reused.
func (e *Engine) dispatchBindingReady(ctx context.Context, root string, d *ProgramDispatch) bool {
	if d == nil || d.RunID == "" || d.WorktreePath == "" || d.Branch == "" {
		return false
	}
	if _, err := e.store.Runs().GetByID(d.RunID); err != nil {
		return false
	}
	return worktreeUsable(ctx, root, d.WorktreePath, d.Branch)
}

func worktreeUsable(ctx context.Context, root, path, branch string) bool {
	return pathExists(path) && harness.BranchExists(ctx, root, branch) && worktreeRegistered(ctx, root, path)
}

// worktreeRegistered reports whether git itself lists path as a live
// worktree (a plain directory left behind is not one).
func worktreeRegistered(ctx context.Context, root, path string) bool {
	list, err := harness.WorktreeList(ctx, root)
	if err != nil {
		return false
	}
	want, werr := filepath.EvalSymlinks(path)
	if werr != nil {
		want = filepath.Clean(path)
	}
	for p := range list {
		got, gerr := filepath.EvalSymlinks(p)
		if gerr != nil {
			got = filepath.Clean(p)
		}
		if got == want {
			return true
		}
	}
	return false
}

// ensureIntegratedWorktree makes path a live worktree on branch.
// Returns "reused" when it already is one, "created" when both the
// directory and branch were absent, and "rebuilt" when a surviving
// branch or a stale directory had to be repaired. A directory with no
// usable branch is pruned/removed, never left to fail forever with
// exit 128.
func ensureIntegratedWorktree(ctx context.Context, root, path, branch, startPoint string) (string, error) {
	hadDir := pathExists(path)
	hadBranch := harness.BranchExists(ctx, root, branch)
	if hadDir {
		if worktreeRegistered(ctx, root, path) && hadBranch {
			return "reused", nil
		}
		_, _ = harness.WorktreePrune(ctx, root)
		if worktreeRegistered(ctx, root, path) {
			// A registered worktree can still hold uncommitted work;
			// force-removing it would delete someone's files. Refuse
			// and keep the directory (AC-002).
			dirty, derr := harness.HasUncommittedChanges(ctx, path)
			if derr != nil {
				return "", derr
			}
			if dirty {
				return "", fmt.Errorf("%s is a registered worktree with uncommitted changes; refusing to remove it — commit or move the work, then re-dispatch", path)
			}
			_, _ = harness.WorktreeRemove(ctx, root, path, true)
		}
		// A leftover directory is only ours to remove when it is empty.
		// Anything with files in it may be someone's work: refuse and
		// keep the files (AC-002: recovery must never delete data).
		if pathExists(path) {
			entries, rerr := os.ReadDir(path)
			if rerr != nil {
				return "", rerr
			}
			if len(entries) > 0 {
				return "", fmt.Errorf("leftover directory %s is not a registered worktree but holds %d entries; refusing to delete it — move those files away and re-dispatch",
					path, len(entries))
			}
			if err := os.Remove(path); err != nil {
				return "", err
			}
		}
	}
	if hadBranch {
		if _, err := harness.WorktreeCheckout(ctx, root, path, branch); err != nil {
			return "", err
		}
		return "rebuilt", nil
	}
	if _, err := harness.WorktreeAdd(ctx, root, path, branch, startPoint); err != nil {
		return "", err
	}
	if hadDir {
		return "rebuilt", nil
	}
	return "created", nil
}

// parallelCompatible: a lake joins the parallel group only when both
// it and every group member declare a non-empty change surface and
// the surfaces are pairwise disjoint. Unknown or shared surfaces are
// serialized.
func parallelCompatible(plan *ProgramState, group []string, slug string) bool {
	l := plan.lake(slug)
	if l == nil || len(l.Touches) == 0 {
		return false
	}
	for _, other := range group {
		o := plan.lake(other)
		if o == nil || len(o.Touches) == 0 {
			return false
		}
		for _, a := range l.Touches {
			for _, b := range o.Touches {
				if strings.TrimSpace(a) == strings.TrimSpace(b) {
					return false
				}
			}
		}
	}
	return true
}

func diffSlugs(all, keep []string) []string {
	in := map[string]bool{}
	for _, s := range keep {
		in[s] = true
	}
	var out []string
	for _, s := range all {
		if !in[s] {
			out = append(out, s)
		}
	}
	return out
}

func pathExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// ProgramIntegratedMerge lands one lake branch on the main star.
// Merges are serialized by mergeMu; a conflict stops the merge and
// leaves dependents blocked (no handoff can be recorded).
func (e *Engine) ProgramIntegratedMerge(ctx context.Context, hostRunID, lake string) (ProgramIntegratedView, error) {
	run, m, plan, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	rec := dispatchForWorktree(m, lake)
	if rec == nil || rec.Branch == "" {
		return ProgramIntegratedView{}, core.NewError(core.ReasonWorktreeMissing, "program.integrated.Merge",
			"no worktree branch recorded for lake "+lake)
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	e.mergeMu.Lock()
	conflicts, _, mergeErr := harness.MergeInto(ctx, root, rec.Branch)
	e.mergeMu.Unlock()
	d := plan.dispatchFor(lake)
	if len(conflicts) > 0 || mergeErr != nil {
		if d != nil {
			d.Status = "conflicted"
			d.FinishedAt = nowISO()
			d.ReasonCode = core.ReasonWorktreeMergeConflict
		}
		plan.UpdatedAt = nowISO()
		_ = e.appendEvent(run, "program_integrated_merge_conflict", map[string]any{
			"program_slug": plan.Slug, "lake": lake, "branch": rec.Branch, "files": conflicts,
			"reason_code": core.ReasonWorktreeMergeConflict,
		})
		_ = e.saveRun(run, m)
		detail := strings.Join(conflicts, ", ")
		if detail == "" && mergeErr != nil {
			detail = strings.TrimSpace(mergeErr.Error())
		}
		return e.integratedView(run, m, false, core.ReasonWorktreeMergeConflict, detail),
			core.NewError(core.ReasonWorktreeMergeConflict, "program.integrated.Merge",
				"merging "+rec.Branch+" into the main star conflicts in "+detail+"; dependents stay blocked")
	}
	rec.State = WorktreeMerged
	rec.MergedAt = nowISO()
	if d != nil {
		d.Status, d.MergedAt, d.FinishedAt, d.ReasonCode = "merged", nowISO(), nowISO(), ""
	}
	plan.UpdatedAt = nowISO()
	_ = e.appendEvent(run, "program_integrated_merge", map[string]any{
		"program_slug": plan.Slug, "lake": lake, "branch": rec.Branch, "direction": "satellite->main_star",
	})
	if err := e.saveRun(run, m); err != nil {
		return ProgramIntegratedView{}, err
	}
	return e.integratedView(run, m, true, "", ""), nil
}

// ProgramIntegratedRoute is the read-only view with the shared start
// decisions (used by callers and TC-001's three-entry check).
func (e *Engine) ProgramIntegratedRoute(hostRunID string) (ProgramIntegratedView, error) {
	run, m, _, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	return e.integratedView(run, m, true, "", ""), nil
}

// ProgramIntegratedQA records total QA for the candidate. All lakes
// must be at their gate and every lake handoff must be an ancestor of
// the one candidate SHA; only then does the program enter
// awaiting_approval.
func (e *Engine) ProgramIntegratedQA(ctx context.Context, hostRunID, verdict string, evidence []string, by string) (ProgramIntegratedView, error) {
	run, m, plan, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	if plan.Stage != ProgramStageIntegrating && plan.Stage != ProgramStageAwaitingApproval {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.QA",
			"total QA is only valid while integrating/awaiting_approval (stage="+plan.Stage+")")
	}
	if strings.TrimSpace(verdict) != "pass" {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.QA",
			"total QA verdict is "+verdict+", not pass")
	}
	if len(evidence) == 0 {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.QA",
			"total QA needs evidence refs")
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	sha, err := harness.HeadCommit(ctx, root)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	order, _ := TopoOrder(plan.Lakes)
	var lakes, runs []string
	for _, slug := range order {
		pl := plan.lake(slug)
		if pl.RunID == "" {
			return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.QA",
				"lake "+slug+" has no bound run")
		}
		if stage, _ := e.lakeRunStage(pl.RunID); stage != string(core.LoopReleaseGate) && stage != string(core.LoopDone) {
			return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.QA",
				"lake "+slug+" is not at its release gate (stage="+stage+")")
		}
		h := plan.handoffFor(slug)
		if !liveHandoffValidity(ctx, root, h, sha) {
			return ProgramIntegratedView{}, core.NewError(core.ReasonProgramNotIntegrated, "program.integrated.QA",
				"lake "+slug+" has no live valid handoff (recorded handoff must still be merged on the main star)")
		}
		ok, aerr := harness.IsAncestor(ctx, root, h.IntegrationSHA, sha)
		if aerr != nil || !ok {
			return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.QA",
				"lake "+slug+" handoff "+h.IntegrationSHA+" is not part of candidate "+sha)
		}
		lakes = append(lakes, slug)
		runs = append(runs, pl.RunID)
	}
	sort.Strings(evidence)
	digest := digestOf(append([]string{sha}, evidence...)...)
	plan.Candidate = &ProgramCandidate{
		SHA: sha, EvidenceDigest: digest, Evidence: evidence,
		Lakes: lakes, Runs: runs, QAVerdict: "pass", BoundAt: nowISO(),
	}
	plan.Stage = ProgramStageAwaitingApproval
	plan.Approval = nil
	plan.UpdatedAt = nowISO()
	_ = e.appendEvent(run, "program_integrated_candidate_ready", map[string]any{
		"program_slug": plan.Slug, "candidate_sha": sha, "evidence_digest": digest,
		"by": by, "lakes": lakes,
	})
	if err := e.saveRun(run, m); err != nil {
		return ProgramIntegratedView{}, err
	}
	return e.integratedView(run, m, true, "", ""), nil
}

// ProgramIntegratedApprove persists the ONE human signature bound to
// the candidate, then enters finalizing.
func (e *Engine) ProgramIntegratedApprove(ctx context.Context, hostRunID, candidateSHA, evidenceDigest, signedBy string) (ProgramIntegratedView, error) {
	run, m, plan, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	if strings.TrimSpace(signedBy) == "" {
		return ProgramIntegratedView{}, core.NewError(core.ReasonGateEvaluatorRequired, "program.integrated.Approve",
			"a human identity is required (--confirmed-by)")
	}
	if plan.Stage != ProgramStageAwaitingApproval || plan.Candidate == nil {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramApprovalInvalid, "program.integrated.Approve",
			"there is no candidate awaiting approval (stage="+plan.Stage+")")
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	sha, err := harness.HeadCommit(ctx, root)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	if sha != plan.Candidate.SHA {
		oldSHA := plan.Candidate.SHA
		plan.Stage = ProgramStageIntegrating
		plan.Approval = nil
		plan.Candidate = nil
		plan.UpdatedAt = nowISO()
		_ = e.appendEvent(run, "program_integrated_candidate_changed", map[string]any{
			"program_slug": plan.Slug, "old_sha": oldSHA, "new_sha": sha,
		})
		_ = e.saveRun(run, m)
		return e.integratedView(run, m, false, core.ReasonProgramCandidateChanged, "main star moved to "+sha),
			core.NewError(core.ReasonProgramCandidateChanged, "program.integrated.Approve",
				"candidate changed: main star is "+sha+", candidate was "+oldSHA+"; re-run total QA")
	}
	if candidateSHA != plan.Candidate.SHA || evidenceDigest != plan.Candidate.EvidenceDigest {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramApprovalInvalid, "program.integrated.Approve",
			"the signature does not match the bound candidate SHA + evidence digest")
	}
	plan.Approval = &ProgramApproval{
		CandidateSHA: plan.Candidate.SHA, EvidenceDigest: plan.Candidate.EvidenceDigest,
		SignedBy: signedBy, SignedAt: nowISO(),
	}
	plan.Stage = ProgramStageFinalizing
	plan.UpdatedAt = nowISO()
	_ = e.appendEvent(run, "program_integrated_approved", map[string]any{
		"program_slug": plan.Slug, "candidate_sha": plan.Candidate.SHA,
		"evidence_digest": plan.Candidate.EvidenceDigest, "by": signedBy,
	})
	if err := e.saveRun(run, m); err != nil {
		return ProgramIntegratedView{}, err
	}
	return e.integratedView(run, m, true, "", ""), nil
}

// ProgramIntegratedFinalize completes every lake and closes the host.
// Idempotent for the same candidate (中断可续).
func (e *Engine) ProgramIntegratedFinalize(ctx context.Context, hostRunID string) (ProgramIntegratedView, error) {
	run, m, plan, err := e.loadIntegrated(hostRunID)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	if plan.Stage == ProgramStageDelivered {
		return e.integratedView(run, m, true, "", ""), nil
	}
	if plan.Stage != ProgramStageFinalizing || plan.Approval == nil || plan.Candidate == nil {
		return ProgramIntegratedView{}, core.NewError(core.ReasonProgramApprovalRequired, "program.integrated.Finalize",
			"a valid approval for the candidate is required (stage="+plan.Stage+")")
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	sha, err := harness.HeadCommit(ctx, root)
	if err != nil {
		return ProgramIntegratedView{}, err
	}
	if sha != plan.Candidate.SHA || plan.Approval.CandidateSHA != plan.Candidate.SHA {
		plan.Stage = ProgramStageIntegrating
		plan.Approval = nil
		plan.Candidate = nil
		plan.UpdatedAt = nowISO()
		_ = e.saveRun(run, m)
		return e.integratedView(run, m, false, core.ReasonProgramCandidateChanged, ""),
			core.NewError(core.ReasonProgramCandidateChanged, "program.integrated.Finalize",
				"candidate changed before finalize; the old signature cannot cover it")
	}
	for _, slug := range plan.Candidate.Lakes {
		pl := plan.lake(slug)
		if pl == nil || pl.RunID == "" {
			return ProgramIntegratedView{}, core.NewError(core.ReasonProgramCandidateNotReady, "program.integrated.Finalize",
				"lake "+slug+" has no bound run")
		}
		if stage, _ := e.lakeRunStage(pl.RunID); stage == string(core.LoopDone) {
			pl.ProgramStatus = "completed"
			continue
		}
		if _, cerr := e.Complete(pl.RunID, CompleteOptions{
			HumanConfirmed: true, ConfirmedBy: "program:" + plan.Approval.SignedBy,
		}); cerr != nil {
			return ProgramIntegratedView{}, cerr
		}
		pl.ProgramStatus = "completed"
		pl.UpdatedAt = nowISO()
	}
	plan.Stage = ProgramStageDelivered
	plan.UpdatedAt = nowISO()
	_ = e.appendEvent(run, "program_integrated_delivered", map[string]any{
		"program_slug": plan.Slug, "candidate_sha": plan.Candidate.SHA,
		"by": plan.Approval.SignedBy, "lakes": plan.Candidate.Lakes,
	})
	if err := e.saveRun(run, m); err != nil {
		return ProgramIntegratedView{}, err
	}
	return e.integratedView(run, m, true, "", ""), nil
}
