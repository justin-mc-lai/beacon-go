// Archive governance (R2c, parity audit M10): locating archived
// requirement packages by version and recording their archive
// status, ported from Python commands/archive.py +
// utils/archive_governance.py.
//
// Python semantics kept exactly: archiving does NOT move the
// package. `apply` writes governance metadata —
// docs/beacon/<version>/.machine/archive/feature-archive-<slug>.json
// per feature, archive-index.json per version, archive-plan.json
// for the last plan — under the status model
// active / superseded / archived-learnable /
// archived-reference-only. (Physical relocation of whole version
// trees is Python's separate version-truth-catalog surface; the
// scan below still READS relocated trees under
// docs/beacon/archive/<version>/ so lineage stays whole.)
//
// This face is disk-based like the Python original: it works on
// any project root, registered or not, and never touches the DB.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// ArchiveStatuses is archive_governance.ARCHIVE_STATUSES verbatim.
var ArchiveStatuses = []string{"active", "superseded", "archived-learnable", "archived-reference-only"}

// archivedStatuses are the two statuses that count as archived.
var archivedStatuses = map[string]bool{
	"archived-learnable": true, "archived-reference-only": true,
}

// ArchiveEntry is one feature package as the archive face sees it.
type ArchiveEntry struct {
	Version     string `json:"version"`
	Slug        string `json:"slug"`
	Location    string `json:"location"` // active | archive
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	TruthStatus string `json:"truth_status,omitempty"`
	PackageDir  string `json:"package_dir"` // project-relative
	MetadataRef string `json:"metadata_ref,omitempty"`
	ACCount     int    `json:"ac_count"`
	Archived    bool   `json:"archived"`
	Learnable   bool   `json:"learnable,omitempty"`
}

type archiveVersionRoot struct {
	Version  string
	RelBase  string // docs/beacon/<v> or docs/beacon/archive/<v>
	Location string
}

