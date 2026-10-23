// Spine support derivations: project root canonicalization and
// project_key (v1 runtime_projection._project_key parity), artifact
// staleness views (AC-010/015), and the phase/next/progress field
// derivations over lifecycle + run metadata (sources documented in
// docs/p1-implementation.md).
package store

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
)

// ResolveRootPath canonicalizes a project root the way Python's
// Path.expanduser().resolve() does in v1 _project_key: expand ~, make
// absolute, resolve symlinks when the path exists, clean the result.
func ResolveRootPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	// The leaf may not exist yet (a package/project directory is
	// often registered before it is created). Resolve the longest
	// existing ancestor and rejoin the remaining components, so the
	// same path always canonicalizes to the same string whether or
	// not the directory exists — Python's Path.resolve(strict=False)
	// behavior.
	dir, tail := abs, []string{}
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return filepath.Clean(resolved)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Clean(abs)
		}
		tail = append(tail, filepath.Base(dir))
		dir = parent
	}
}

// ProjectKey reproduces v1 runtime_projection._project_key exactly:
// "<basename>--<first 10 hex of SHA1(resolved root path)>" where the
// hashed string is the resolved absolute path itself.
func ProjectKey(rootPath string) string {
	resolved := ResolveRootPath(rootPath)
	sum := sha1.Sum([]byte(resolved))
	return filepath.Base(resolved) + "--" + hex.EncodeToString(sum[:])[:10]
}

// ArtifactView is an artifact_refs row plus its live staleness verdict
// (AC-010): stale when the blob is missing/unreadable or its actual
// SHA-256 differs from the stored content_hash.
type ArtifactView struct {
	ArtifactRef
	Stale       bool   `json:"stale"`
	StaleReason string `json:"stale_reason,omitempty"` // file_missing | file_unreadable | hash_mismatch
}

// ArtifactStaleness hashes the blob at a.AbsPath and reports staleness.
// It never returns an error: any file problem is a stale flag (AC-015).
func ArtifactStaleness(a ArtifactRef) (stale bool, reason string) {
	f, err := os.Open(a.AbsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, "file_missing"
		}
		return true, "file_unreadable"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return true, "file_unreadable"
	}
	if IsStale(a, hex.EncodeToString(h.Sum(nil))) {
		return true, "hash_mismatch"
	}
	return false, ""
}

// ViewOf wraps a stored pointer with its live staleness verdict.
func ViewOf(a ArtifactRef) ArtifactView {
	stale, reason := ArtifactStaleness(a)
	return ArtifactView{ArtifactRef: a, Stale: stale, StaleReason: reason}
}

func isTerminalRun(s core.RunStatus) bool {
	switch s {
	case core.RunSucceeded, core.RunFailed, core.RunClosed, core.RunStopped:
		return true
	}
	return false
}

// stagePhase maps a lifecycle stage to the v1.6 spine phase vocabulary
// (truth/generate/qa/release/repair, + design/archive extensions).
func stagePhase(s core.WorkflowStage) string {
	switch s {
	case core.StageThink, core.StageUserStory, core.StagePrd, core.StageTestCase, core.StageFreeze:
		return "truth"
	case core.StagePrototype:
		return "design"
	case core.StageImplement:
		return "generate"
	case core.StageQa:
		return "qa"
	case core.StageRelease:
		return "release"
	case core.StageArchive:
		return "archive"
	}
	return ""
}

// stageHarness maps a stage to the harness that owns the next step.
func stageHarness(s core.WorkflowStage) string {
	switch stagePhase(s) {
	case "truth":
		return "truth"
	case "design":
		return "design"
	case "generate":
		return "implement"
	case "qa":
		return "qa"
	case "release":
		return "release"
	}
	return ""
}

func metaString(meta map[string]any, key string) (string, bool) {
	if meta == nil {
		return "", false
	}
	if v, ok := meta[key].(string); ok && v != "" {
		return v, true
	}
	return "", false
}

