package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Host is one agent host that discovers skills from a directory on
// disk. Beacon ships one table so the CLI install face and the
// post-release hook can never disagree about where a host looks.
type Host struct {
	Name      string   `json:"name"`
	Dir       string   `json:"dir"`
	Exists    bool     `json:"exists"`
	Installed []string `json:"installed"` // beacon-* skill directories found
}

// hostSpecs is the ordered host table. Paths are relative to $HOME.
//
// workbuddy is first on purpose: it is the host most operators run
// today, and it was missing entirely from the earlier claude|codex|
// generic list — which is why `beacon skills install` wrote skills to
// a directory nothing was reading.
var hostSpecs = []struct {
	name string
	rel  string
}{
	{"workbuddy", filepath.Join(".workbuddy-ai", "skills")},
	{"claude", filepath.Join(".claude", "skills")},
	{"codex", filepath.Join(".codex", "skills")},
	{"agents", filepath.Join(".agents", "skills")},
	{"cursor", filepath.Join(".cursor", "skills")},
	{"windsurf", filepath.Join(".windsurf", "skills")},
	{"trae", filepath.Join(".trae", "skills")},
}

// canonicalHost normalises aliases. "generic" stays accepted because
// it was the previous default and scripts may still pass it.
func canonicalHost(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "generic", "agent", "agents":
		return "agents"
	case "claude-code", "claudecode":
		return "claude"
	case "workbuddy-ai", "work-buddy":
		return "workbuddy"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

// HostNames lists the supported host names in table order.
func HostNames() []string {
	out := make([]string, 0, len(hostSpecs))
	for _, h := range hostSpecs {
		out = append(out, h.name)
	}
	return out
}

// HostDir resolves one host name to its absolute skill directory.
func HostDir(name, home string) (string, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	want := canonicalHost(name)
	for _, h := range hostSpecs {
		if h.name == want {
			return filepath.Join(home, h.rel), nil
		}
	}
	return "", fmt.Errorf("unknown host %q (want one of: %s; or pass --dir)",
		name, strings.Join(HostNames(), "|"))
}

// HostDirs returns the whole table with existence and installed-set
// information — the `beacon skills hosts` view and the installer's
// auto-detection both read this.
func HostDirs(home string) ([]Host, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	out := make([]Host, 0, len(hostSpecs))
	for _, h := range hostSpecs {
		dir := filepath.Join(home, h.rel)
		host := Host{Name: h.name, Dir: dir, Installed: installedSkills(dir)}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			host.Exists = true
		}
		out = append(out, host)
	}
	return out, nil
}

// DetectHost picks the first existing host directory in table order.
// It fails closed rather than guessing a directory nothing reads.
func DetectHost(home string) (Host, error) {
	hosts, err := HostDirs(home)
	if err != nil {
		return Host{}, err
	}
	for _, h := range hosts {
		if h.Exists {
			return h, nil
		}
	}
	names := make([]string, 0, len(hosts))
	for _, h := range hosts {
		names = append(names, h.Name+" ("+h.Dir+")")
	}
	return Host{}, fmt.Errorf("no known host skill directory exists; pass --host or --dir. Candidates: %s",
		strings.Join(names, ", "))
}

func installedSkills(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "beacon-") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}
