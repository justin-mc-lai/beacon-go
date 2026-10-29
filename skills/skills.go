// Package skills embeds the public ability skills into the binary
// ("edit as files, ship as binary"). Progressive disclosure lives
// one level down: the SKILL.md is the always-loaded surface;
// references/ files are loaded on intent match — by the host when
// the package is materialized into its skill directory (see
// Install), or through the CLI's references face.
//
// The set is the Scheme A 1+6 public surface of the Python package
// (skills/beacon/): the goal facade plus plan / truth / design /
// implement / qa / release. plan joined in R2c (parity audit H9):
// before that the Go set silently replaced plan with goal and
// shipped no references at all, so a host had nothing to discover.
package skills

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed goal plan truth design implement qa release
var files embed.FS

// Skill is one embedded public skill.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body,omitempty"`
}

// RefFile is one references/ file of a skill package.
type RefFile struct {
	Path string `json:"path"` // package-relative, e.g. references/core.md
	Body string `json:"body,omitempty"`
}

var order = []string{"goal", "plan", "truth", "design", "implement", "qa", "release"}

// IDs returns the public skill ids in pipeline order.
func IDs() []string { return append([]string{}, order...) }

// List returns the public skills in pipeline order (bodies omitted).
func List() []Skill {
	out := make([]Skill, 0, len(order))
	for _, id := range order {
		if s, ok := Get(id); ok {
			s.Body = ""
			out = append(out, s)
		}
	}
	return out
}

// Get returns one skill including its SKILL.md body.
func Get(id string) (Skill, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	raw, err := files.ReadFile(id + "/SKILL.md")
	if err != nil {
		return Skill{}, false
	}
	s := Skill{ID: id, Body: string(raw)}
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "name:"):
			s.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		case strings.HasPrefix(line, "description:"):
			s.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return s, true
}

// References returns the skill's references/ files (bodies
// included), sorted by path. Empty when the skill is unknown.
func References(id string) []RefFile {
	id = strings.ToLower(strings.TrimSpace(id))
	base := id + "/references"
	var out []RefFile
	_ = fs.WalkDir(files, base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		raw, rerr := files.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(id, path)
		out = append(out, RefFile{Path: filepath.ToSlash(rel), Body: string(raw)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// InstallDirName is the host-side directory name of one skill:
// hosts discover flat beacon-<id> directories (Scheme A 1+6, the
// Python agent_surface layout).
func InstallDirName(id string) string { return "beacon-" + id }

// EvalCheck is one machine-verifiable assertion about the package
// bytes. It answers "does the skill actually state the rule this case
// expects?" — never "did a model behave well". Behavioural grading
// needs a model run and stays a human judgement.
type EvalCheck struct {
	Kind string `json:"kind"` // contains | references_contains | min_examples
	Text string `json:"text,omitempty"`
	Min  int    `json:"min,omitempty"`
}

// EvalCase is one test prompt plus its expectations.
type EvalCase struct {
	ID           string      `json:"id"`
	Prompt       string      `json:"prompt"`
	Expectations []string    `json:"expectations"`
	Checks       []EvalCheck `json:"checks"`
}

// EvalSet is one package's evals/evals.json.
type EvalSet struct {
	Schema string     `json:"schema"`
	Skill  string     `json:"skill"`
	Cases  []EvalCase `json:"cases"`
}

// Evals reads one skill's evals/evals.json.
func Evals(id string) (EvalSet, bool) {
	raw, err := files.ReadFile(id + "/evals/evals.json")
	if err != nil {
		return EvalSet{}, false
	}
	var set EvalSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return EvalSet{}, false
	}
	return set, true
}

// Examples returns the skill's examples/ files (bodies included).
func Examples(id string) []RefFile {
	return walkFiles(id, "examples")
}

// packageFiles lists every embedded file of one skill package,
// package-relative and sorted: SKILL.md, references/**, examples/**,
// evals/**. The installer copies exactly this list, so a host gets the
// same bytes the binary holds.
func packageFiles(id string) []string {
	var out []string
	_ = fs.WalkDir(files, id, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(id, path)
		if rerr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

// walkFiles reads one subdirectory of a package.
func walkFiles(id, sub string) []RefFile {
	base := id + "/" + sub
	var out []RefFile
	_ = fs.WalkDir(files, base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		raw, rerr := files.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(id, path)
		out = append(out, RefFile{Path: filepath.ToSlash(rel), Body: string(raw)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ConflictError reports that Install refused to overwrite existing
// skill directories without force. Nothing was written.
type ConflictError struct {
	Paths []string
}

func (e *ConflictError) Error() string {
	return "skill install conflicts (rerun with --force to overwrite): " + strings.Join(e.Paths, ", ")
}

// InstallResult is one Install outcome.
type InstallResult struct {
	TargetDir string   `json:"target_dir"`
	Skills    []string `json:"skills"`
	Files     []string `json:"files"` // target-relative paths written
	Forced    bool     `json:"forced,omitempty"`
}

// PlannedFiles lists every file Install would write for one target
// dir, as target-relative paths — the pre-write display surface.
func PlannedFiles(targetDir string) []string {
	var out []string
	for _, id := range order {
		base := filepath.Join(targetDir, InstallDirName(id))
		for _, rel := range packageFiles(id) {
			out = append(out, filepath.Join(base, filepath.FromSlash(rel)))
		}
	}
	return out
}

// Install materializes the embedded skill packages into a host
// skill directory: <target>/beacon-<id>/ with SKILL.md, references/,
// examples/ and evals/. Existing beacon-<id> directories are a
// conflict and refuse the WHOLE install unless force is set (no
// half-written skill set). Unrelated directories in the target are
// never touched.
func Install(targetDir string, force bool) (InstallResult, error) {
	if strings.TrimSpace(targetDir) == "" {
		return InstallResult{}, fmt.Errorf("skills.Install: empty target dir")
	}
	var conflicts []string
	for _, id := range order {
		dest := filepath.Join(targetDir, InstallDirName(id))
		if info, err := os.Stat(dest); err == nil && info.IsDir() && !force {
			conflicts = append(conflicts, dest)
		}
	}
	if len(conflicts) > 0 {
		return InstallResult{}, &ConflictError{Paths: conflicts}
	}
	res := InstallResult{TargetDir: targetDir, Forced: force}
	for _, id := range order {
		base := filepath.Join(targetDir, InstallDirName(id))
		files := packageFiles(id)
		if len(files) == 0 {
			return InstallResult{}, fmt.Errorf("skills.Install: embedded skill %s missing", id)
		}
		for _, rel := range files {
			raw, err := readPackageFile(id, rel)
			if err != nil {
				return InstallResult{}, err
			}
			dest := filepath.Join(base, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return InstallResult{}, err
			}
			if err := os.WriteFile(dest, raw, 0o644); err != nil {
				return InstallResult{}, err
			}
			res.Files = append(res.Files, dest)
		}
		res.Skills = append(res.Skills, id)
	}
	return res, nil
}

func readPackageFile(id, rel string) ([]byte, error) {
	return files.ReadFile(id + "/" + rel)
}
