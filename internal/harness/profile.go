package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"beacon-go/internal/core"
)

// Profiles (ImplementHarness surface, AC-045 shape / AC-100): an
// agent profile is a NAME plus a COMMAND TEMPLATE plus the context
// handoff rule — nothing else. The two built-in profiles, codex-style
// and claude-style, are exactly that: generic templates with a
// {context} placeholder. NO vendor-private protocol is hardcoded
// anywhere: pointing a profile at the real claude/codex binary (or
// at any script honouring the same argv shape) is configuration, not
// code, and this package claims no vendor integration it cannot
// demonstrate.
//
// Template placeholders: {context} (stage-context file path),
// {prompt} (iteration prompt), {workdir}, {iteration}, {stage}.
// Substitution is plain string replacement over the template's argv
// elements — the result is still an argv executed without a shell.
type Profile struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Argv        []string `json:"argv,omitempty"` // template; may contain {placeholders}
}

// BuiltinProfiles returns the two built-in profile definitions. A
// built-in profile without a command override resolves through the
// caller's command configuration; on its own its Argv names the
// canonical binary of its style (claude / codex CLIs) so that an
// unconfigured machine fails with executor_not_configured at
// selection time instead of silently falling back to a placeholder.
func BuiltinProfiles() []Profile {
	return []Profile{
		{
			Name:        "codex-style",
			Description: "Codex-style agent CLI: non-interactive exec with the stage-context file passed by path",
			Argv:        []string{"codex", "exec", "--context-file", "{context}", "{prompt}"},
		},
		{
			Name:        "claude-style",
			Description: "Claude-style agent CLI: print mode with the stage-context file passed by path",
			Argv:        []string{"claude", "-p", "--context-file", "{context}", "{prompt}"},
		},
	}
}

// LookupProfile resolves a profile by name from the built-ins.
func LookupProfile(name string) (Profile, bool) {
	for _, p := range BuiltinProfiles() {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// ProfileOverride lets callers (CLI --cmd, config) replace a profile's
// argv template with an explicit command. An empty override keeps the
// built-in template.
func (p Profile) WithCommand(argv []string) Profile {
	if len(argv) > 0 {
		p.Argv = argv
	}
	return p
}

// ResolveTemplate substitutes the placeholders in one argv element.
func ResolveTemplate(tpl string, vars map[string]string) string {
	return strings.NewReplacer(
		"{context}", vars["context"],
		"{prompt}", vars["prompt"],
		"{workdir}", vars["workdir"],
		"{iteration}", vars["iteration"],
		"{stage}", vars["stage"],
	).Replace(tpl)
}

// StageContext is the stage-context document handed to a profile
// command: where the requirement package lives, the stage plan, the
// admission verdict when it was packed, and the rollback rules. It is
// written to a file (stage-context.json under the run's artifact
// dir) and the PATH travels to the command — as the {context}
// argument and as $BEACON_CONTEXT_FILE.
type StageContext struct {
	RunID       string   `json:"run_id"`
	FeatureID   string   `json:"feature_id"`
	FeatureSlug string   `json:"feature_slug"`
	Stage       string   `json:"stage"`
	Lane        string   `json:"lane,omitempty"`
	Iteration   int      `json:"iteration"`
	Workdir     string   `json:"workdir"`
	PackageRoot string   `json:"package_root,omitempty"`
	TruthPath   string   `json:"truth_path,omitempty"`
	TestsPath   string   `json:"tests_path,omitempty"`
	StagePlan   any      `json:"stage_plan"`
	Admission   any      `json:"admission,omitempty"`
	Acceptance  []string `json:"acceptance,omitempty"`
	Rollback    []string `json:"rollback,omitempty"`
	Prompt      string   `json:"prompt"`
}

// WriteStageContext writes the context document as indented JSON and
// returns its absolute path.
func WriteStageContext(dir string, sc StageContext) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "stage-context.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ProfileBackend runs a profile's command template as an
// AgentBackend. It IS the harness execution layer — profile argv
// resolved against a concrete stage context, executed via the
// subprocess machinery, with the working directory aligned to the
// run's bound worktree (AC-099/AC-100: the harness runs INSIDE the
// worktree and hands over a stage-context file; a plain
// SubprocessBackend does neither, so the two are not
// interchangeable and neither may impersonate the other).
type ProfileBackend struct {
	profile Profile
	ctxFile string
	prompt  string
	timeout time.Duration
}

// NewProfileBackend binds a profile to one stage-context file and
// iteration prompt.
func NewProfileBackend(p Profile, contextFile, prompt string, timeout time.Duration) *ProfileBackend {
	return &ProfileBackend{profile: p, ctxFile: contextFile, prompt: prompt, timeout: timeout}
}

// Name implements AgentBackend: subprocess:<profile-name>.
func (b *ProfileBackend) Name() string { return "subprocess:" + b.profile.Name }

// Profile returns the bound profile.
func (b *ProfileBackend) Profile() Profile { return b.profile }

// HealthCheck probes the template's command (placeholders stripped
// from the probe: only argv[0] matters).
func (b *ProfileBackend) HealthCheck(ctx context.Context) HealthStatus {
	if len(b.profile.Argv) == 0 {
		return HealthStatus{Backend: b.Name(), OK: false, Detail: core.ReasonExecutorProfileIncomplete}
	}
	probe := NewSubprocessBackend([]string{b.profile.Argv[0]}, b.timeout)
	hs := probe.HealthCheck(ctx)
	hs.Backend = b.Name()
	return hs
}

// Abort mirrors the subprocess backend: iterations are synchronous,
// context cancellation kills the child.
func (b *ProfileBackend) Abort(ctx context.Context) error { return nil }

// Iterate resolves the template against the request and runs it.
// A template that references {context} without a context file is a
// configuration error (executor_profile_incomplete) — the profile is
// never silently run context-less.
func (b *ProfileBackend) Iterate(ctx context.Context, req IterationRequest) (IterationResult, error) {
	if len(b.profile.Argv) == 0 {
		return IterationResult{}, core.NewError(core.ReasonExecutorProfileIncomplete,
			"harness.ProfileBackend.Iterate", "profile "+b.profile.Name+" has no command template")
	}
	joined := strings.Join(b.profile.Argv, " ")
	if strings.Contains(joined, "{context}") && b.ctxFile == "" {
		return IterationResult{}, core.NewError(core.ReasonExecutorProfileIncomplete,
			"harness.ProfileBackend.Iterate", "profile "+b.profile.Name+" template needs {context} but no stage-context file was built")
	}
	vars := map[string]string{
		"context":   b.ctxFile,
		"prompt":    b.prompt,
		"workdir":   req.Workdir,
		"iteration": strconv.Itoa(req.Iteration),
		"stage":     req.Stage,
	}
	argv := make([]string, len(b.profile.Argv))
	for i, tpl := range b.profile.Argv {
		argv[i] = ResolveTemplate(tpl, vars)
	}
	sub := NewSubprocessBackend(argv, b.timeout)
	sub.backendName = b.Name()
	req2 := req
	req2.ContextPath = b.ctxFile
	return sub.Iterate(ctx, req2)
}
