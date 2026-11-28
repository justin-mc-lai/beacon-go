package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"beacon-go/internal/harness"
)

// Extract a real Git snapshot without checking out or editing the live tree.
// Only contract-bound tests are overlaid; business source remains baseline.
func prepareCaptureReplay(ctx context.Context, root, baseline string, tests []string) (string, string, func(), error) {
	noop := func() {}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(baseline) {
		return "", "", noop, captureError("capture_baseline_invalid", "exact full commit required")
	}
	head, err := qaGit(root, "rev-parse", "HEAD")
	if err != nil {
		return "", "", noop, err
	}
	head = strings.TrimSpace(head)
	found, err := qaGit(root, "rev-parse", "--verify", baseline+"^{commit}")
	if err != nil || strings.TrimSpace(found) != baseline || baseline == head {
		return "", "", noop, captureError("capture_baseline_invalid", "existing older commit required")
	}
	ancestry := harness.RunCommand(ctx, []string{"git", "merge-base", "--is-ancestor", baseline, head}, root, os.Environ(), 0)
	if ancestry.Err != nil || ancestry.ExitCode != 0 {
		return "", "", noop, captureError("capture_baseline_invalid", "baseline must be ancestor")
	}
	listing := harness.RunCommand(ctx, []string{"git", "ls-tree", "-r", "-z", baseline}, root, os.Environ(), 0)
	if listing.Err != nil || listing.ExitCode != 0 || listing.TimedOut {
		return "", "", noop, captureError("capture_baseline_invalid", "tree listing failed")
	}
	dir, err := os.MkdirTemp("", "beacon-replay-")
	if err != nil {
		return "", "", noop, err
	}
	if real, e := filepath.EvalSymlinks(dir); e == nil {
		dir = real
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	fail := func(err error) (string, string, func(), error) { cleanup(); return "", "", noop, err }
	selected := map[string]bool{}
	for _, name := range tests {
		path, err := captureFile(root, name)
		if err != nil {
			return fail(err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fail(err)
		}
		selected[rel] = true
	}
	for _, entry := range strings.Split(listing.Stdout, "\x00") {
		if entry == "" {
			continue
		}
		metadata, name, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return fail(captureError("capture_baseline_invalid", "symlink/submodule/special Git entry refused"))
		}
		name = filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fail(captureError("capture_baseline_invalid", "Git path escapes"))
		}
		// Refuse rather than silently execute historical tests outside the overlay.
		if strings.HasSuffix(name, "_test.go") && !selected[name] {
			return fail(captureError("capture_replay_test_set_unsupported", "baseline contains test outside current effective set: "+filepath.ToSlash(name)))
		}
		path := filepath.Join(dir, name)
		blob := harness.RunCommand(ctx, []string{"git", "cat-file", "blob", fields[2]}, root, os.Environ(), 0)
		if blob.Err != nil || blob.ExitCode != 0 || blob.TimedOut {
			return fail(captureError("capture_baseline_invalid", "blob read failed"))
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return fail(err)
		}
		mode := os.FileMode(0600)
		if fields[0] == "100755" {
			mode = 0700
		}
		if err := os.WriteFile(path, []byte(blob.Stdout), mode); err != nil {
			return fail(err)
		}
	}
	for _, name := range tests {
		source, err := captureFile(root, name)
		if err != nil {
			return fail(err)
		}
		rel, err := filepath.Rel(root, source)
		if err != nil {
			return fail(err)
		}
		target := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return fail(err)
		}
		b, err := os.ReadFile(source)
		if err != nil {
			return fail(err)
		}
		info, err := os.Stat(source)
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(target, b, info.Mode().Perm()); err != nil {
			return fail(err)
		}
	}
	return dir, head, cleanup, nil
}
