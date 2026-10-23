// Shared helpers for the R2c exec & host surfaces (autotest, state
// portability, post-release hooks): resolving a feature context from
// a feature id alone, and reading the project-local .beacon/config
// line format that branch.go established for truth_canonical_branch.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// featureContext joins one feature id to its version and project
// rows via the delegate surface's resolver (featureContextOf in
// delegate.go), failing closed when the feature is unknown.
func (e *Engine) featureContext(featureID string) (store.FeatureContext, error) {
	ctx, ok, err := e.featureContextOf(featureID)
	if err != nil {
		return store.FeatureContext{}, err
	}
	if !ok {
		return store.FeatureContext{}, core.NewError(ReasonFeatureNotFound,
			"featureContext", "feature not found: "+featureID)
	}
	return ctx, nil
}

// projectConfigValue reads one key from the project root's
// .beacon/config (minimal line format: `key = value`, `#` comments
// allowed — the same format canonicalBranchOverride parses in
// branch.go). "" when the file or the key is absent.
func projectConfigValue(root, key string) string {
	raw, err := os.ReadFile(filepath.Join(root, ProjectConfigRelPath))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			k, v, found = strings.Cut(line, ":")
		}
		if found && strings.TrimSpace(k) == key {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}
