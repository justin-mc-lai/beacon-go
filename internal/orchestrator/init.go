// Project init (R2c, parity audit M11): assembling a new project's
// context, ported from Python commands/init.py. `projects register`
// alone only wrote a DB row; init writes the context file set the
// Python command writes — the docs tree skeleton for the version,
// the onboarding state, and the host context stubs — and registers
// the project + version in the store as the Go-side counterpart of
// "the project exists now".
//
// File contents marked "Python verbatim" are byte-identical to
// init.py's templates. Two adaptations are deliberate and recorded
// in the batch report: (1) global-boundaries.md — Python syncs the
// toolkit's canonical text, the Go kernel has no canonical text to
// sync, so it writes a project-owned stub; (2) the rich CLAUDE.md /
// AGENTS.md context-guidance blocks come from Python's
// context_files.setup_context_files — the Go face writes Python's
// own fallback stubs ("# CLAUDE\n\n<!-- BEACON-MANAGED -->\n").
//
// Write discipline mirrors _write_if_apply: an existing file is
// never overwritten unless --force (then it is "refreshed"); every
// file lands in the actions list as created/unchanged/planned.
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// InitOptions carries one InitProject invocation.
type InitOptions struct {
	Version        string // docs version; "" = the registered current version
	ProjectMode    string // greenfield | takeover
	RuntimeVersion string // caller's build version, recorded in the files
	Name           string
	DryRun         bool
	Force          bool
	WithClaude     bool
	WithAgents     bool
}

// InitAction is one line of the init report's action list.
type InitAction struct {
	Scope  string `json:"scope"`  // docs | base | state | store
	Action string `json:"action"` // created | unchanged | planned | refreshed | registered
	Path   string `json:"path"`
}

// InitReport is the init outcome (also written to
// .beacon/state/init-report.json on apply).
type InitReport struct {
	SchemaVersion  string       `json:"schema_version"`
	Command        string       `json:"command"`
	ProjectRoot    string       `json:"project_root"`
	DocsVersion    string       `json:"docs_version"`
	RuntimeVersion string       `json:"runtime_version"`
	ProjectMode    string       `json:"project_mode"`
	DryRun         bool         `json:"dry_run"`
	Status         string       `json:"status"`
	Actions        []InitAction `json:"actions"`
	ReportPath     string       `json:"report_path,omitempty"`
}

type initFile struct {
	scope   string
	rel     string
	content string
}

