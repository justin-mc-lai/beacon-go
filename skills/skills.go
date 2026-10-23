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
		out = append(out, filepath.Join(base, "SKILL.md"))
		for _, ref := range References(id) {
			out = append(out, filepath.Join(base, filepath.FromSlash(ref.Path)))
		}
	}
	return out
}

// Install materializes the embedded skill packages into a host
// skill directory: <target>/beacon-<id>/SKILL.md plus its
// references/. Existing beacon-<id> directories are a conflict and
// refuse the WHOLE install unless force is set (no half-written
// skill set). Unrelated directories in the target are never touched.
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
		s, ok := Get(id)
		if !ok {
			return InstallResult{}, fmt.Errorf("skills.Install: embedded skill %s missing", id)
		}
		base := filepath.Join(targetDir, InstallDirName(id))
		if err := os.MkdirAll(base, 0o755); err != nil {
			return InstallResult{}, err
		}
		skillPath := filepath.Join(base, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(s.Body), 0o644); err != nil {
			return InstallResult{}, err
		}
		res.Skills = append(res.Skills, id)
		res.Files = append(res.Files, skillPath)
		for _, ref := range References(id) {
			dest := filepath.Join(base, filepath.FromSlash(ref.Path))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return InstallResult{}, err
			}
			if err := os.WriteFile(dest, []byte(ref.Body), 0o644); err != nil {
				return InstallResult{}, err
			}
			res.Files = append(res.Files, dest)
		}
	}
	return res, nil
}
