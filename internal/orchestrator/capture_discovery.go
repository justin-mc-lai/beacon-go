package orchestrator

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/harness"
	"beacon-go/internal/intenttree"
)

// DiscoverCaptureTests is execution input discovery, not a requirement edit.
// The same package projection is subsequently checked by preflight.
func discoverCaptureTests(ctx context.Context, root string, packages []string, timeout time.Duration, observed *intenttree.CapturePreflight) ([]string, error) {
	argv := append([]string{"go", "list", "-e", "-json"}, packages...)
	out := harness.RunCommand(ctx, argv, root, captureEnv(), timeout)
	*observed = intenttree.CapturePreflight{Command: argv, ExitCode: out.ExitCode, Stdout: out.Stdout, Stderr: out.Stderr, TimedOut: out.TimedOut}
	if out.Err != nil {
		observed.Error = out.Err.Error()
	}
	if out.Err != nil || out.ExitCode != 0 || out.TimedOut {
		if out.TimedOut {
			return nil, capturePreflightTimeout{}
		}
		return nil, captureError("capture_discovery_failed", "go list could not discover current test inputs")
	}
	decoder := json.NewDecoder(strings.NewReader(out.Stdout))
	seen := map[string]bool{}
	seenPackages := map[string]bool{}
	var files []string
	for decoder.More() {
		var p struct {
			Dir                       string
			TestGoFiles, XTestGoFiles []string
			Error                     *struct{ Err string }
			Incomplete                bool
		}
		if err := decoder.Decode(&p); err != nil {
			return nil, err
		}
		if p.Error != nil || p.Incomplete || p.Dir == "" {
			return nil, captureError("capture_package_invalid", "go list reported incomplete package")
		}
		dir, err := filepath.EvalSymlinks(p.Dir)
		if err != nil {
			return nil, err
		}
		if seenPackages[dir] {
			return nil, captureError("capture_package_invalid", "duplicate/alias package")
		}
		seenPackages[dir] = true
		for _, name := range append(p.TestGoFiles, p.XTestGoFiles...) {
			path, err := captureFile(root, filepath.Join(p.Dir, name))
			if err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil, err
			}
			if seen[rel] {
				return nil, captureError("capture_discovery_failed", "overlapping package selection")
			}
			seen[rel] = true
			files = append(files, filepath.ToSlash(rel))
		}
	}
	if len(files) == 0 {
		return nil, captureError("capture_test_files_invalid", "discovery found zero test inputs")
	}
	sort.Strings(files)
	return files, nil
}