// deriveSpineFields fills phase/next_*/progress fields. Source order:
// run metadata_json (orchestrator-written, P2) wins; otherwise the
// documented derivation over lifecycle + latest run + latest gates in
// docs/p1-implementation.md applies. Nothing is invented silently.
func deriveSpineFields(snap *SpineSnapshot, meta map[string]any) {
	if v, ok := metaString(meta, "phase"); ok {
		snap.Phase = v
	} else if snap.Status == core.StatusRepairing || snap.RunStatus == core.RunRepairing {
		snap.Phase = "repair"
	} else {
		snap.Phase = stagePhase(snap.Stage)
	}
	if meta != nil {
		if raw, ok := meta["phases_completed"].([]any); ok {
			done := []string{}
			for _, item := range raw {
				if s, ok := item.(string); ok {
					done = append(done, s)
				}
			}
			snap.PhasesCompleted = done
		} else {
			snap.PhasesCompleted = derivedPhasesCompleted(snap.Stage)
		}
		if v, ok := meta["phase_promoted"].(bool); ok {
			snap.PhasePromoted = v
		} else {
			snap.PhasePromoted = derivedPhasePromoted(snap)
		}
	} else {
		snap.PhasesCompleted = derivedPhasesCompleted(snap.Stage)
		snap.PhasePromoted = derivedPhasePromoted(snap)
	}
	if v, ok := metaString(meta, "next_harness"); ok {
		snap.NextHarness = v
	}
	if v, ok := metaString(meta, "next_action"); ok {
		snap.NextAction = v
	}
	if snap.NextAction == "" && snap.Status == core.StatusBlocked {
		snap.NextAction = "resolve_harness_blocker"
		snap.NextHarness = stageHarness(snap.Stage)
	}
	if snap.NextAction == "" && snap.Stage != "" {
		if next := nextStage(snap.Stage); next != "" {
			snap.NextHarness = stageHarness(next)
			snap.NextAction = "phase:" + stagePhase(next)
		}
	}
	if v, ok := metaString(meta, "progress_status"); ok {
		snap.ProgressStatus = v
	} else {
		snap.ProgressStatus = derivedProgress(snap)
	}
}

// derivedPhasesCompleted assumes linear pipeline progression: reaching
// stage S implies every earlier stage completed. Phase names are the
// stage->phase mapping, de-duplicated in pipeline order. This is a
// heuristic until the orchestrator (P2) writes phases_completed into
// run metadata; recorded as a gap in the P1 report.
func derivedPhasesCompleted(stage core.WorkflowStage) []string {
	out := []string{}
	if stage == "" {
		return out
	}
	seen := map[string]bool{}
	for _, s := range core.AllWorkflowStages() {
		if s == stage {
			break
		}
		if p := stagePhase(s); p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// derivedPhasePromoted mirrors v1.6 semantics: false when the last
// harness/gate failed (blocked/fail verdict) — the operator must look.
func derivedPhasePromoted(snap *SpineSnapshot) bool {
	if snap.Stage == "" && snap.RunCount == 0 {
		return false
	}
	for _, g := range snap.LatestGates {
		switch strings.ToLower(g.Verdict) {
		case "blocked", "fail", "failed", "no-go", "no_go":
			return false
		}
	}
	return true
}

func derivedProgress(snap *SpineSnapshot) string {
	switch snap.RunStatus {
	case core.RunSucceeded, core.RunClosed:
		return "done"
	case core.RunRunning, core.RunResolving, core.RunCreated, core.RunPaused,
		core.RunRepairing, core.RunAwaitingQa, core.RunImporting:
		return "in_progress"
	case core.RunFailed, core.RunStopped, core.RunStopping:
		return "unknown"
	}
	switch snap.Status {
	case core.StatusCompleted:
		return "done"
	case core.StatusInProgress, core.StatusBlocked, core.StatusRepairing:
		return "in_progress"
	}
	return "unknown"
}

func nextStage(s core.WorkflowStage) core.WorkflowStage {
	stages := core.AllWorkflowStages()
	for i, v := range stages {
		if v == s && i+1 < len(stages) {
			return stages[i+1]
		}
	}
	return ""
}