// InitProject assembles the project context at root.
func (e *Engine) InitProject(projectRoot string, opts InitOptions) (InitReport, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.ProjectMode))
	if mode == "" {
		mode = "greenfield"
	}
	if mode != "greenfield" && mode != "takeover" {
		return InitReport{}, core.NewError(core.ReasonProjectModeInvalid,
			"projects.Init", "project_mode must be greenfield|takeover, got "+mode)
	}
	root := store.ResolveRootPath(projectRoot)
	apply := !opts.DryRun

	// Store side: register the project (idempotent) and resolve /
	// record the docs version. In dry-run nothing is written; the
	// version must then come from the flags or an existing record.
	proj, err := e.store.Projects().GetByRootPath(root)
	registered := false
	if errors.Is(err, sql.ErrNoRows) {
		if apply {
			name := opts.Name
			if name == "" {
				name = filepath.Base(root)
			}
			proj, err = e.store.Projects().Register(store.Project{RootPath: root, Name: name})
			if err != nil {
				return InitReport{}, err
			}
			registered = true
		}
	} else if err != nil {
		return InitReport{}, err
	}
	version := opts.Version
	if apply && !registered && proj.ID == "" {
		// unreachable in practice; keep the zero value out of the files
		return InitReport{}, core.NewError(ReasonProjectNotRegistered, "projects.Init", "project resolution failed: "+root)
	}
	if apply {
		if version != "" {
			if err := e.ensureVersionRecorded(proj, version); err != nil {
				return InitReport{}, err
			}
		} else {
			cur, cerr := e.store.Projects().CurrentVersion(proj.ID)
			if cerr != nil || cur.Version == "" {
				return InitReport{}, core.NewError(ReasonTruthVersionMissing,
					"projects.Init", "no docs version given and none recorded for "+root+" (pass --version)")
			}
			version = cur.Version
		}
	} else if version == "" {
		if proj.ID != "" {
			if cur, cerr := e.store.Projects().CurrentVersion(proj.ID); cerr == nil {
				version = cur.Version
			}
		}
		if version == "" {
			return InitReport{}, core.NewError(ReasonTruthVersionMissing,
				"projects.Init", "no docs version given and none recorded for "+root+" (pass --version)")
		}
	}

	rtVersion := opts.RuntimeVersion
	vroot := filepath.Join("docs", "beacon", version)
	exec := filepath.Join(vroot, "execution")
	files := []initFile{
		{"docs", filepath.Join(vroot, "SUMMARY.md"), "# Beacon Summary\n\n- status: draft\n"},
		{"docs", filepath.Join(exec, "boundaries.md"), "# Boundaries\n\n- status: draft\n"},
		{"docs", filepath.Join(exec, "index.md"), "# Execution Index\n\n- status: draft\n"},
		{"docs", filepath.Join(exec, "architecture-blueprint.md"),
			"# Architecture Blueprint\n\n- status: draft\n- project_mode: " + mode + "\n\n## Scope\n- system positioning\n- module/service map\n- data flow\n- boundaries\n"},
		{"docs", filepath.Join(exec, "version-evolution-blueprint.md"),
			"# Version Evolution Blueprint\n\n- status: draft\n- version: " + version + "\n- runtime_version: " + rtVersion + "\n- project_mode: " + mode + "\n\n## Capability Map\n- planned\n- implemented\n- deprecated\n"},
		{"docs", filepath.Join(exec, "implementation-archive-index.md"),
			"# Implementation Archive Index\n\n- status: draft\n- technical implementation archives will be listed here by feature.\n"},
		{"docs", filepath.Join(exec, "project-lifecycle-onboarding.md"),
			"# Project Lifecycle Onboarding\n\n- docs_version: `" + version + "`\n- runtime_version: `" + rtVersion + "`\n- project_mode: `" + mode + "`\n- lifecycle_adoption_level: `2`\n- recommended reading order:\n  1. SUMMARY.md\n  2. execution/index.md\n  3. execution/architecture-blueprint.md\n  4. prd/ user-story/ qa/test-cases/\n  5. .machine/\n"},
		{"docs", filepath.Join(vroot, "release", version+".md"), "# Release: " + version + "\n"},
		{"docs", filepath.Join("docs", "beacon", "global-boundaries.md"),
			"# Global Boundaries\n\n- status: draft\n- project-owned: record the cross-version constraints for this project here.\n"},
	}
	if mode == "takeover" {
		files = append(files,
			initFile{"docs", filepath.Join(exec, "current-state-service-map.md"),
				"# Current-State Service Map\n\n- status: draft\n- capture existing service ownership, contracts, and dependencies before delivery changes.\n"},
			initFile{"docs", filepath.Join(exec, "refactor-boundaries.md"),
				"# Refactor Boundaries\n\n- status: draft\n- record protected boundaries, risky modules, and allowed change slices for takeover work.\n"},
		)
	}
	if opts.WithClaude {
		files = append(files, initFile{"base", "CLAUDE.md", "# CLAUDE\n\n<!-- BEACON-MANAGED -->\n"})
	}
	if opts.WithAgents {
		files = append(files, initFile{"base", "AGENTS.md", "# AGENTS\n\n<!-- BEACON-MANAGED -->\n"})
	}

	report := InitReport{
		SchemaVersion: "1.0", Command: "projects init", ProjectRoot: root,
		DocsVersion: version, RuntimeVersion: rtVersion, ProjectMode: mode,
		DryRun: opts.DryRun, Status: "completed",
	}
	if registered {
		report.Actions = append(report.Actions, InitAction{Scope: "store", Action: "registered", Path: root})
	}
	dirs := []string{vroot, exec, filepath.Join(vroot, "features"), filepath.Join(vroot, "release"), ".beacon", filepath.Join(".beacon", "state")}
	for _, d := range dirs {
		if apply {
			if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
				return InitReport{}, err
			}
			report.Actions = append(report.Actions, InitAction{Scope: "docs", Action: "created", Path: filepath.ToSlash(d)})
		} else {
			report.Actions = append(report.Actions, InitAction{Scope: "docs", Action: "planned", Path: filepath.ToSlash(d)})
		}
	}
	for _, f := range files {
		action := writeInitFile(root, f, apply, opts.Force)
		report.Actions = append(report.Actions, InitAction{Scope: f.scope, Action: action, Path: filepath.ToSlash(f.rel)})
	}
	// Onboarding state (Python: .beacon/state/project-onboarding.json,
	// written unconditionally on apply).
	onboarding := map[string]any{
		"schema_version":                 "1.0",
		"project_mode":                   mode,
		"lifecycle_adoption_level":       2,
		"docs_version":                   version,
		"runtime_version":                rtVersion,
		"architecture_blueprint_ready":   true,
		"version_blueprint_ready":        true,
		"implementation_archive_enabled": false,
		"qa_managed":                     false,
		"gate_managed":                   false,
		"takeover_baseline_status":       "not_applicable",
		"takeover_baseline_frozen_at":    "",
		"next_recommended_stage":         "think",
		"updated_at":                     time.Now().UTC().Format(time.RFC3339),
	}
	if mode == "takeover" {
		onboarding["takeover_baseline_status"] = "draft"
		onboarding["next_recommended_stage"] = "architecture-current-state"
	}
	if apply {
		raw, merr := json.MarshalIndent(onboarding, "", "  ")
		if merr != nil {
			return InitReport{}, merr
		}
		obPath := filepath.Join(root, ".beacon", "state", "project-onboarding.json")
		if err := os.WriteFile(obPath, append(raw, '\n'), 0o644); err != nil {
			return InitReport{}, err
		}
	}
	report.Actions = append(report.Actions, InitAction{Scope: "state",
		Action: map[bool]string{true: "created", false: "planned"}[apply],
		Path:   ".beacon/state/project-onboarding.json"})
	if apply {
		raw, merr := json.MarshalIndent(report, "", "  ")
		if merr != nil {
			return InitReport{}, merr
		}
		rp := filepath.Join(root, ".beacon", "state", "init-report.json")
		if err := os.WriteFile(rp, append(raw, '\n'), 0o644); err != nil {
			return InitReport{}, err
		}
		report.ReportPath = ".beacon/state/init-report.json"
	}
	return report, nil
}

// ensureVersionRecorded mirrors the projects register --version
// bookkeeping: record once, mark current.
func (e *Engine) ensureVersionRecorded(proj store.Project, version string) error {
	versions, err := e.store.Projects().ListVersions(proj.ID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if v.Version == version {
			if !v.IsCurrent {
				return e.store.Projects().SetCurrent(v.ID)
			}
			return nil
		}
	}
	_, err = e.store.Projects().AddVersion(store.ProjectVersion{
		ProjectID: proj.ID, Version: version,
		DocsRoot: filepath.Join("docs", "beacon", version), IsCurrent: true,
	})
	return err
}

// writeInitFile applies _write_if_apply semantics.
func writeInitFile(root string, f initFile, apply, force bool) string {
	abs := filepath.Join(root, f.rel)
	if _, err := os.Stat(abs); err == nil {
		if !apply {
			return "planned"
		}
		if !force {
			return "unchanged"
		}
		_ = os.WriteFile(abs, []byte(f.content), 0o644)
		return "refreshed"
	}
	if !apply {
		return "planned"
	}
	_ = os.MkdirAll(filepath.Dir(abs), 0o755)
	_ = os.WriteFile(abs, []byte(f.content), 0o644)
	return "created"
}
