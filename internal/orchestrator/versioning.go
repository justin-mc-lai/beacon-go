// Docs-version iteration rules (SDD: a version is a delivery line, not a
// per-feature counter). Runtime and docs are two axes: runtime lines look
// like 2.0.0-go.12; docs lines are pure vX.Y.Z.
package orchestrator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

var docsVersionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// NormalizeDocsVersion accepts X.Y.Z or vX.Y.Z and returns vX.Y.Z.
// Anything else (including runtime-shaped x.y.z-go.N) is refused.
func NormalizeDocsVersion(raw string) (string, error) {
	m := docsVersionRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", core.NewError(core.ReasonDocsVersionInvalid, "docs.version",
			"invalid docs version "+raw+" (want X.Y.Z or vX.Y.Z; runtime lines like 2.0.0-go.12 are not docs versions)")
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

// BumpDocsVersion opens the next delivery line. The default level is
// patch; minor and major require the caller to say so explicitly.
func BumpDocsVersion(current, level string) (string, error) {
	norm, err := NormalizeDocsVersion(current)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.TrimPrefix(norm, "v"), ".")
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "patch":
		patch++
	case "minor":
		minor++
		patch = 0
	case "major":
		major++
		minor = 0
		patch = 0
	default:
		return "", core.NewError(core.ReasonDocsVersionInvalid, "docs.version",
			"unknown bump level "+level+" (patch|minor|major)")
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func docsVersionKey(v string) (int, int, int) {
	norm, err := NormalizeDocsVersion(v)
	if err != nil {
		return 0, 0, 0
	}
	parts := strings.Split(strings.TrimPrefix(norm, "v"), ".")
	a, _ := strconv.Atoi(parts[0])
	b, _ := strconv.Atoi(parts[1])
	c, _ := strconv.Atoi(parts[2])
	return a, b, c
}

// latestDocsVersion returns the largest registered docs version.
func latestDocsVersion(versions []store.ProjectVersion) (store.ProjectVersion, bool) {
	var best store.ProjectVersion
	found := false
	for _, v := range versions {
		if !found {
			best, found = v, true
			continue
		}
		bm, bn, bp := docsVersionKey(best.Version)
		vm, vn, vp := docsVersionKey(v.Version)
		if vm > bm || (vm == bm && vn > bn) || (vm == bm && vn == bn && vp > bp) {
			best = v
		}
	}
	return best, found
}

// LatestDocsVersion exposes the largest registered docs line for the
// CLI bump surface.
func LatestDocsVersion(versions []store.ProjectVersion) (store.ProjectVersion, bool) {
	return latestDocsVersion(versions)
}
