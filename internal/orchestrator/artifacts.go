// Artifact registration surface (P6 E2E fix): the missing write path
// that lets a host or a human declare "this file on disk is an
// artifact of this run" and have it land in artifact_refs with a real
// sha256 + size. Before this surface existed, stage executors were
// the only writers (stage_stdout / stage_stderr / stage_context) and
// receive-result only harvested implement delegations, so a QA report
// produced by a real stage agent could never become a registered
// QA-kind pointer — and the formal qa gate, which only trusts
// registered pointers, blocked completion (P6 E2E step 10).
//
// Trust model: registration is a DECLARATION by the registrant named
// in --by; it records the file's hash at registration time and judges
// nothing about quality. The defence line is downstream: every gate
// evaluation re-hashes registered files (store.ArtifactStaleness),
// so a tampered or replaced file turns stale and blocks. Nothing in
// this file reads or influences any gate verdict.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// Reason codes of the registration surface.
const (
	ReasonArtifactKindNotAllowed    = "artifact_kind_not_allowed"
	ReasonArtifactFileMissing       = "artifact_file_missing"
	ReasonArtifactRegistrarRequired = "artifact_registrar_required"
	// ReasonArtifactKindExtensionNotAllowed refuses a surface-evidence
	// kind whose file extension cannot be that evidence (a11y_tree as
	// .txt, browser_trace as .md, e2e_report as .zip).
	ReasonArtifactKindExtensionNotAllowed = "artifact_kind_extension_not_allowed"
)

// EventArtifactRegistered is appended once per NEW artifact_refs row
// written through RegisterArtifact (idempotent re-registration of an
// unchanged file writes neither a row nor an event).
const EventArtifactRegistered = "artifact_registered"

// registerableKinds is the controlled vocabulary for the manual
// registration face. The QA family mirrors qaKind() in gates_facts.go
// exactly (qa / scorecard / test_result / test_results / qa_*), so a
// kind this face accepts as QA evidence is a kind the qa gate counts.
// Kernel capture kinds (stage_stdout, stage_stderr, stage_context)
// are deliberately absent: executors write those themselves, and a
// manual registration must not be able to impersonate a capture.
var registerableKinds = map[string]bool{
	"qa": true, "scorecard": true,
	"test_result": true, "test_results": true,
	"evidence": true, "implement_artifact": true,
	"report": true, "log": true, "artifact": true,
	// qa-surface-e2e: the surface-evidence vocabulary. a11y_tree is a
	// serialized accessibility tree, browser_trace is a Playwright/
	// Chromium trace archive, e2e_report is the runner's verdict.
	"a11y_tree": true, "browser_trace": true, "e2e_report": true,
}

// artifactKindExtensions is the extension whitelist of the three
// surface-evidence kinds. Kinds outside this map accept any extension
// (their meaning is the registrant's declaration, as before).
var artifactKindExtensions = map[string][]string{
	"a11y_tree":     {".json"},
	"browser_trace": {".zip", ".json"},
	"e2e_report":    {".json", ".md"},
}

