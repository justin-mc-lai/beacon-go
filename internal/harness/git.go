package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git plumbing for the real worktree integration (AC-099). All git
// invocations are argv-only (no shell), run with the repository root
// as cwd, and their combined output is returned for evidence — git
// failures surface as errors carrying git's own words, never
// swallowed.

// IsGitRepoRoot reports whether root is the top level of a git work
// tree (rev-parse --show-toplevel must resolve exactly to root).
func IsGitRepoRoot(ctx context.Context, root string) bool {
	out, err := gitOutput(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return top == abs
}

// IsGitRepo reports whether dir sits inside any git work tree.
func IsGitRepo(ctx context.Context, dir string) bool {
	_, err := gitOutput(ctx, dir, "rev-parse", "--git-dir")
	return err == nil
}

// CurrentBranch returns the checked-out branch of the work tree at
// dir ("HEAD" when detached).
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := gitOutput(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// MainDirty reports whether the work tree at root has uncommitted
// changes (tracked modifications or untracked files —
// status --porcelain non-empty). Beacon's own reserved runtime
// directories are excluded via pathspec: .beacon-worktrees/ (the
// satellite container — the worktrees beacon itself created must not
// make the main tree "dirty" forever) and .beacon/ (captured
// evidence). Everything else counts, exactly like git status.
func MainDirty(ctx context.Context, root string) (bool, error) {
	out, err := gitOutput(ctx, root, "status", "--porcelain", "--", ".",
		":(exclude).beacon-worktrees", ":(exclude).beacon")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// HasUncommittedChanges is MainDirty for an arbitrary work tree.
func HasUncommittedChanges(ctx context.Context, dir string) (bool, error) {
	return MainDirty(ctx, dir)
}

// HeadCommit returns the HEAD commit hash of the work tree at dir.
func HeadCommit(ctx context.Context, dir string) (string, error) {
	out, err := gitOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// BranchExists reports whether a local branch exists.
func BranchExists(ctx context.Context, root, branch string) bool {
	_, err := gitOutput(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// WorktreeList parses `git worktree list --porcelain` into
// path -> branch ("" for a detached HEAD entry).
func WorktreeList(ctx context.Context, root string) (map[string]string, error) {
	out, err := gitOutput(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	res := map[string]string{}
	var cur string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			res[cur] = ""
		case strings.HasPrefix(line, "branch ") && cur != "":
			res[cur] = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "branch ")), "refs/heads/")
		}
	}
	return res, nil
}

// WorktreeAdd creates a worktree at path on a NEW branch `branch`
// forked from startPoint (a commit-ish; empty = HEAD).
func WorktreeAdd(ctx context.Context, root, path, branch, startPoint string) (string, error) {
	args := []string{"worktree", "add", "-b", branch, path}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	return gitOutput(ctx, root, args...)
}

// MergeInto runs `git merge --no-ff --no-edit <branch>` in dir and
// reports the conflicted files when the merge stops on conflicts.
func MergeInto(ctx context.Context, dir, branch string) (conflicts []string, out string, err error) {
	out, err = gitOutput(ctx, dir, "merge", "--no-ff", "--no-edit", branch)
	if err == nil {
		return nil, out, nil
	}
	// A failed merge with conflict markers in the index: list them,
	// then abort so the main star is never left mid-merge.
	if diffOut, derr := gitOutput(ctx, dir, "diff", "--name-only", "--diff-filter=U"); derr == nil {
		for _, f := range strings.Split(diffOut, "\n") {
			if f = strings.TrimSpace(f); f != "" {
				conflicts = append(conflicts, f)
			}
		}
	}
	if len(conflicts) > 0 {
		_, _ = gitOutput(ctx, dir, "merge", "--abort")
	}
	return conflicts, out, err
}

// IsAncestor reports whether commit `anc` is reachable from `desc`
// (merge-base --is-ancestor; exit 1 = not an ancestor).
func IsAncestor(ctx context.Context, dir, anc, desc string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", anc, desc)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// ChangedFiles lists the files that differ between two refs
// (three-dot diff: what merging head into base would touch). Used
// by the protected-path write guard before a satellite merge lands
// anything on the main star.
func ChangedFiles(ctx context.Context, dir, base, head string) ([]string, error) {
	out, err := gitOutput(ctx, dir, "diff", "--name-only", base+"..."+head)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// WorktreeRemove removes a worktree (git refuses a dirty one unless
// force is set — callers gate force behind an explicit discard).
func WorktreeRemove(ctx context.Context, root, path string, force bool) (string, error) {
	args := []string{"worktree", "remove", path}
	if force {
		args = []string{"worktree", "remove", "--force", path}
	}
	return gitOutput(ctx, root, args...)
}

// DeleteBranch deletes a local branch (safe delete: git refuses an
// unmerged branch without force).
func DeleteBranch(ctx context.Context, root, branch string, force bool) (string, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return gitOutput(ctx, root, "branch", flag, branch)
}

// SwitchBranch checks out an existing branch in dir.
func SwitchBranch(ctx context.Context, dir, branch string) (string, error) {
	return gitOutput(ctx, dir, "switch", branch)
}

// CreateAndSwitchBranch creates a new branch at HEAD and checks it
// out in dir (git switch -c).
func CreateAndSwitchBranch(ctx context.Context, dir, branch string) (string, error) {
	return gitOutput(ctx, dir, "switch", "-c", branch)
}

// gitOutput runs one git command and returns its combined output.
// ShowPrefix returns dir's prefix inside its repository ("" at
// the top level, e.g. "sub/dir/" for a nested root) — the piece
// needed to build repo-top-relative paths from project-relative
// ones (S1 baseline probes).
func ShowPrefix(ctx context.Context, dir string) (string, error) {
	out, err := gitOutput(ctx, dir, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ShowFile returns the committed bytes of path (repo-top-relative)
// at ref, exactly as committed (no smudge, no newline games: the
// raw blob via `git show ref:path`).
func ShowFile(ctx context.Context, dir, ref, path string) ([]byte, error) {
	out, err := gitOutput(ctx, dir, "show", ref+":"+path)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// LogFileCommit returns the hash of the newest commit on ref that
// touched path (repo-top-relative), or "" when the path has no
// history on that ref.
func LogFileCommit(ctx context.Context, dir, ref, path string) (string, error) {
	out, err := gitOutput(ctx, dir, "log", "-1", "--format=%H", ref, "--", path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// StatusPorcelain returns the raw `git status --porcelain` output
// scoped to one path (path is interpreted relative to dir, exactly
// like a pathspec typed there). Empty output = the path is clean
// (committed and unmodified, or absent and untracked-free).
func StatusPorcelain(ctx context.Context, dir, path string) (string, error) {
	return gitOutput(ctx, dir, "status", "--porcelain", "--", path)
}

// StatusEntries returns the dirty paths of the work tree at dir
// (tracked modifications + untracked files), parsed from
// `git status --porcelain` and relative to dir. Beacon's own
// reserved runtime directories (.beacon/, .beacon-worktrees/) are
// excluded via pathspec, exactly as in MainDirty: kernel state
// must never make a tree "dirty" by existing. Rename entries
// report the destination path.
func StatusEntries(ctx context.Context, dir string) ([]string, error) {
	out, err := gitOutput(ctx, dir, "status", "--porcelain", "--", ".",
		":(exclude).beacon-worktrees", ":(exclude).beacon")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if i := strings.Index(p, " -> "); i >= 0 {
			p = strings.TrimSpace(p[i+4:])
		}
		p = strings.Trim(p, `"`)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// DiffCheck runs the R11 governance verification `git diff --check`
// over the unstaged and staged diffs of the work tree at dir and
// returns the combined findings text ("" = clean). A git failure
// with no findings text is reported as an error.
func DiffCheck(ctx context.Context, dir string) (string, error) {
	var b strings.Builder
	for _, args := range [][]string{{"diff", "--check"}, {"diff", "--cached", "--check"}} {
		out, err := gitOutput(ctx, dir, args...)
		if strings.TrimSpace(out) != "" {
			b.WriteString(out)
		} else if err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}
