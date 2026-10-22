// GateDao and ArtifactDao (AC-073, AC-074, AC-076, AC-010). The Spine
// read model lives in spine.go.
package store

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
)

// GateResult is a gate_results row (AC-073).
type GateResult struct {
	ID          string   `json:"id"`
	FeatureID   string   `json:"feature_id"`
	GateType    string   `json:"gate_type"`
	Verdict     string   `json:"verdict"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	EvaluatedAt string   `json:"evaluated_at"`
}

// GateDao implements gate result access.
type GateDao struct {
	s    *Store
	q    dbtx
	inTx bool
}

// Gates returns the GateDao.
func (s *Store) Gates() *GateDao { return &GateDao{s: s, q: s.db} }

// Record writes one gate evaluation (AC-013). A "blocked" verdict must
// carry reason codes, mirroring the core Blocked obligation (AC-028).
func (d *GateDao) Record(g GateResult) (GateResult, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return GateResult{}, err
	}
	if g.Verdict == "blocked" && len(orEmptyCodes(g.ReasonCodes)) == 0 {
		return GateResult{}, core.NewError(core.ReasonBlockedMissingReasons, "GateDao.Record",
			"blocked gate verdict requires reason_codes")
	}
	if g.ID == "" {
		g.ID = newID("gate_")
	}
	g.EvaluatedAt = orNow(g.EvaluatedAt)
	codes, err := json.Marshal(orEmptyCodes(g.ReasonCodes))
	if err != nil {
		return GateResult{}, err
	}
	_, err = d.q.Exec(`INSERT INTO gate_results(id, feature_id, gate_type, verdict, reason_codes_json, evaluated_at) VALUES(?,?,?,?,?,?)`,
		g.ID, g.FeatureID, g.GateType, g.Verdict, string(codes), g.EvaluatedAt)
	if err != nil {
		return GateResult{}, err
	}
	return g, nil
}

func scanGate(row interface{ Scan(...any) error }) (GateResult, error) {
	var g GateResult
	var codes string
	err := row.Scan(&g.ID, &g.FeatureID, &g.GateType, &g.Verdict, &codes, &g.EvaluatedAt)
	if codes != "" {
		_ = json.Unmarshal([]byte(codes), &g.ReasonCodes)
	}
	return g, err
}

const gateCols = `id, feature_id, gate_type, verdict, reason_codes_json, evaluated_at`

// Latest returns the newest evaluation of (feature, gate_type).
func (d *GateDao) Latest(featureID, gateType string) (GateResult, error) {
	return scanGate(d.q.QueryRow(`SELECT `+gateCols+` FROM gate_results WHERE feature_id = ? AND gate_type = ? ORDER BY evaluated_at DESC, rowid DESC LIMIT 1`, featureID, gateType))
}

// ListByFeature returns all gate results of a feature, newest first.
func (d *GateDao) ListByFeature(featureID string) ([]GateResult, error) {
	rows, err := d.q.Query(`SELECT `+gateCols+` FROM gate_results WHERE feature_id = ? ORDER BY evaluated_at DESC, rowid DESC`, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GateResult
	for rows.Next() {
		g, err := scanGate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ArtifactRef is an artifact_refs row: a blob pointer only (AC-074). The
// blob itself lives on disk; the DB stores abs_path + content_hash (AC-010).
type ArtifactRef struct {
	ID          string `json:"id"`
	RunID       string `json:"run_id"`
	Kind        string `json:"kind"`
	AbsPath     string `json:"abs_path"`
	ContentHash string `json:"content_hash"`
	SizeBytes   int64  `json:"size_bytes"`
	CreatedAt   string `json:"created_at"`
}

// ArtifactDao implements artifact pointer access.
type ArtifactDao struct {
	s    *Store
	q    dbtx
	inTx bool
}

// Artifacts returns the ArtifactDao.
func (s *Store) Artifacts() *ArtifactDao { return &ArtifactDao{s: s, q: s.db} }

// Add inserts a blob pointer. AbsPath must be absolute (AC-083).
func (d *ArtifactDao) Add(a ArtifactRef) (ArtifactRef, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return ArtifactRef{}, err
	}
	if !filepath.IsAbs(a.AbsPath) {
		return ArtifactRef{}, core.NewError(ReasonArtifactPathNotAbs, "ArtifactDao.Add",
			"abs_path must be absolute: "+a.AbsPath)
	}
	if a.ID == "" {
		a.ID = newID("art_")
	}
	a.CreatedAt = orNow(a.CreatedAt)
	_, err := d.q.Exec(`INSERT INTO artifact_refs(id, run_id, kind, abs_path, content_hash, size_bytes, created_at) VALUES(?,?,?,?,?,?,?)`,
		a.ID, a.RunID, a.Kind, a.AbsPath, a.ContentHash, a.SizeBytes, a.CreatedAt)
	if err != nil {
		return ArtifactRef{}, err
	}
	return a, nil
}

func scanArtifact(row interface{ Scan(...any) error }) (ArtifactRef, error) {
	var a ArtifactRef
	err := row.Scan(&a.ID, &a.RunID, &a.Kind, &a.AbsPath, &a.ContentHash, &a.SizeBytes, &a.CreatedAt)
	return a, err
}

const artifactCols = `id, run_id, kind, abs_path, content_hash, size_bytes, created_at`

// ListByRun returns artifact pointers of a run in creation order.
func (d *ArtifactDao) ListByRun(runID string) ([]ArtifactRef, error) {
	rows, err := d.q.Query(`SELECT `+artifactCols+` FROM artifact_refs WHERE run_id = ? ORDER BY created_at, rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArtifactRef
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListByHash returns artifact pointers with the given content hash.
func (d *ArtifactDao) ListByHash(hash string) ([]ArtifactRef, error) {
	rows, err := d.q.Query(`SELECT `+artifactCols+` FROM artifact_refs WHERE content_hash = ? ORDER BY created_at, rowid`, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArtifactRef
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// IsStale reports whether a pointer is stale relative to the blob's
// current hash (AC-010): the stored hash differs from the actual hash.
func IsStale(a ArtifactRef, actualHash string) bool {
	return !strings.EqualFold(a.ContentHash, actualHash)
}

// The Spine read model (SpineSnapshot, SpineDao, project_key) lives in
// spine.go since P1; this file keeps GateDao and ArtifactDao.