// ArtifactKindAllowsPath reports whether path's extension is allowed
// for kind. Unconstrained kinds always pass.
func ArtifactKindAllowsPath(kind, path string) bool {
	allowed, ok := artifactKindExtensions[strings.ToLower(strings.TrimSpace(kind))]
	if !ok {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, candidate := range allowed {
		if ext == candidate {
			return true
		}
	}
	return false
}

// ValidRegisterKind reports whether kind may be registered through
// the manual face (exact vocabulary member or any qa_* kind).
func ValidRegisterKind(kind string) bool {
	if registerableKinds[kind] {
		return true
	}
	return strings.HasPrefix(kind, "qa_")
}

// ArtifactRegistration is the outcome of one RegisterArtifact call:
// the stored pointer, whether it already existed unchanged, and —
// when a different row id still has to be referred to (legacy
// duplicates from before the (run_id, abs_path) upsert) — that row's
// id. A plain content refresh keeps the same id and reports nothing
// under Supersedes (AC-002).
type ArtifactRegistration struct {
	Artifact          store.ArtifactRef `json:"artifact"`
	AlreadyRegistered bool              `json:"already_registered"`
	Supersedes        string            `json:"supersedes,omitempty"`
}

// RegisterArtifact registers the file at path as an artifact of the
// run under kind, declared by registrant `by`. The file must exist
// as a regular file; its sha256 and size are computed from disk at
// registration time. Re-registering the same run+kind+path with an
// unchanged hash is idempotent (returns the existing row, writes
// nothing). With a changed hash the stored pointer is refreshed in
// place (AC-001/AC-002, artifact-upsert): the row keeps its id, so
// the qa evidence check sees a live pointer instead of a stale
// leftover, and the event trail records the refresh.
func (e *Engine) RegisterArtifact(runID, kind, path, by string) (ArtifactRegistration, error) {
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	path, err = filepath.Abs(path)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	if kind == "qa_candidate" {
		if _, err := e.validateQACandidate(run, path, by); err != nil {
			return ArtifactRegistration{}, err
		}
	}
	res, payload, err := e.registerArtifactScoped(run.ID, kind, path, by)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	if res.AlreadyRegistered && kind != "qa_candidate" && kind != "qa_report" {
		return res, nil
	}
	if payload == nil {
		payload = map[string]any{"kind": kind, "abs_path": path, "content_hash": res.Artifact.ContentHash, "artifact_id": res.Artifact.ID, "registered_by": by}
	}
	if kind == "qa_report" {
		// Attribute even malformed reviews to the revision being reviewed.
		// An invalid latest review must not expose an older PASS.
		if c, err := e.currentQACandidate(run); err == nil {
			payload["candidate_hash"] = c.Hash
		}
		var r qaProductReview
		_ = qaRead(path, &r)
		fc, ferr := e.featureContext(run.FeatureID)
		if ferr == nil {
			var files []qaFile
			for _, ref := range r.EvidenceRefs {
				abs := resolvePkgPath(fc.Project.RootPath, ref)
				if a, err := e.qaRegisteredFile(run.ID, abs); err == nil {
					files = append(files, qaFile{abs, a.ContentHash})
				}
			}
			payload["evidence_files"] = files
		}
	}
	if err := e.appendEvent(run, EventArtifactRegistered, payload); err != nil {
		return ArtifactRegistration{}, err
	}
	return res, nil
}

// RegisterFeatureArtifact registers a file as evidence of a FEATURE
// instead of a run (AC-002: the freeze -> run -> artifact cycle). The
// pointer lands under the feature-scope sentinel run_id so the
// evidence-reference check accepts it with no run in existence; the
// event is feature-scoped. Run-scoped registration is unchanged.
func (e *Engine) RegisterFeatureArtifact(fc store.FeatureContext, kind, path, by string) (ArtifactRegistration, error) {
	res, payload, err := e.registerArtifactScoped(store.FeatureScopeRunID(fc.Feature.ID), kind, path, by)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	if res.AlreadyRegistered {
		return res, nil
	}
	payload["feature_id"] = fc.Feature.ID
	payload["feature_slug"] = fc.Feature.Slug
	payload["scope"] = "feature"
	if err := e.appendFeatureEvent(fc.Feature.ID, EventArtifactRegistered, payload); err != nil {
		return ArtifactRegistration{}, err
	}
	return res, nil
}

// registerArtifactScoped is the shared write half: validate the
// declaration, hash the file, upsert the pointer under scopeRunID and
// return the event payload the caller's scope writes.
func (e *Engine) registerArtifactScoped(scopeRunID, kind, path, by string) (ArtifactRegistration, map[string]any, error) {
	return e.registerArtifactScopedCaptured(scopeRunID, kind, path, by, false)
}

func (e *Engine) registerArtifactScopedCaptured(scopeRunID, kind, path, by string, captured bool) (ArtifactRegistration, map[string]any, error) {
	if strings.TrimSpace(by) == "" {
		return ArtifactRegistration{}, nil, core.NewError(ReasonArtifactRegistrarRequired,
			"goal.RegisterArtifact", "a registrant identity (--by) is required")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !ValidRegisterKind(kind) && !(captured && kind == "capture_result") {
		return ArtifactRegistration{}, nil, core.NewError(ReasonArtifactKindNotAllowed,
			"goal.RegisterArtifact", "kind "+kind+" is not in the registration vocabulary "+
				"(qa, qa_*, scorecard, test_result, test_results, evidence, implement_artifact, report, log, artifact, a11y_tree, browser_trace, e2e_report)")
	}
	if !ArtifactKindAllowsPath(kind, path) {
		return ArtifactRegistration{}, nil, core.NewError(ReasonArtifactKindExtensionNotAllowed,
			"goal.RegisterArtifact", "kind "+kind+" does not accept extension "+filepath.Ext(path)+
				" (allowed: "+strings.Join(artifactKindExtensions[kind], ", ")+")")
	}
	abs := path
	if !filepath.IsAbs(abs) {
		var err error
		abs, err = filepath.Abs(path)
		if err != nil {
			return ArtifactRegistration{}, nil, err
		}
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return ArtifactRegistration{}, nil, core.NewError(ReasonArtifactFileMissing,
			"goal.RegisterArtifact", "artifact file does not exist or is not a regular file: "+abs)
	}
	f, err := os.Open(abs)
	if err != nil {
		return ArtifactRegistration{}, nil, core.NewError(ReasonArtifactFileMissing,
			"goal.RegisterArtifact", "artifact file unreadable: "+abs)
	}
	h := sha256.New()
	size, err := io.Copy(h, f)
	f.Close()
	if err != nil {
		return ArtifactRegistration{}, nil, err
	}
	sum := hex.EncodeToString(h.Sum(nil))

	existing, err := e.store.Artifacts().ListByRun(scopeRunID)
	if err != nil {
		return ArtifactRegistration{}, nil, err
	}
	var previous, previousHash string
	for _, a := range existing {
		original := a.AbsPath
		candidate := abs
		if real, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = real
		}
		if real, err := filepath.EvalSymlinks(original); err == nil {
			original = real
		}
		if original == candidate && a.Kind == "capture_result" {
			return ArtifactRegistration{}, nil, core.NewError("artifact_capture_immutable", "artifact.register", "captured results cannot be re-registered or refreshed")
		}
		if a.Kind != kind || a.AbsPath != abs {
			continue
		}
		if a.ContentHash == sum {
			return ArtifactRegistration{Artifact: a, AlreadyRegistered: true}, nil, nil
		}
		previous = a.ID // previous row for this scope+kind+path
		previousHash = a.ContentHash
	}
	ref, err := e.store.Artifacts().Add(store.ArtifactRef{
		RunID: scopeRunID, Kind: kind, AbsPath: abs,
		ContentHash: sum, SizeBytes: size,
	})
	if err != nil {
		return ArtifactRegistration{}, nil, err
	}
	payload := map[string]any{
		"artifact_id": ref.ID, "kind": kind, "abs_path": abs,
		"content_hash": sum, "size_bytes": size, "registered_by": by,
	}
	// Add upserts by (run_id, abs_path), so a changed file refreshes
	// the previous row instead of leaving it behind as a stale
	// pointer. Supersedes is only set if a different row id survives
	// (legacy data); the refresh itself is flagged explicitly.
	supersedes := ""
	if previous != "" {
		payload["refreshed"] = true
		payload["previous_content_hash"] = previousHash
		if previous != ref.ID {
			supersedes = previous
			payload["supersedes"] = previous
		}
	}
	return ArtifactRegistration{Artifact: ref, Supersedes: supersedes}, payload, nil
}

// RegisterScopedArtifact registers evidence under an explicit
// feature/node scope without requiring a run. Feature callers keep the
// existing RegisterFeatureArtifact wrapper; node evidence uses this.
func (e *Engine) RegisterScopedArtifact(scopeKind, scopeID, kind, path, by string) (ArtifactRegistration, error) {
	k, id, err := store.NormalizeArtifactScope(scopeKind, scopeID)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	res, payload, err := e.registerArtifactScoped(store.ScopeRunID(k, id), kind, path, by)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	if res.AlreadyRegistered {
		return res, nil
	}
	if payload != nil {
		payload["scope"] = k
		payload["scope_id"] = id
	}
	if k == "feature" {
		payload["feature_id"] = id
		if err := e.appendFeatureEvent(id, EventArtifactRegistered, payload); err != nil {
			return ArtifactRegistration{}, err
		}
	}
	return res, nil
}

// ListScopedArtifacts lists evidence for one explicit scope.
func (e *Engine) ListScopedArtifacts(scopeKind, scopeID string) ([]store.ArtifactView, error) {
	arts, err := e.store.Artifacts().ListByScope(scopeKind, scopeID)
	if err != nil {
		return nil, err
	}
	views := []store.ArtifactView{}
	for _, a := range arts {
		views = append(views, store.ViewOf(a))
	}
	return views, nil
}

// ListRunArtifacts returns the run's registered pointers with live
// staleness verdicts — the same presentation spine artifacts uses
// (store.ViewOf), shared by `goal artifacts list`.
func (e *Engine) ListRunArtifacts(runID string) ([]store.ArtifactView, error) {
	if _, _, _, err := e.loadRun(runID); err != nil {
		return nil, err
	}
	arts, err := e.store.Artifacts().ListByRun(runID)
	if err != nil {
		return nil, err
	}
	views := []store.ArtifactView{}
	for _, a := range arts {
		views = append(views, store.ViewOf(a))
	}
	return views, nil
}

// ListFeatureArtifacts returns the feature-scoped pointers of a feature
// with live staleness verdicts (the `goal artifacts list --feature`
// read side).
func (e *Engine) ListFeatureArtifacts(fc store.FeatureContext) ([]store.ArtifactView, error) {
	arts, err := e.store.Artifacts().ListByFeatureScope(fc.Feature.ID)
	if err != nil {
		return nil, err
	}
	views := []store.ArtifactView{}
	for _, a := range arts {
		views = append(views, store.ViewOf(a))
	}
	return views, nil
}
