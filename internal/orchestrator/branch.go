// Standard flow S1 (AC-136/139): the canonical-branch discipline
// and the committed-baseline machine.
//
// AC-136: truth writes (create / freeze / change / refreeze /
// baseline) may only happen on the truth canonical branch
// (truth_canonical_branch; default main, master when main is
// missing — the frozen R10 contract). The Go kernel has no
// governance-contract store, so the branch is resolved as:
//  1. an explicit project override — the `truth_canonical_branch`
//     key in the project root's .beacon/config (the project-local
//     config file the kernel already uses for .beacon/loop/ and
//     .beacon/artifacts/ state; no store column may be added —
//     the nine-table schema is frozen);
//  2. `main` when that branch exists, else `master` when it does;
//  3. `main` (the frozen default).
//
// A project that is not a git repository is NOT subject to branch
// discipline (this slice's caliber): the guard passes and every
// surface reports branch_check=not_a_git_repo instead of silently
// skipping.
//
// AC-139: a refreeze only becomes a consumable baseline once it
// is committed on the canonical branch. The baseline state of a
// frozen package is a recorded fact, not a vibe: freeze/baseline
// events carry baseline_commit + the sha256 of the frozen body,
// and FrozenFact verifies the live body against that record (one
// hash of bytes it already read — no git call on the read path).
// With no record on file (packages frozen before S1, or a freeze
// whose commit has not happened yet) the fact falls back to a
// live git probe; `beacon truth baseline` persists the record.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// Branch check outcomes (FrozenFact / TruthPackageView surfaces).
const (
	BranchCheckOK       = "ok"
	BranchCheckMismatch = "mismatch"
	BranchCheckNotGit   = "not_a_git_repo"
)

// Baseline statuses (AC-139): the commit state of a frozen body.
const (
	BaselineCommitted   = "committed"
	BaselineUncommitted = "uncommitted"
	BaselineNotGit      = "not_a_git_repo"
	BaselineNoDocument  = "no_document"
)

// EventTruthBaselineRecorded is appended by `truth baseline` (and
// by freeze's auto-record) when a frozen body is verified as
// committed on the canonical branch.
const EventTruthBaselineRecorded = "truth_baseline_recorded"

// ProjectConfigRelPath is the project-local config file (under
// the project's .beacon/ runtime directory) that may carry the
// truth_canonical_branch override key.
const ProjectConfigRelPath = ".beacon/config"

// BranchInfo is the branch-discipline fact for one project root.
type BranchInfo struct {
	IsGit           bool   `json:"is_git"`
	BranchCheck     string `json:"branch_check"` // ok | mismatch | not_a_git_repo
	CurrentBranch   string `json:"current_branch,omitempty"`
	CanonicalBranch string `json:"canonical_branch,omitempty"`
}

