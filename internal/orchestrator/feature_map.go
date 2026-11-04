// Feature map (skills-distribution's sibling, issue: tweet 1's
// "Feature Map + CLI"). A one-way projection of truth + tests +
// tasks: for every AC, where the feature lives and which runnable
// command proves it. It never rewrites truth.
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

const FeatureMapContract = "feature_map.v1"

type FeatureMapRow struct {
	AC           string `json:"ac"`
	EntryPoint   string `json:"entry_point,omitempty"`
	Verification string `json:"verification,omitempty"`
	Status       string `json:"status"` // mapped | missing_entry | missing_verification
}

type FeatureMapView struct {
	Contract           string          `json:"contract"`
	Slug               string          `json:"slug"`
	Version            string          `json:"version"`
	Path               string          `json:"path"`
	SurfaceFingerprint string          `json:"surface_fingerprint"`
	Rows               []FeatureMapRow `json:"rows"`
	Mapped             int             `json:"mapped"`
	Status             string          `json:"status"` // pass | incomplete
}

var taskACRe = regexp.MustCompile(`ac=([A-Za-z0-9-]+)`)
var taskEvidenceRe = regexp.MustCompile(`evidence=([^\s·|]+)`)

// parseTaskBindings returns AC -> first entry-point-looking evidence.
func parseTaskBindings(tasks string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(tasks, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "- [") {
			continue
		}
		acm := taskACRe.FindStringSubmatch(line)
		if len(acm) < 2 {
			continue
		}
		ac := strings.ToUpper(acm[1])
		if !strings.HasPrefix(ac, "AC-") {
			continue
		}
		evm := taskEvidenceRe.FindStringSubmatch(line)
		if len(evm) < 2 {
			continue
		}
		ev := strings.TrimSpace(evm[1])
		if !entryPointLike(ev) {
			continue
		}
		if _, ok := out[ac]; !ok {
			out[ac] = ev
		}
	}
	return out
}

func entryPointLike(p string) bool {
	if p == "" || !strings.Contains(p, "/") {
		return false
	}
	if strings.HasPrefix(p, ".beacon/") || strings.HasPrefix(p, "docs/") {
		return false
	}
	return true
}

func uniqACs(docs []gate.ACDoc) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range docs {
		id := strings.ToUpper(strings.TrimSpace(d.ID))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// BuildFeatureMap writes <package>/feature-map.md and returns what it
// wrote. The truth/tests/tasks/evidence bodies are never modified.
func (e *Engine) BuildFeatureMap(projectRoot, slug string) (FeatureMapView, error) {
	fc, err := e.truthMapContext(projectRoot, slug)
	if err != nil {
		return FeatureMapView{}, err
	}
	pkg := e.packageFiles(fc)
	if !pkg.Truth.Present || strings.TrimSpace(pkg.Truth.Text) == "" {
		return FeatureMapView{}, core.NewError(core.ReasonTruthMapNotResolved, "truth.map.build",
			"no truth.md on disk for feature "+slug)
	}
	acs := uniqACs(gate.ExtractACDescriptions(pkg.Truth.Text))
	commands := map[string]string{}
	for _, row := range gate.ParseCoverageRows(pkg.Tests.Text) {
		if strings.TrimSpace(row.Command) == "" {
			continue
		}
		ac := strings.ToUpper(strings.TrimSpace(row.AC))
		if _, ok := commands[ac]; !ok {
			commands[ac] = row.Command
		}
	}
	entries := parseTaskBindings(pkg.Tasks.Text)

	view := FeatureMapView{
		Contract: FeatureMapContract, Slug: fc.Feature.Slug, Version: fc.Version.Version,
		SurfaceFingerprint: core.PackageSurfaceFingerprint(
			[]byte(pkg.Truth.Text), []byte(pkg.Tests.Text), []byte(pkg.Tasks.Text), []byte(pkg.Evidence.Text)),
	}
	for _, ac := range acs {
		row := FeatureMapRow{AC: ac, EntryPoint: entries[ac], Verification: commands[ac]}
		switch {
		case row.EntryPoint == "":
			row.Status = "missing_entry"
		case row.Verification == "":
			row.Status = "missing_verification"
		default:
			row.Status = "mapped"
			view.Mapped++
		}
		view.Rows = append(view.Rows, row)
	}
	view.Status = "incomplete"
	if len(view.Rows) > 0 && view.Mapped == len(view.Rows) {
		view.Status = "pass"
	}
	dir := resolvePkgPath(fc.Project.RootPath, pkgDirOf(pkg.Truth.Path))
	view.Path = filepath.Join(dir, "feature-map.md")
	if err := os.WriteFile(view.Path, []byte(renderFeatureMap(view)), 0o644); err != nil {
		return FeatureMapView{}, err
	}
	return view, nil
}

func renderFeatureMap(v FeatureMapView) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "slug: %s\nversion: %s\ngenerated_at: %s\nsurface_fingerprint: %s\n",
		v.Slug, v.Version, time.Now().UTC().Format(time.RFC3339Nano), v.SurfaceFingerprint)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# Feature Map: %s\n\n", v.Slug)
	b.WriteString("One-way projection of truth + tests + tasks. It never rewrites truth.\n\n")
	b.WriteString("| AC | Entry Point | Verification Command | Status |\n")
	b.WriteString("|----|-------------|----------------------|--------|\n")
	for _, r := range v.Rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			r.AC, cell(r.EntryPoint), cell(r.Verification), r.Status)
	}
	return b.String()
}

func cell(s string) string {
	if s == "" {
		return "—"
	}
	return strings.ReplaceAll(s, "|", "\\|")
}
