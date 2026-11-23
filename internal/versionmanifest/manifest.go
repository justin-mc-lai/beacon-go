// Version manifest: the delivery-line contract.
//
// A version is a manifest that references immutable feature packages by
// their contract hash (truth + tests + tasks + restatement). Unchanged
// features are inherited by reference, never copied. A sea is a
// version-scoped lake graph. The manifest is the contract; run metadata
// remains the execution anchor.
package versionmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/core"
)

const (
	Schema        = "beacon-version-manifest.v1"
	PackageSchema = "beacon-package.v1"

	StatusBaseline = "baseline"
	StatusDraft    = "draft"
	StatusFrozen   = "frozen"
	StatusReleased = "released"

	LakeInherited = "inherited"
	LakeChanged   = "changed"
	LakeNew       = "new"
	LakeDirty     = "dirty"
	LakeRetired   = "retired"
	LakeBaseline  = "baseline"
)

type Manifest struct {
	Schema     string   `json:"schema"`
	Version    string   `json:"version"`
	Parent     string   `json:"parent,omitempty"`
	Runtime    string   `json:"runtime,omitempty"`
	Status     string   `json:"status"`
	Lakes      []Lake   `json:"lakes"`
	Seas       []Sea    `json:"seas,omitempty"`
	Changes    []Change `json:"changes,omitempty"`
	CreatedAt  string   `json:"created_at"`
	FrozenAt   string   `json:"frozen_at,omitempty"`
	ReleasedAt string   `json:"released_at,omitempty"`
}

type Lake struct {
	Slug     string            `json:"slug"`
	Package  string            `json:"package"`
	Status   string            `json:"status"`
	Gates    map[string]string `json:"gates,omitempty"`
	Artifact string            `json:"artifact,omitempty"`
	Changes  []string          `json:"changes,omitempty"`
}

type Sea struct {
	Slug        string    `json:"slug"`
	HostFeature string    `json:"host_feature"`
	Frozen      bool      `json:"frozen"`
	Lakes       []SeaLake `json:"lakes"`
	Proposal    string    `json:"proposal,omitempty"`
}