// BaselineInfo is the AC-139 commit state of one frozen package.
type BaselineInfo struct {
	Status      string `json:"status"` // committed | uncommitted | not_a_git_repo | no_document
	Commit      string `json:"commit,omitempty"`
	TruthSHA256 string `json:"truth_sha256,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

func sha256HexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// canonicalBranchOverride reads the truth_canonical_branch key
// from the project root's .beacon/config (minimal line format:
// `truth_canonical_branch = dev`, `#` comments allowed). "" when
// no override is declared.
func canonicalBranchOverride(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, ProjectConfigRelPath))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			k, v, found = strings.Cut(line, ":")
		}
		if found && strings.TrimSpace(k) == "truth_canonical_branch" {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// resolveCanonicalBranch: project override -> main -> master ->
// main (the frozen default chain of AC-136/R10).
func resolveCanonicalBranch(root string) string {
	if v := canonicalBranchOverride(root); v != "" {
		return v
	}
	gctx := context.Background()
	if harness.BranchExists(gctx, root, "main") {
		return "main"
	}
	if harness.BranchExists(gctx, root, "master") {
		return "master"
	}
	return "main"
}

// branchInfo assembles the BranchInfo for a project root. A
// detached HEAD reports current_branch=HEAD, which never equals
// the canonical branch (fail-closed mismatch, as in v1).
func (e *Engine) branchInfo(root string) BranchInfo {
	gctx := context.Background()
	if !harness.IsGitRepo(gctx, root) {
		return BranchInfo{BranchCheck: BranchCheckNotGit}
	}
	canonical := resolveCanonicalBranch(root)
	current, err := harness.CurrentBranch(gctx, root)
	if err != nil {
		current = ""
	}
	check := BranchCheckMismatch
	if current != "" && current == canonical {
		check = BranchCheckOK
	}
	return BranchInfo{IsGit: true, BranchCheck: check, CurrentBranch: current, CanonicalBranch: canonical}
}

// requireCanonicalBranch is the AC-136 write guard shared by
// every truth write action. Non-git projects pass (discipline
// not applicable) with the not_a_git_repo fact on the returned
// info; git projects on any other branch — including a detached
// HEAD — are refused with the frozen reason code.
func (e *Engine) requireCanonicalBranch(root, op string) (BranchInfo, error) {
	bi := e.branchInfo(root)
	if !bi.IsGit {
		return bi, nil
	}
	if bi.BranchCheck != BranchCheckOK {
		return bi, core.NewError(core.ReasonTruthCanonicalBranchMismatch, op,
			"truth writes are canonical-only (AC-136): current branch "+bi.CurrentBranch+
				" != truth_canonical_branch "+bi.CanonicalBranch+
				"; run `git checkout "+bi.CanonicalBranch+"` first")
	}
	return bi, nil
}

// ---- trail records (one scan, three readers) ----

type freezeRecord struct {
	Fingerprint             string // package_fingerprint (truth+tests, P4c)
	SurfaceFingerprint      string // surface_fingerprint (four surfaces, S1)
	RevisionID              string
	TruthSHA256After        string            // frozen body hash (R1 drift base)
	RequirementFingerprints map[string]string // per-AC hashes (R1 drift)
}

type baselineRecord struct {
	Commit      string
	TruthSHA256 string
}

type changeOpenRecord struct {
	ChangeID          string
	FingerprintAtOpen string // package_fingerprint when the change was opened ("" = pre-S1 change)
	BaseTruthSHA256   string // frozen body hash the change was authored against (R1)
}

type trailRecords struct {
	baseline       *baselineRecord // nil = no live baseline record
	freeze         *freezeRecord   // latest freeze/refreeze record
	pendingChanges []changeOpenRecord
}

// trailRecords scans the feature trail once and derives: the
// live baseline record (a freeze/refreeze WITHOUT a committed
// baseline clears any earlier record — the body it names is no
// longer the frozen body), the latest freeze record (AC-137's
// proof base) and the pending changes with their open-state
// fingerprints.
func stringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func (e *Engine) trailRecords(featureID string) (trailRecords, error) {
	events, err := e.store.ListEvents(featureID)
	if err != nil {
		return trailRecords{}, err
	}
	var recs trailRecords
	for _, ev := range events {
		var p map[string]any
		if len(ev.Payload) > 0 {
			_ = json.Unmarshal(ev.Payload, &p)
		}
		str := func(k string) string {
			if p == nil {
				return ""
			}
			if s, ok := p[k].(string); ok {
				return s
			}
			return ""
		}
		switch ev.EventType {
		case EventTruthChangeRequested, "change_requested", "gen_change":
			recs.pendingChanges = append(recs.pendingChanges, changeOpenRecord{
				ChangeID: str("change_id"), FingerprintAtOpen: str("package_fingerprint"),
				BaseTruthSHA256: str("base_truth_sha256"),
			})
		case EventTruthFrozen, EventTruthRefrozen, "refreeze", "gen_refreeze":
			recs.pendingChanges = nil
			recs.freeze = &freezeRecord{
				Fingerprint:             str("package_fingerprint"),
				SurfaceFingerprint:      str("surface_fingerprint"),
				RevisionID:              str("revision_id"),
				TruthSHA256After:        str("truth_sha256_after"),
				RequirementFingerprints: stringMap(p["requirement_fingerprints"]),
			}
			if c := str("baseline_commit"); c != "" {
				recs.baseline = &baselineRecord{Commit: c, TruthSHA256: str("truth_sha256_after")}
			} else {
				recs.baseline = nil
			}
		case EventTruthChangeRebased:
			for i := range recs.pendingChanges {
				if recs.pendingChanges[i].ChangeID != str("change_id") {
					continue
				}
				if fp := str("package_fingerprint"); fp != "" {
					recs.pendingChanges[i].FingerprintAtOpen = fp
				}
				if bs := str("base_truth_sha256"); bs != "" {
					recs.pendingChanges[i].BaseTruthSHA256 = bs
				}
			}
		case EventTruthBaselineRecorded:
			recs.baseline = &baselineRecord{Commit: str("baseline_commit"), TruthSHA256: str("truth_sha256")}
		}
	}
	return recs, nil
}

// probeBaseline verifies the frozen body's commit state live
// against git: the truth file must be clean in the working tree,
// its canonical-branch version must be the frozen document, and
// the committed bytes must equal the working-tree bytes. The
// named baseline commit is the newest commit on the canonical
// branch that touched the file.
func (e *Engine) probeBaseline(ctx store.FeatureContext) BaselineInfo {
	root := ctx.Project.RootPath
	truthPath := ctx.Feature.TruthPath
	raw, err := os.ReadFile(resolvePkgPath(root, truthPath))
	if err != nil {
		return BaselineInfo{Status: BaselineNoDocument}
	}
	sum := sha256HexOf(raw)
	gctx := context.Background()
	if !harness.IsGitRepo(gctx, root) {
		return BaselineInfo{Status: BaselineNotGit, TruthSHA256: sum}
	}
	uncommitted := func(detail string) BaselineInfo {
		return BaselineInfo{Status: BaselineUncommitted, TruthSHA256: sum, Detail: detail}
	}
	canonical := resolveCanonicalBranch(root)
	porcelain, err := harness.StatusPorcelain(gctx, root, filepath.ToSlash(truthPath))
	if err != nil {
		return uncommitted("git status failed for the truth file: " + err.Error())
	}
	if strings.TrimSpace(porcelain) != "" {
		return uncommitted("truth file has uncommitted changes")
	}
	prefix, err := harness.ShowPrefix(gctx, root)
	if err != nil {
		return uncommitted("git rev-parse --show-prefix failed: " + err.Error())
	}
	relTop := path.Join(prefix, filepath.ToSlash(truthPath))
	committed, err := harness.ShowFile(gctx, root, canonical, relTop)
	if err != nil {
		return uncommitted("truth file is not committed on " + canonical)
	}
	if status, parsed := frontmatterStatusOf(gate.PackageFile{Present: true, Text: string(committed)}); !parsed || status != "frozen" {
		return uncommitted("the " + canonical + " version of truth.md is not status: frozen")
	}
	if sha256HexOf(committed) != sum {
		return uncommitted("the committed truth on " + canonical + " differs from the working-tree body")
	}
	commit, err := harness.LogFileCommit(gctx, root, canonical, relTop)
	if err != nil || commit == "" {
		return uncommitted("no commit on " + canonical + " touches truth.md")
	}
	return BaselineInfo{Status: BaselineCommitted, Commit: commit, TruthSHA256: sum}
}

// factBaseline is FrozenFact's read path for the baseline
// dimension: a recorded baseline is verified against the live
// body hash (no git call); with no record on file it falls back
// to the live probe.
func (e *Engine) factBaseline(ctx store.FeatureContext, truth gate.PackageFile) BaselineInfo {
	if !truth.Present {
		return BaselineInfo{Status: BaselineNoDocument}
	}
	root := ctx.Project.RootPath
	if !harness.IsGitRepo(context.Background(), root) {
		return BaselineInfo{Status: BaselineNotGit, TruthSHA256: sha256HexOf([]byte(truth.Text))}
	}
	if recs, err := e.trailRecords(ctx.Feature.ID); err == nil && recs.baseline != nil {
		sum := sha256HexOf([]byte(truth.Text))
		if sum == recs.baseline.TruthSHA256 {
			return BaselineInfo{Status: BaselineCommitted, Commit: recs.baseline.Commit, TruthSHA256: sum}
		}
		return BaselineInfo{Status: BaselineUncommitted, Commit: recs.baseline.Commit, TruthSHA256: sum,
			Detail: "truth body moved after baseline " + recs.baseline.Commit +
				" was recorded; commit the frozen body and re-run `beacon truth baseline`"}
	}
	return e.probeBaseline(ctx)
}

// BaselineTruth is the `truth baseline` action (AC-139): verify
// the frozen body's committed state live and persist the
// baseline record (baseline_commit + body sha) on the trail.
// Idempotent — an identical live record is not re-appended.
// Freeze attempts the same recording in passing when the body
// is already committed; this command is the explicit补记 path
// after the author commits.
func (e *Engine) BaselineTruth(ctx store.FeatureContext, by string) (TruthPackageView, error) {
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.Baseline"); err != nil {
		return TruthPackageView{}, err
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if !view.Frozen {
		return view, core.NewError(ReasonTruthNotFrozen, "truth.Baseline",
			"package "+view.Slug+" is not frozen; a baseline exists only for frozen truth (AC-139)")
	}
	bl := e.probeBaseline(ctx)
	view.BaselineStatus, view.BaselineCommit = bl.Status, bl.Commit
	switch bl.Status {
	case BaselineNotGit:
		view.BaselineNote = "not a git repository: no commit baseline applies; the frontmatter is the authority (P4b caliber)"
		return view, nil
	case BaselineCommitted:
		recs, err := e.trailRecords(ctx.Feature.ID)
		if err != nil {
			return TruthPackageView{}, err
		}
		if recs.baseline != nil && recs.baseline.Commit == bl.Commit && recs.baseline.TruthSHA256 == bl.TruthSHA256 {
			view.BaselineRecorded, view.BaselineAlready = true, true
			return view, nil
		}
		if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthBaselineRecorded, map[string]any{
			"baseline_commit": bl.Commit, "truth_sha256": bl.TruthSHA256,
			"canonical_branch": view.CanonicalBranch, "by": by,
		}); err != nil {
			return TruthPackageView{}, err
		}
		view.BaselineRecorded = true
		return view, nil
	default:
		return view, core.NewError(core.ReasonCanonicalRefreezeNotCommitted, "truth.Baseline",
			"frozen truth is not a committed baseline yet: "+bl.Detail+
				"; commit the package on "+view.CanonicalBranch+", then re-run `beacon truth baseline` (AC-139)")
	}
}
