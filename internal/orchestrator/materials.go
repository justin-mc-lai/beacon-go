// Package materials freshness (R2a, parity audit M7): the Go port
// of truth_review_gate.mark_package_materials_stale +
// evaluate_release_truth_materials_gate's stale half.
//
// When a truth change opens, the tests/tasks materials no longer
// describe the (about-to-move) truth: their frontmatter
// materials_status is set to stale, persistently, in the files
// themselves. A successful freeze/refreeze clears the mark back to
// current. The release gate reads the marks and fails closed
// while any surface is stale — old materials must not ride into a
// release wearing a current face.
//
// The Python gate's other half (a truth-review report file must
// exist and pass) has no file counterpart here: the Go kernel's
// review verdicts are persisted gate results, and the release
// gate already requires the truth_freeze verdict as a
// prerequisite. See the R2a report's difference list.
package orchestrator

import (
	"os"
	"strings"

	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// markFileMaterialsStale sets materials_status: stale in one
// file's frontmatter, prepending a minimal frontmatter block when
// the file has none (mark_package_materials_stale, verbatim shape).
func markFileMaterialsStale(absPath, slug, version string) (bool, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return false, err
	}
	text := string(raw)
	if !strings.HasPrefix(text, "---") {
		out := "---\nslug: " + slug + "\nversion: " + version + "\nmaterials_status: stale\n---\n\n" + text
		if err := os.WriteFile(absPath, []byte(out), 0o644); err != nil {
			return false, err
		}
		return true, nil
	}
	updated, changed := setFrontmatterScalar(text, "materials_status", "stale")
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// markPackageMaterialsStale marks the tests and tasks surfaces of
// one package stale (the Python surfaces=("tests","tasks")).
func (e *Engine) markPackageMaterialsStale(ctx store.FeatureContext) ([]string, error) {
	pkg := e.packageFiles(ctx)
	var updated []string
	for _, pf := range []gate.PackageFile{pkg.Tests, pkg.Tasks} {
		if !pf.Present {
			continue
		}
		changed, err := markFileMaterialsStale(
			resolvePkgPath(ctx.Project.RootPath, pf.Path), ctx.Feature.Slug, ctx.Version.Version)
		if err != nil {
			return updated, err
		}
		if changed {
			updated = append(updated, pf.Path)
		}
	}
	return updated, nil
}

// clearPackageMaterialsStale flips stale marks back to current at
// freeze/refreeze. Only the stale value is touched — a draft or
// current mark is the author's own state, not the kernel's.
func (e *Engine) clearPackageMaterialsStale(ctx store.FeatureContext) {
	pkg := e.packageFiles(ctx)
	for _, pf := range []gate.PackageFile{pkg.Tests, pkg.Tasks} {
		if !pf.Present {
			continue
		}
		scalars, _, ok := gate.ParseFrontmatter(pf.Text)
		if !ok || !strings.EqualFold(strings.TrimSpace(scalars["materials_status"]), "stale") {
			continue
		}
		abs := resolvePkgPath(ctx.Project.RootPath, pf.Path)
		if updated, changed := setFrontmatterScalar(pf.Text, "materials_status", "current"); changed {
			_ = os.WriteFile(abs, []byte(updated), 0o644)
		}
	}
}

// staleMaterialSurfaces lists the surfaces (truth/tests/tasks —
// the Python gate's surface set) whose frontmatter currently
// reads materials_status: stale.
func (e *Engine) staleMaterialSurfaces(ctx store.FeatureContext) []string {
	pkg := e.packageFiles(ctx)
	var out []string
	for _, s := range []struct {
		name string
		pf   gate.PackageFile
	}{{"truth", pkg.Truth}, {"tests", pkg.Tests}, {"tasks", pkg.Tasks}} {
		if !s.pf.Present {
			continue
		}
		scalars, _, ok := gate.ParseFrontmatter(s.pf.Text)
		if ok && strings.EqualFold(strings.TrimSpace(scalars["materials_status"]), "stale") {
			out = append(out, s.name)
		}
	}
	return out
}