type SeaLake struct {
	Slug      string   `json:"slug"`
	Package   string   `json:"package,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type Change struct {
	ChangeID string   `json:"change_id"`
	Lakes    []string `json:"lakes"`
	Source   string   `json:"source"`
	Status   string   `json:"status"`
}

type PackageMeta struct {
	Schema                  string            `json:"schema"`
	Slug                    string            `json:"slug"`
	ContractHash            string            `json:"contract_hash"`
	Files                   map[string]string `json:"files"`
	RequirementFingerprints map[string]string `json:"requirement_fingerprints,omitempty"`
	SourceVersion           string            `json:"source_version,omitempty"`
	ChangeIDs               []string          `json:"change_ids,omitempty"`
	CreatedAt               string            `json:"created_at"`
}

func Path(root, version string) string {
	return filepath.Join(root, "docs", "beacon", version, "manifest.json")
}

func PackageDir(root, slug, hash string) string {
	return filepath.Join(root, "docs", "beacon", "packages", slug, hash)
}

func Load(path string) (Manifest, error) {
	var m Manifest
	raw, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("manifest JSON: %w", err)
	}
	return m, nil
}

func LoadVersion(root, version string) (Manifest, error) {
	return Load(Path(root, version))
}

func Save(path string, m Manifest) error {
	if err := Validate(m); err != nil {
		return err
	}
	if m.CreatedAt == "" {
		m.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Validate(m Manifest) error {
	if m.Schema != Schema {
		return fmt.Errorf("manifest schema %q is not %s", m.Schema, Schema)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("manifest version is required")
	}
	switch m.Status {
	case StatusBaseline, StatusDraft, StatusFrozen, StatusReleased:
	default:
		return fmt.Errorf("unknown manifest status %q", m.Status)
	}
	seen := map[string]bool{}
	for i, l := range m.Lakes {
		if strings.TrimSpace(l.Slug) == "" {
			return fmt.Errorf("lakes[%d] has an empty slug", i)
		}
		if seen[l.Slug] {
			return fmt.Errorf("duplicate lake %q", l.Slug)
		}
		seen[l.Slug] = true
		if l.Status != LakeRetired && strings.TrimSpace(l.Package) == "" {
			return fmt.Errorf("lake %q has no package hash", l.Slug)
		}
	}
	seaSeen := map[string]bool{}
	for i, s := range m.Seas {
		if strings.TrimSpace(s.Slug) == "" || strings.TrimSpace(s.HostFeature) == "" {
			return fmt.Errorf("seas[%d] needs slug and host_feature", i)
		}
		if seaSeen[s.Slug] {
			return fmt.Errorf("duplicate sea %q", s.Slug)
		}
		seaSeen[s.Slug] = true
		lakeSeen := map[string]bool{}
		for j, l := range s.Lakes {
			if strings.TrimSpace(l.Slug) == "" {
				return fmt.Errorf("seas[%d].lakes[%d] has an empty slug", i, j)
			}
			if lakeSeen[l.Slug] {
				return fmt.Errorf("sea %q has duplicate lake %q", s.Slug, l.Slug)
			}
			lakeSeen[l.Slug] = true
		}
	}
	return nil
}

// Inherit opens a child manifest from a parent: every lake and sea is
// carried by reference, with no file copy.
func Inherit(parent Manifest, version string) Manifest {
	now := time.Now().UTC().Format(time.RFC3339)
	child := Manifest{
		Schema:    Schema,
		Version:   version,
		Parent:    parent.Version,
		Runtime:   parent.Runtime,
		Status:    StatusDraft,
		Lakes:     make([]Lake, 0, len(parent.Lakes)),
		Seas:      make([]Sea, 0, len(parent.Seas)),
		CreatedAt: now,
	}
	for _, l := range parent.Lakes {
		child.Lakes = append(child.Lakes, Lake{
			Slug: l.Slug, Package: l.Package, Status: LakeInherited,
		})
	}
	for _, s := range parent.Seas {
		child.Seas = append(child.Seas, s)
	}
	return child
}

// HashContract is the package identity: truth + tests + tasks + restatement.
func HashContract(truth, tests, tasks, restatement []byte) string {
	return core.PackageContractFingerprint(truth, tests, tasks, restatement)
}

// WritePackage snapshots the contract files under packages/<slug>/<hash>/.
// It is idempotent: the same bytes and hash write the same files.
func WritePackage(root, slug, hash, sourceVersion string, files map[string][]byte, changeIDs []string) (PackageMeta, error) {
	dir := PackageDir(root, slug, hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return PackageMeta{}, err
	}
	hashes := map[string]string{}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return PackageMeta{}, err
		}
		sum := sha256.Sum256(body)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	meta := PackageMeta{
		Schema:        PackageSchema,
		Slug:          slug,
		ContractHash:  hash,
		Files:         hashes,
		SourceVersion: sourceVersion,
		ChangeIDs:     changeIDs,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	meta.RequirementFingerprints = core.RequirementFingerprints(files["truth.md"])
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return PackageMeta{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), append(raw, '\n'), 0o644); err != nil {
		return PackageMeta{}, err
	}
	return meta, nil
}

// LakeIndex returns slug -> lake for quick lookup.
func (m Manifest) LakeIndex() map[string]*Lake {
	out := make(map[string]*Lake, len(m.Lakes))
	for i := range m.Lakes {
		out[m.Lakes[i].Slug] = &m.Lakes[i]
	}
	return out
}

// SortedLakes returns lakes sorted by slug for stable output.
func (m Manifest) SortedLakes() []Lake {
	out := append([]Lake{}, m.Lakes...)
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}
