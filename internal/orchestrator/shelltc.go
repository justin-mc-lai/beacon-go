// Project-relative shell TC resolution (kernel-autotest-shell-tc):
// tests.md command cells may name a script inside the project (for
// example `scripts/check.sh`). exec.LookPath resolves a bare relative
// path against the CLI's working directory and demands an execute
// bit, so such a TC never reached its script. This layer recognises
// a project-root script candidate, picks its interpreter, and leaves
// every other command (explicit `sh ...`, PATH commands, absolute
// paths outside the project) on the existing exec.LookPath path.
package orchestrator

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
)

// resolveShellTC rewrites argv when argv[0] is an existing regular
// file under root whose type is a supported shell script:
//
//	.sh  -> /bin/sh
//	.zsh -> /bin/zsh
//	no extension -> read the first line shebang (sh/zsh, also via env)
//
// The script path is resolved against root, never the process cwd,
// and stays one argv element (spaces included). A missing interpreter
// is a typed error: callers surface it instead of falling back to PATH.
func resolveShellTC(root string, argv []string) ([]string, error) {
	if len(argv) == 0 || argv[0] == "" || !strings.ContainsRune(argv[0], filepath.Separator) {
		return argv, nil
	}
	script := argv[0]
	if !filepath.IsAbs(script) {
		script = filepath.Join(root, script)
	}
	script = filepath.Clean(script)
	if !withinRoot(root, script) {
		return argv, nil
	}
	info, err := os.Stat(script)
	if err != nil || !info.Mode().IsRegular() {
		return argv, nil
	}
	interp, ok := shellInterpreter(script)
	if !ok {
		return argv, nil
	}
	if info, err := os.Stat(interp); err != nil || info.IsDir() {
		return nil, core.NewError(core.ReasonExecutorCommandMissing,
			"resolveShellTC", "interpreter not found for "+script+": "+interp)
	}
	out := make([]string, 0, len(argv)+1)
	out = append(out, interp, script)
	out = append(out, argv[1:]...)
	return out, nil
}

// withinRoot reports whether path stays inside root after cleaning.
func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// shellInterpreter maps one script file to its interpreter. Only the
// documented interpreters are supported; anything else returns false
// so the caller keeps the original PATH lookup semantics.
func shellInterpreter(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sh":
		return "/bin/sh", true
	case ".zsh":
		return "/bin/zsh", true
	}
	if filepath.Ext(path) != "" {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "#!") {
		return "", false
	}
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "#!")))
	if len(fields) == 0 {
		return "", false
	}
	name := filepath.Base(fields[0])
	if name == "env" && len(fields) > 1 {
		name = filepath.Base(fields[1])
	}
	switch name {
	case "sh":
		return "/bin/sh", true
	case "zsh":
		return "/bin/zsh", true
	}
	return "", false
}
