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
// when the same run+kind+path was previously registered with a
// different hash — the superseded row's id.
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
// nothing). The same run+kind+path with a changed hash appends a new
// row and reports the previous row as superseded; old rows are never
// mutated, so gate staleness checks keep seeing the full history.
func (e *Engine) RegisterArtifact(runID, kind, path, by string) (ArtifactRegistration, error) {
	if strings.TrimSpace(by) == "" {
		return ArtifactRegistration{}, core.NewError(ReasonArtifactRegistrarRequired,
			"goal.RegisterArtifact", "a registrant identity (--by) is required")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !ValidRegisterKind(kind) {
		return ArtifactRegistration{}, core.NewError(ReasonArtifactKindNotAllowed,
			"goal.RegisterArtifact", "kind "+kind+" is not in the registration vocabulary "+
				"(qa, qa_*, scorecard, test_result, test_results, evidence, implement_artifact, report, log, artifact)")
	}
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs, err = filepath.Abs(path)
		if err != nil {
			return ArtifactRegistration{}, err
		}
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return ArtifactRegistration{}, core.NewError(ReasonArtifactFileMissing,
			"goal.RegisterArtifact", "artifact file does not exist or is not a regular file: "+abs)
	}
	f, err := os.Open(abs)
	if err != nil {
		return ArtifactRegistration{}, core.NewError(ReasonArtifactFileMissing,
			"goal.RegisterArtifact", "artifact file unreadable: "+abs)
	}
	h := sha256.New()
	size, err := io.Copy(h, f)
	f.Close()
	if err != nil {
		return ArtifactRegistration{}, err
	}
	sum := hex.EncodeToString(h.Sum(nil))

	existing, err := e.store.Artifacts().ListByRun(run.ID)
	if err != nil {
		return ArtifactRegistration{}, err
	}
	var supersedes string
	for _, a := range existing {
		if a.Kind != kind || a.AbsPath != abs {
			continue
		}
		if a.ContentHash == sum {
			return ArtifactRegistration{Artifact: a, AlreadyRegistered: true}, nil
		}
		supersedes = a.ID // latest previous row for this run+kind+path
	}
	ref, err := e.store.Artifacts().Add(store.ArtifactRef{
		RunID: run.ID, Kind: kind, AbsPath: abs,
		ContentHash: sum, SizeBytes: size,
	})
	if err != nil {
		return ArtifactRegistration{}, err
	}
	payload := map[string]any{
		"artifact_id": ref.ID, "kind": kind, "abs_path": abs,
		"content_hash": sum, "size_bytes": size, "registered_by": by,
	}
	if supersedes != "" {
		payload["supersedes"] = supersedes
	}
	if err := e.appendEvent(run, EventArtifactRegistered, payload); err != nil {
		return ArtifactRegistration{}, err
	}
	return ArtifactRegistration{Artifact: ref, Supersedes: supersedes}, nil
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
