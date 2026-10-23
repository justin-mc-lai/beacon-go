// State portability (R2c, parity audit H10): project state that
// travels WITH the repository. Python kept its records in the repo
// (.machine/ + beacon/version-truth.json), so a clone carried the
// project's progress with it. The Go kernel's single source of
// truth is the machine-global DB — that rule does NOT change. What
// changes is that a project can EXPORT a readable snapshot of its
// state into .beacon/state/project-state.json (a directory Python's
// own init already used for project state), commit it, and IMPORT
// it on another machine to rebuild the registry view.
//
// Conflict rule (one rule, no negotiation): the DB wins. Import
// only FILLS rows that are missing locally — an existing project,
// version, feature, lifecycle snapshot or gate verdict is never
// overwritten by a snapshot. Runs are exported as summaries for
// humans; they are NOT imported (a run's metadata is live engine
// state, and resurrecting it from a file would fabricate history).
// Gate verdicts and lifecycle snapshots ARE imported, because they
// are conclusions, not live state — that is what lets `spine show`
// on the new machine display the project's real progress.
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// StateSnapshotRelPath is the in-repo location of the portable
// state snapshot.
const StateSnapshotRelPath = ".beacon/state/project-state.json"

// StateSnapshotSchema is the snapshot document's schema marker.
const StateSnapshotSchema = "beacon-state-snapshot.v1"

// EventStateExported / EventStateImported are recorded per feature
// touched when a snapshot is written into / restored from the DB
// trail. Export itself is a file write (recorded in the document).
const (
	EventStateExported = "state_snapshot_exported"
	EventStateImported = "state_snapshot_imported"
)

// StateGate is one feature's latest verdict for one gate type.
type StateGate struct {
	GateType    string   `json:"gate_type"`
	Verdict     string   `json:"verdict"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	EvaluatedAt string   `json:"evaluated_at"`
}

// StateRunSummary is the human-facing summary of one run. Export
// only; never imported.
type StateRunSummary struct {
	ID        string `json:"id"`
	RunKind   string `json:"run_kind"`
	Status    string `json:"status"`
	Iteration int    `json:"iteration"`
	LoopStage string `json:"loop_stage,omitempty"`
	Blocked   bool   `json:"blocked,omitempty"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at,omitempty"`
}