// semverTuple parses v?X.Y.Z (Python _semver_tuple semantics:
// unparseable -> not a version).
func semverTuple(v string) ([3]int, bool) {
	s := strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// archiveGovernanceSupported mirrors archive_governance_supported:
// version >= 1.3.8.
func archiveGovernanceSupported(version string) bool {
	t, ok := semverTuple(version)
	if !ok {
		return false
	}
	return t[0] > 1 || (t[0] == 1 && (t[1] > 3 || (t[1] == 3 && t[2] >= 8)))
}

// discoverArchiveRoots lists the version roots (active first, then
// archive roots), each sorted by semver.
func discoverArchiveRoots(root string) []archiveVersionRoot {
	var out []archiveVersionRoot
	scan := func(base string, location string) {
		entries, err := os.ReadDir(filepath.Join(root, base))
		if err != nil {
			return
		}
		var found []archiveVersionRoot
		for _, en := range entries {
			if !en.IsDir() {
				continue
			}
			if _, ok := semverTuple(en.Name()); !ok {
				continue
			}
			found = append(found, archiveVersionRoot{
				Version: en.Name(), RelBase: filepath.Join(base, en.Name()), Location: location,
			})
		}
		sort.Slice(found, func(i, j int) bool {
			a, _ := semverTuple(found[i].Version)
			b, _ := semverTuple(found[j].Version)
			return a[0] < b[0] || (a[0] == b[0] && (a[1] < b[1] || (a[1] == b[1] && a[2] < b[2])))
		})
		out = append(out, found...)
	}
	scan(filepath.Join("docs", "beacon"), "active")
	scan(filepath.Join("docs", "beacon", "archive"), "archive")
	return out
}

// archiveMetadata is the on-disk feature-archive-<slug>.json shape
// (build_feature_archive_metadata's payload).
type archiveMetadata struct {
	SchemaVersion  string   `json:"schema_version"`
	Feature        string   `json:"feature"`
	FeatureSlug    string   `json:"feature_slug"`
	Version        string   `json:"version"`
	Status         string   `json:"status"`
	Reason         string   `json:"reason"`
	ActiveDelivery bool     `json:"active_delivery"`
	SourceRefs     []string `json:"source_refs"`
	LearnableRefs  []string `json:"learnable_archive_refs"`
	ExcludedRefs   []string `json:"excluded_from_current_delivery_refs"`
	ArchivedAt     string   `json:"archived_at"`
	GeneratedAt    string   `json:"generated_at"`
}

func archiveMetadataRel(vr archiveVersionRoot, slug string) string {
	return filepath.Join(vr.RelBase, ".machine", "archive", "feature-archive-"+slug+".json")
}

func loadArchiveMetadata(root string, vr archiveVersionRoot, slug string) (archiveMetadata, bool) {
	raw, err := os.ReadFile(filepath.Join(root, archiveMetadataRel(vr, slug)))
	if err != nil {
		return archiveMetadata{}, false
	}
	var md archiveMetadata
	if json.Unmarshal(raw, &md) != nil || md.Status == "" {
		return archiveMetadata{}, false
	}
	return md, true
}

var archiveACRe = regexp.MustCompile(`AC-\d{3}`)

// scanRootEntries lists the packages of one version root with
// their archive facts (status from metadata when present, else
// active — Python's _normalize_status default).
func scanRootEntries(root string, vr archiveVersionRoot) []ArchiveEntry {
	featuresDir := filepath.Join(root, vr.RelBase, "features")
	dirs, err := os.ReadDir(featuresDir)
	if err != nil {
		return nil
	}
	var out []ArchiveEntry
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		slug := d.Name()
		truthAbs := filepath.Join(featuresDir, slug, "truth.md")
		raw, rerr := os.ReadFile(truthAbs)
		if rerr != nil {
			continue // Python discovers features by features/*/truth.md
		}
		entry := ArchiveEntry{
			Version: vr.Version, Slug: slug, Location: vr.Location, Status: "active",
			PackageDir: filepath.ToSlash(filepath.Join(vr.RelBase, "features", slug)),
		}
		if fm, _, ok := gate.ParseFrontmatter(string(raw)); ok {
			entry.TruthStatus = fm["status"]
		}
		seen := map[string]bool{}
		for _, ac := range archiveACRe.FindAllString(string(raw), -1) {
			if !seen[ac] {
				seen[ac] = true
				entry.ACCount++
			}
		}
		if md, ok := loadArchiveMetadata(root, vr, slug); ok {
			entry.Status = md.Status
			entry.Reason = md.Reason
			entry.MetadataRef = filepath.ToSlash(archiveMetadataRel(vr, slug))
		}
		entry.Archived = archivedStatuses[entry.Status]
		entry.Learnable = entry.Status == "archived-learnable"
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ScanArchiveEntries lists every package in every version root
// (active + relocated archive roots), version order then slug.
func ScanArchiveEntries(root string) []ArchiveEntry {
	var out []ArchiveEntry
	for _, vr := range discoverArchiveRoots(root) {
		out = append(out, scanRootEntries(root, vr)...)
	}
	return out
}

// ArchiveLineage traces one feature across all version roots,
// oldest version first (build_archive_lineage's hop list, flattened
// to the same entries list shows).
func ArchiveLineage(root, slug string) []ArchiveEntry {
	var out []ArchiveEntry
	for _, e := range ScanArchiveEntries(root) {
		if e.Slug == slug {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := semverTuple(out[i].Version)
		b, _ := semverTuple(out[j].Version)
		if a != b {
			return a[0] < b[0] || (a[0] == b[0] && (a[1] < b[1] || (a[1] == b[1] && a[2] < b[2])))
		}
		return out[i].Location < out[j].Location
	})
	return out
}

// ArchiveApplyResult is one ApplyArchiveStatus outcome.
type ArchiveApplyResult struct {
	Version     string `json:"version"`
	Feature     string `json:"feature"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	MetadataRef string `json:"metadata_ref"`
	IndexRef    string `json:"index_ref"`
	PlanRef     string `json:"plan_ref"`
}

// ApplyArchiveStatus records one feature's archive status the
// Python way: per-feature metadata + the version's archive index +
// the archive plan, all under <version root>/.machine/archive/.
func ApplyArchiveStatus(root, version, slug, status, reason string) (ArchiveApplyResult, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	valid := false
	for _, s := range ArchiveStatuses {
		if s == status {
			valid = true
		}
	}
	if !valid {
		return ArchiveApplyResult{}, core.NewError(core.ReasonArchiveStatusInvalid,
			"archive.Apply", "archive status must be one of "+strings.Join(ArchiveStatuses, ", "))
	}
	if !archiveGovernanceSupported(version) {
		return ArchiveApplyResult{}, core.NewError(core.ReasonArchiveNotSupported,
			"archive.Apply", "archive governance is not supported for version "+version+" (requires >= v1.3.8)")
	}
	var target *archiveVersionRoot
	for _, vr := range discoverArchiveRoots(root) {
		if vr.Version != version {
			continue
		}
		for _, e := range scanRootEntries(root, vr) {
			if e.Slug == slug {
				v := vr
				target = &v
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		return ArchiveApplyResult{}, core.NewError(core.ReasonArchiveFeatureNotFound,
			"archive.Apply", "feature "+slug+" not found in version "+version+" under "+root)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// Source refs: the package files that exist (Python collected
	// prd/user-story/qa/machine refs; the Go package layout keeps
	// the four-piece set under features/<slug>/).
	var refs []string
	for _, name := range []string{"truth.md", "tests.md", "tasks.md", "evidence.md"} {
		rel := filepath.Join(target.RelBase, "features", slug, name)
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			refs = append(refs, filepath.ToSlash(rel))
		}
	}
	md := archiveMetadata{
		SchemaVersion: "1.0", Feature: slug, FeatureSlug: slug, Version: version,
		Status: status, Reason: reason, ActiveDelivery: status == "active",
		SourceRefs: refs, GeneratedAt: now,
	}
	if status == "archived-learnable" {
		md.LearnableRefs = refs
	}
	if archivedStatuses[status] {
		md.ExcludedRefs = refs
		md.ArchivedAt = now
	}
	if md.SourceRefs == nil {
		md.SourceRefs = []string{}
	}
	if md.LearnableRefs == nil {
		md.LearnableRefs = []string{}
	}
	if md.ExcludedRefs == nil {
		md.ExcludedRefs = []string{}
	}
	machineDir := filepath.Join(root, target.RelBase, ".machine", "archive")
	if err := os.MkdirAll(machineDir, 0o755); err != nil {
		return ArchiveApplyResult{}, err
	}
	writeJSON := func(name string, v any) error {
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(machineDir, name), append(raw, '\n'), 0o644)
	}
	if err := writeJSON("feature-archive-"+slug+".json", md); err != nil {
		return ArchiveApplyResult{}, err
	}
	// Version index: every feature of the version with its status.
	entries := scanRootEntries(root, *target)
	type indexRow struct {
		Feature        string `json:"feature"`
		FeatureSlug    string `json:"feature_slug"`
		Status         string `json:"status"`
		Reason         string `json:"reason,omitempty"`
		ActiveDelivery bool   `json:"active_delivery"`
		MetadataRef    string `json:"feature_metadata_ref"`
	}
	rows := []indexRow{}
	for _, e := range entries {
		rows = append(rows, indexRow{
			Feature: e.Slug, FeatureSlug: e.Slug, Status: e.Status, Reason: e.Reason,
			ActiveDelivery: e.Status == "active",
			MetadataRef:    filepath.ToSlash(archiveMetadataRel(*target, e.Slug)),
		})
	}
	index := map[string]any{
		"schema_version": "1.0", "version": version,
		"current_version_truth_root": filepath.ToSlash(target.RelBase),
		"features":                   rows, "generated_at": now,
	}
	if err := writeJSON("archive-index.json", index); err != nil {
		return ArchiveApplyResult{}, err
	}
	plan := map[string]any{
		"schema_version": "1.0", "version": version, "feature": slug,
		"requested_status": status, "reason": reason, "generated_at": now,
	}
	if err := writeJSON("archive-plan.json", plan); err != nil {
		return ArchiveApplyResult{}, err
	}
	base := filepath.Join(target.RelBase, ".machine", "archive")
	return ArchiveApplyResult{
		Version: version, Feature: slug, Status: status, Reason: reason,
		MetadataRef: filepath.ToSlash(filepath.Join(base, "feature-archive-"+slug+".json")),
		IndexRef:    filepath.ToSlash(filepath.Join(base, "archive-index.json")),
		PlanRef:     filepath.ToSlash(filepath.Join(base, "archive-plan.json")),
	}, nil
}