// StateFeature is one feature's portable state.
type StateFeature struct {
	Slug        string            `json:"slug"`
	Version     string            `json:"version"`
	TruthPath   string            `json:"truth_path"`
	TestsPath   string            `json:"tests_path,omitempty"`
	Status      string            `json:"status"`
	Stage       string            `json:"lifecycle_stage,omitempty"`
	StageStatus string            `json:"lifecycle_status,omitempty"`
	ReasonCodes []string          `json:"reason_codes,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
	LatestGates []StateGate       `json:"latest_gates,omitempty"`
	Runs        []StateRunSummary `json:"runs,omitempty"`
}

// StateVersion is one docs version's portable record.
type StateVersion struct {
	Version   string `json:"version"`
	DocsRoot  string `json:"docs_root"`
	IsCurrent bool   `json:"is_current"`
}

// StateSnapshot is the portable project-state document.
type StateSnapshot struct {
	SchemaVersion string         `json:"schema_version"`
	ExportedAt    string         `json:"exported_at"`
	ProjectName   string         `json:"project_name"`
	RecordedRoot  string         `json:"recorded_root_path"`
	ProjectKey    string         `json:"project_key"`
	Versions      []StateVersion `json:"versions"`
	Features      []StateFeature `json:"features"`
}

// StateImportResult counts what one import filled in vs skipped.
type StateImportResult struct {
	ProjectCreated     bool `json:"project_created"`
	VersionsAdded      int  `json:"versions_added"`
	FeaturesAdded      int  `json:"features_added"`
	LifecyclesRestored int  `json:"lifecycles_restored"`
	GatesRestored      int  `json:"gates_restored"`
	SkippedExisting    int  `json:"skipped_existing"`
	RunsSummarizedOnly int  `json:"runs_summarized_only"` // exported runs are never imported
}

// HasStateSnapshot reports whether the project root carries a
// portable snapshot (the auto-recognition signal).
func HasStateSnapshot(root string) bool {
	info, err := os.Stat(filepath.Join(root, StateSnapshotRelPath))
	return err == nil && !info.IsDir()
}

// LoadStateSnapshot reads and validates a project's snapshot file.
func LoadStateSnapshot(root string) (StateSnapshot, error) {
	raw, err := os.ReadFile(filepath.Join(root, StateSnapshotRelPath))
	if err != nil {
		return StateSnapshot{}, core.NewError(core.ReasonStateSnapshotMissing,
			"state.LoadSnapshot", "no state snapshot at "+StateSnapshotRelPath+" under "+root)
	}
	var snap StateSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return StateSnapshot{}, core.NewError(core.ReasonStateSnapshotInvalid,
			"state.LoadSnapshot", "snapshot is not valid JSON: "+err.Error())
	}
	if snap.SchemaVersion != StateSnapshotSchema {
		return StateSnapshot{}, core.NewError(core.ReasonStateSnapshotInvalid,
			"state.LoadSnapshot", "unsupported snapshot schema "+snap.SchemaVersion+" (want "+StateSnapshotSchema+")")
	}
	return snap, nil
}

// ExportProjectState builds the snapshot for a registered project
// and writes it into the repo at StateSnapshotRelPath.
func (e *Engine) ExportProjectState(projectRoot string) (StateSnapshot, string, error) {
	root := store.ResolveRootPath(projectRoot)
	proj, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StateSnapshot{}, "", core.NewError(ReasonProjectNotRegistered,
				"state.Export", "project not registered: "+root)
		}
		return StateSnapshot{}, "", err
	}
	snap := StateSnapshot{
		SchemaVersion: StateSnapshotSchema,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		ProjectName:   proj.Name, RecordedRoot: proj.RootPath,
		ProjectKey: store.ProjectKey(proj.RootPath),
	}
	versions, err := e.store.Projects().ListVersions(proj.ID)
	if err != nil {
		return StateSnapshot{}, "", err
	}
	for _, v := range versions {
		snap.Versions = append(snap.Versions, StateVersion{
			Version: v.Version, DocsRoot: v.DocsRoot, IsCurrent: v.IsCurrent,
		})
	}
	ctxs, err := e.store.ListFeatureContexts(root)
	if err != nil {
		return StateSnapshot{}, "", err
	}
	for _, ctx := range ctxs {
		sf := StateFeature{
			Slug: ctx.Feature.Slug, Version: ctx.Version.Version,
			TruthPath: ctx.Feature.TruthPath, TestsPath: ctx.Feature.TestsPath,
			Status: ctx.Feature.Status,
		}
		if lc, lerr := e.store.Features().GetLifecycle(ctx.Feature.ID); lerr == nil {
			sf.Stage = string(lc.Stage)
			sf.StageStatus = string(lc.Status)
			sf.ReasonCodes = lc.ReasonCodes
			sf.UpdatedAt = lc.UpdatedAt
		}
		gates, gerr := e.store.Gates().LatestByType(ctx.Feature.ID)
		if gerr != nil {
			return StateSnapshot{}, "", gerr
		}
		for _, g := range gates {
			sf.LatestGates = append(sf.LatestGates, StateGate{
				GateType: g.GateType, Verdict: g.Verdict,
				ReasonCodes: g.ReasonCodes, EvaluatedAt: g.EvaluatedAt,
			})
		}
		runs, rerr := e.store.Runs().ListByFeature(ctx.Feature.ID)
		if rerr != nil {
			return StateSnapshot{}, "", rerr
		}
		for _, r := range runs {
			sum := StateRunSummary{
				ID: r.ID, RunKind: r.RunKind, Status: string(r.Status),
				Iteration: r.Iteration, StartedAt: r.StartedAt,
			}
			if r.EndedAt.Valid {
				sum.EndedAt = r.EndedAt.String
			}
			var m runMeta
			if len(r.Metadata) > 0 && json.Unmarshal(r.Metadata, &m) == nil {
				sum.LoopStage = m.LoopStage
				sum.Blocked = m.Blocked
			}
			sf.Runs = append(sf.Runs, sum)
		}
		snap.Features = append(snap.Features, sf)
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return StateSnapshot{}, "", err
	}
	abs := filepath.Join(root, StateSnapshotRelPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return StateSnapshot{}, "", err
	}
	if err := os.WriteFile(abs, append(raw, '\n'), 0o644); err != nil {
		return StateSnapshot{}, "", err
	}
	for _, ctx := range ctxs {
		_ = e.appendFeatureEvent(ctx.Feature.ID, EventStateExported, map[string]any{
			"snapshot": StateSnapshotRelPath, "exported_at": snap.ExportedAt,
		})
	}
	return snap, abs, nil
}

// ImportProjectState restores a project's registry view from the
// snapshot in its repo, filling ONLY what the local DB lacks (the
// DB wins every conflict). The project binds to the directory the
// snapshot was found in — a clone at a new path is a new root, and
// the snapshot's recorded root is informational only.
func (e *Engine) ImportProjectState(projectRoot string) (StateImportResult, error) {
	root := store.ResolveRootPath(projectRoot)
	snap, err := LoadStateSnapshot(root)
	if err != nil {
		return StateImportResult{}, err
	}
	res := StateImportResult{}
	proj, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return StateImportResult{}, err
		}
		name := snap.ProjectName
		if name == "" {
			name = filepath.Base(root)
		}
		proj, err = e.store.Projects().Register(store.Project{RootPath: root, Name: name})
		if err != nil {
			return StateImportResult{}, err
		}
		res.ProjectCreated = true
	}
	// Versions: add the missing, then apply the snapshot's current
	// marker only when the project has no current version yet (an
	// existing current marker is DB state and wins).
	verByName := map[string]store.ProjectVersion{}
	refreshVersions := func() error {
		list, lerr := e.store.Projects().ListVersions(proj.ID)
		if lerr != nil {
			return lerr
		}
		verByName = map[string]store.ProjectVersion{}
		for _, v := range list {
			verByName[v.Version] = v
		}
		return nil
	}
	if err := refreshVersions(); err != nil {
		return StateImportResult{}, err
	}
	for _, sv := range snap.Versions {
		if _, ok := verByName[sv.Version]; ok {
			res.SkippedExisting++
			continue
		}
		if _, err := e.store.Projects().AddVersion(store.ProjectVersion{
			ProjectID: proj.ID, Version: sv.Version, DocsRoot: sv.DocsRoot,
		}); err != nil {
			return StateImportResult{}, err
		}
		res.VersionsAdded++
	}
	if err := refreshVersions(); err != nil {
		return StateImportResult{}, err
	}
	if _, err := e.store.Projects().CurrentVersion(proj.ID); err != nil {
		for _, sv := range snap.Versions {
			if sv.IsCurrent {
				if v, ok := verByName[sv.Version]; ok {
					if err := e.store.Projects().SetCurrent(v.ID); err != nil {
						return StateImportResult{}, err
					}
				}
				break
			}
		}
	}
	for _, sf := range snap.Features {
		ver, ok := verByName[sf.Version]
		if !ok {
			continue
		}
		feat, ferr := e.store.Features().GetBySlug(ver.ID, sf.Slug)
		if ferr != nil {
			if !errors.Is(ferr, sql.ErrNoRows) {
				return StateImportResult{}, ferr
			}
			status := sf.Status
			if status == "" {
				status = "draft"
			}
			feat, ferr = e.store.Features().CreatePackage(store.FeaturePackage{
				ProjectVersionID: ver.ID, Slug: sf.Slug,
				TruthPath: sf.TruthPath, TestsPath: sf.TestsPath, Status: status,
			})
			if ferr != nil {
				return StateImportResult{}, ferr
			}
			res.FeaturesAdded++
		}
		if sf.Stage != "" {
			if _, lerr := e.store.Features().GetLifecycle(feat.ID); errors.Is(lerr, sql.ErrNoRows) {
				if err := e.store.Features().UpdateLifecycle(store.Lifecycle{
					FeatureID: feat.ID, Stage: core.WorkflowStage(sf.Stage),
					Status: core.StageStatus(sf.StageStatus), ReasonCodes: sf.ReasonCodes,
				}); err != nil {
					return StateImportResult{}, err
				}
				res.LifecyclesRestored++
			} else if lerr == nil {
				res.SkippedExisting++
			} else {
				return StateImportResult{}, lerr
			}
		}
		for _, sg := range sf.LatestGates {
			if _, gerr := e.store.Gates().Latest(feat.ID, sg.GateType); gerr == nil {
				res.SkippedExisting++
				continue
			} else if !errors.Is(gerr, sql.ErrNoRows) {
				return StateImportResult{}, gerr
			}
			if _, err := e.store.Gates().Record(store.GateResult{
				FeatureID: feat.ID, GateType: sg.GateType, Verdict: sg.Verdict,
				ReasonCodes: sg.ReasonCodes, EvaluatedAt: sg.EvaluatedAt,
			}); err != nil {
				return StateImportResult{}, err
			}
			res.GatesRestored++
		}
		res.RunsSummarizedOnly += len(sf.Runs)
		_ = e.appendFeatureEvent(feat.ID, EventStateImported, map[string]any{
			"snapshot": StateSnapshotRelPath, "exported_at": snap.ExportedAt,
		})
	}
	return res, nil
}

// AutoImportState is the recognition path for commands: when a
// project root is not registered but carries a snapshot, import it
// (auto-register from the repo) and report true. Any failure leaves
// the caller to produce its normal not-registered error.
func (e *Engine) AutoImportState(projectRoot string) bool {
	root := store.ResolveRootPath(projectRoot)
	if _, err := e.store.Projects().GetByRootPath(root); err == nil {
		return false // already registered; nothing to recognize
	}
	if !HasStateSnapshot(root) {
		return false
	}
	_, err := e.ImportProjectState(root)
	return err == nil
}
