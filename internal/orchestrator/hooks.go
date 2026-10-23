// Project lifecycle hooks (R2c, parity audit M12): the hook
// contract framework of Python commands/hooks.py +
// utils/project_hooks.py, plus the release-completion trigger.
//
// Contracts are markdown files with frontmatter. Python's root is
// docs/beacon/hooks/<hook_id>.md; the Go face ALSO reads
// .beacon/hooks/<hook_id>.md (the project-local config dir the
// batch brief names), with docs/beacon/hooks winning an id clash.
// State/evidence lives where Python puts it: run records under
// .beacon/state/hooks/<id>/, the latest read model under
// docs/beacon/.machine/hooks/<id>.latest.json.
//
// Admission is Python's validate_hook_contract, diagnostic codes
// verbatim: hook_code in {post_implement_pre_qa, qa_diagnostics,
// post_qa_pre_release, post_release}, trigger_policy in
// {dry_run_only, manual_confirmed, auto_allowed}, failure_route in
// {implement, qa, release, change, blocked}, a skill or a
// command_template is required, produces_evidence is required, the
// body must carry Purpose/Admission/Evidence/Failure sections, and
// a hook may never claim lifecycle-truth authority
// (hook_authority_bypass).
//
// Run gating is Python's: dry-run writes planned evidence only;
// dry_run_only refuses a real run; manual_confirmed needs
// --confirm; and even auto_allowed refuses an unconfirmed run
// (hidden_auto_run_not_allowed). ONE deliberate strengthening,
// recorded in the batch report: Python validates command_template
// but never executes it (only the builtin post-release install
// does real work). The Go face executes command_template through
// the harness (argv only, never a shell) because this batch's
// mandate is a working post-release trigger. The builtin
// beacon-post-release-local-install executes its Go counterpart:
// installing the embedded skills into the host skill directories
// (claude + agents roots), which is the core of Python's followup.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/skills"
)

// PostReleaseInstallHookID is project_hooks'
// BEACON_POST_RELEASE_LOCAL_INSTALL_HOOK_ID verbatim.
const PostReleaseInstallHookID = "beacon-post-release-local-install"

// EventPostReleaseHooks is appended to a run when completion
// triggers its post_release hooks.
const EventPostReleaseHooks = "post_release_hooks_triggered"

// Allowed hook vocabularies (project_hooks.py verbatim).
var (
	allowedHookCodes = map[string]bool{
		"post_implement_pre_qa": true, "qa_diagnostics": true,
		"post_qa_pre_release": true, "post_release": true,
	}
	allowedTriggerPolicies = map[string]bool{
		"dry_run_only": true, "manual_confirmed": true, "auto_allowed": true,
	}
	allowedFailureRoutes = map[string]bool{
		"implement": true, "qa": true, "release": true, "change": true, "blocked": true,
	}
	hookAuthorityKeywords = []string{
		"prd truth", "user-story truth", "test-case truth", "qa verdict",
		"release verdict", "gate verdict", "freeze state", "requirement truth",
	}
)

// HookContract is one parsed hook contract file.
type HookContract struct {
	HookID           string   `json:"hook_id"`
	Path             string   `json:"path"` // project-relative
	Exists           bool     `json:"exists"`
	HookCode         string   `json:"hook_code"`
	Enabled          bool     `json:"enabled"`
	TriggerPolicy    string   `json:"trigger_policy"`
	FailureRoute     string   `json:"failure_route"`
	Skill            string   `json:"skill,omitempty"`
	CommandTemplate  string   `json:"command_template,omitempty"`
	ProducesEvidence []string `json:"produces_evidence,omitempty"`
	FeaturePattern   string   `json:"feature_pattern,omitempty"`
	ParseErrors      []string `json:"-"`
	body             string
	authorityText    string
	// declaredID is the hook_id exactly as written in frontmatter
	// ("" when absent). Python compares front.get("hook_id")
	// against the file stem, so an ABSENT hook_id is a mismatch
	// too — the requested id must not quietly stand in for it.
	declaredID string
}

// HookDiagnostic is one admission finding (Python diagnostics).
type HookDiagnostic struct {
	ReasonCode string `json:"reason_code"`
	Message    string `json:"message"`
}

// HookAdmission is validate_hook_contract's payload.
type HookAdmission struct {
	SchemaVersion string           `json:"schema_version"`
	HookID        string           `json:"hook_id"`
	HookCode      string           `json:"hook_code"`
	Feature       string           `json:"feature,omitempty"`
	Path          string           `json:"path"`
	Status        string           `json:"status"` // pass | blocked
	Blocked       bool             `json:"blocked"`
	ReasonCodes   []string         `json:"reason_codes,omitempty"`
	Diagnostics   []HookDiagnostic `json:"diagnostics,omitempty"`
	Contract      HookContract     `json:"contract"`
}

// hookRoots are the contract locations in precedence order.
func hookRoots(root string) []string {
	return []string{
		filepath.Join(root, "docs", "beacon", "hooks"),
		filepath.Join(root, ".beacon", "hooks"),
	}
}

func hookContractPath(root, hookID string) (string, bool) {
	for _, dir := range hookRoots(root) {
		p := filepath.Join(dir, hookID+".md")
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return filepath.Join(root, "docs", "beacon", "hooks", hookID+".md"), false
}

// parseHookFrontmatter parses the small YAML subset hook contracts
// use: top-level scalars, booleans, inline [a, b] lists and block
// lists. Returns (fields, body, parseErrors).
func parseHookFrontmatter(text string) (map[string]any, string, []string) {
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return map[string]any{}, text, []string{"frontmatter_missing"}
	}
	rest := strings.SplitN(text, "\n", 2)[1]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return map[string]any{}, text, []string{"frontmatter_unclosed"}
	}
	raw := rest[:end]
	body := ""
	if tail := rest[end+1:]; strings.HasPrefix(tail, "---") {
		if idx := strings.Index(tail, "\n"); idx >= 0 {
			body = tail[idx+1:]
		}
	}
	fields := map[string]any{}
	var listKey string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			if listKey == "" {
				return map[string]any{}, body, []string{"frontmatter_parse_failed"}
			}
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			fields[listKey] = append(fields[listKey].([]string), strings.Trim(item, `"'`))
			continue
		}
		key, val, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// Indented continuations of a scalar are folded in
			// loosely: only top-level keys are contract fields.
			if !found {
				return map[string]any{}, body, []string{"frontmatter_parse_failed"}
			}
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		listKey = ""
		switch {
		case val == "":
			fields[key] = []string{}
			listKey = key
		case strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]"):
			var items []string
			for _, it := range strings.Split(strings.Trim(val, "[]"), ",") {
				if it = strings.Trim(strings.TrimSpace(it), `"'`); it != "" {
					items = append(items, it)
				}
			}
			fields[key] = items
		case val == "true" || val == "True":
			fields[key] = true
		case val == "false" || val == "False":
			fields[key] = false
		default:
			fields[key] = strings.Trim(val, `"'`)
		}
	}
	return fields, body, nil
}

func hookFieldString(fields map[string]any, key string) string {
	if v, ok := fields[key].(string); ok {
		return v
	}
	return ""
}

func hookFieldList(fields map[string]any, key string) []string {
	switch v := fields[key].(type) {
	case []string:
		return v
	case string:
		if strings.TrimSpace(v) != "" {
			return []string{v}
		}
	}
	return nil
}

// LoadHookContract reads one contract (missing file -> Exists=false
// with the hook_file_missing parse error, as Python).
func LoadHookContract(root, hookID string) HookContract {
	abs, found := hookContractPath(root, hookID)
	rel, _ := filepath.Rel(root, abs)
	c := HookContract{HookID: hookID, Path: filepath.ToSlash(rel)}
	if !found {
		c.ParseErrors = []string{core.ReasonHookFileMissing}
		return c
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		c.ParseErrors = []string{core.ReasonHookFileMissing}
		return c
	}
	c.Exists = true
	fields, body, perrs := parseHookFrontmatter(string(raw))
	c.ParseErrors = perrs
	c.body = body
	if v := hookFieldString(fields, "hook_id"); v != "" {
		c.HookID = v
		c.declaredID = v
	}
	c.HookCode = hookFieldString(fields, "hook_code")
	if v, ok := fields["enabled"].(bool); ok {
		c.Enabled = v
	}
	c.TriggerPolicy = hookFieldString(fields, "trigger_policy")
	c.FailureRoute = hookFieldString(fields, "failure_route")
	c.Skill = hookFieldString(fields, "skill")
	c.CommandTemplate = hookFieldString(fields, "command_template")
	c.ProducesEvidence = hookFieldList(fields, "produces_evidence")
	c.FeaturePattern = hookFieldString(fields, "feature_pattern")
	c.authorityText = hookFieldString(fields, "authority") + "\n" + hookFieldString(fields, "authority_boundary")
	return c
}

// ListHookContracts returns every contract in both roots (the
// docs/beacon/hooks root wins an id clash), sorted by hook id.
func ListHookContracts(root string) []HookContract {
	seen := map[string]bool{}
	var out []HookContract
	for _, dir := range hookRoots(root) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, en := range entries {
			if en.IsDir() || !strings.HasSuffix(en.Name(), ".md") {
				continue
			}
			id := strings.TrimSuffix(en.Name(), ".md")
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, LoadHookContract(root, id))
		}
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].HookID < out[i].HookID {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

var hookAuthorityRe = func() []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, kw := range hookAuthorityKeywords {
		out = append(out, regexp.MustCompile(`(?i)\b(can|may|will|writes?|rewrites?|sets?|passes?)\b[^.\n]*`+regexp.QuoteMeta(kw)))
	}
	return out
}()

// ValidateHookContract is project_hooks.validate_hook_contract.
func ValidateHookContract(root, hookID, feature, hookCode string) HookAdmission {
	c := LoadHookContract(root, hookID)
	var diags []HookDiagnostic
	add := func(code, msg string) { diags = append(diags, HookDiagnostic{ReasonCode: code, Message: msg}) }
	for _, e := range c.ParseErrors {
		add(e, e)
	}
	if c.Exists {
		fileID := strings.TrimSuffix(filepath.Base(c.Path), ".md")
		if c.declaredID != fileID {
			add("hook_id_mismatch", "hook_id must match file name")
		}
		if !allowedHookCodes[c.HookCode] {
			add("unknown_hook_code", "hook_code is not allowed")
		}
		if hookCode != "" && c.HookCode != hookCode {
			add("hook_code_mismatch", "hook_code does not match requested boundary")
		}
		if !c.Enabled {
			add("hook_disabled", "hook is disabled or missing enabled: true")
		}
		if !allowedTriggerPolicies[c.TriggerPolicy] {
			add("invalid_trigger_policy", "trigger_policy is not allowed")
		}
		if !allowedFailureRoutes[c.FailureRoute] {
			add("invalid_failure_route", "failure_route is not allowed")
		}
		if c.Skill == "" && c.CommandTemplate == "" {
			add("hook_invocation_missing", "skill or command_template is required")
		}
		if len(c.ProducesEvidence) == 0 {
			add("required_evidence_missing", "produces_evidence is required")
		}
		if feature != "" && c.FeaturePattern != "" && c.FeaturePattern != "*" &&
			c.FeaturePattern != feature && !strings.Contains(feature, c.FeaturePattern) {
			add("feature_selector_mismatch", "hook does not apply to this feature")
		}
		lower := strings.ToLower(c.body)
		for _, section := range []string{"purpose", "admission", "evidence", "failure"} {
			if !strings.Contains(lower, "## "+section) && !strings.Contains(lower, "### "+section) {
				add("section_missing_"+section, "missing body section: "+section)
			}
		}
		text := c.body + "\n" + c.authorityText
		for _, re := range hookAuthorityRe {
			if re.MatchString(text) {
				add("hook_authority_bypass", "hook cannot claim lifecycle truth authority")
				break
			}
		}
	}
	codes := []string{}
	for _, d := range diags {
		codes = append(codes, d.ReasonCode)
	}
	status := "pass"
	if len(diags) > 0 {
		status = "blocked"
	}
	return HookAdmission{
		SchemaVersion: "1.0", HookID: hookID, HookCode: c.HookCode, Feature: feature,
		Path: c.Path, Status: status, Blocked: len(diags) > 0,
		ReasonCodes: codes, Diagnostics: diags, Contract: c,
	}
}

// HookPlan is plan_hook's payload.
type HookPlan struct {
	SchemaVersion  string        `json:"schema_version"`
	Action         string        `json:"action"`
	HookID         string        `json:"hook_id"`
	Version        string        `json:"version,omitempty"`
	Feature        string        `json:"feature,omitempty"`
	Status         string        `json:"status"` // planned | blocked
	Blocked        bool          `json:"blocked"`
	Admission      HookAdmission `json:"admission"`
	EvidencePolicy string        `json:"evidence_policy"`
	TruthAuthority bool          `json:"truth_authority"`
	RuntimeRoot    string        `json:"runtime_root"`
	LatestModel    string        `json:"latest_read_model"`
}

func hookStateDir(root, hookID string) string {
	return filepath.Join(root, ".beacon", "state", "hooks", hookID)
}

func hookLatestRel(hookID string) string {
	return filepath.Join("docs", "beacon", ".machine", "hooks", hookID+".latest.json")
}

// PlanHook validates without executing anything.
func PlanHook(root, hookID, version, feature, hookCode string) HookPlan {
	adm := ValidateHookContract(root, hookID, feature, hookCode)
	status := "planned"
	if adm.Blocked {
		status = "blocked"
	}
	rel, _ := filepath.Rel(root, hookStateDir(root, hookID))
	return HookPlan{
		SchemaVersion: "1.0", Action: "plan", HookID: hookID, Version: version, Feature: feature,
		Status: status, Blocked: adm.Blocked, Admission: adm,
		EvidencePolicy: "runtime_evidence_only", TruthAuthority: false,
		RuntimeRoot: filepath.ToSlash(rel), LatestModel: filepath.ToSlash(hookLatestRel(hookID)),
	}
}

// HookRunOptions carries one RunHook invocation.
type HookRunOptions struct {
	Version  string
	Feature  string
	HookCode string
	DryRun   bool
	Confirm  bool
	Home     string // override for the builtin install followup (tests); "" = os.UserHomeDir
	Timeout  time.Duration
}

// HookRunResult is run_hook's payload; it is also the on-disk
// evidence document.
type HookRunResult struct {
	SchemaVersion       string            `json:"schema_version"`
	Action              string            `json:"action"`
	RunID               string            `json:"run_id"`
	HookID              string            `json:"hook_id"`
	Version             string            `json:"version,omitempty"`
	Feature             string            `json:"feature,omitempty"`
	DryRun              bool              `json:"dry_run"`
	Confirmed           bool              `json:"confirmed"`
	Status              string            `json:"status"` // passed | planned | blocked
	Blocked             bool              `json:"blocked"`
	ReasonCode          string            `json:"reason_code,omitempty"`
	Plan                HookPlan          `json:"plan"`
	SideEffectExecuted  bool              `json:"side_effect_executed"`
	RuntimeEvidenceOnly bool              `json:"runtime_evidence_only"`
	TruthAuthority      bool              `json:"truth_authority"`
	Execution           map[string]any    `json:"execution,omitempty"`
	Followup            map[string]any    `json:"post_release_followup,omitempty"`
	WrittenAt           string            `json:"written_at"`
	EvidencePaths       map[string]string `json:"evidence_paths"`
}

// RunHook executes the Python run flow (admission -> trigger
// policy -> execution) and always writes its evidence record.
func RunHook(ctx context.Context, root, hookID string, opts HookRunOptions) HookRunResult {
	plan := PlanHook(root, hookID, opts.Version, opts.Feature, opts.HookCode)
	runID := fmt.Sprintf("hook-%d", time.Now().UnixNano())
	res := HookRunResult{
		SchemaVersion: "1.0", Action: "run", RunID: runID, HookID: hookID,
		Version: opts.Version, Feature: opts.Feature, DryRun: opts.DryRun,
		Confirmed: opts.Confirm, Status: "passed", Plan: plan,
		RuntimeEvidenceOnly: true, TruthAuthority: false,
		WrittenAt: time.Now().UTC().Format(time.RFC3339),
	}
	block := func(code string) {
		res.Status = "blocked"
		res.Blocked = true
		res.ReasonCode = code
	}
	switch {
	case plan.Blocked:
		block(core.ReasonHookAdmissionBlocked)
	case opts.DryRun:
		res.Status = "planned"
	default:
		policy := plan.Admission.Contract.TriggerPolicy
		switch {
		case policy == "dry_run_only":
			block(core.ReasonHookDryRunOnly)
		case policy == "manual_confirmed" && !opts.Confirm:
			block(core.ReasonHookManualConfirm)
		case !opts.Confirm:
			block(core.ReasonHookHiddenAutoRun)
		default:
			res.SideEffectExecuted = true
			executeHook(ctx, root, plan.Admission.Contract, opts, &res, block)
		}
	}
	// Evidence: run record + latest read model (Python paths).
	runtimePath := filepath.Join(hookStateDir(root, hookID), runID+".json")
	latestPath := filepath.Join(root, hookLatestRel(hookID))
	res.EvidencePaths = map[string]string{
		"runtime":           relSlash(root, runtimePath),
		"latest_read_model": filepath.ToSlash(hookLatestRel(hookID)),
	}
	if raw, err := json.MarshalIndent(res, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(runtimePath), 0o755)
		_ = os.WriteFile(runtimePath, append(raw, '\n'), 0o644)
		_ = os.MkdirAll(filepath.Dir(latestPath), 0o755)
		_ = os.WriteFile(latestPath, append(raw, '\n'), 0o644)
	}
	return res
}

// executeHook performs the confirmed side effect: the builtin
// install followup, a command_template through the harness, or —
// for skill-only hooks — an honest no-op record (the skill runs
// host-side; the kernel has nothing to execute).
func executeHook(ctx context.Context, root string, c HookContract, opts HookRunOptions, res *HookRunResult, block func(string)) {
	if c.HookID == PostReleaseInstallHookID {
		followup := runPostReleaseInstallFollowup(opts.Home)
		res.Followup = followup
		if failed, _ := followup["blocked"].(bool); failed {
			block(core.ReasonHookFollowupFailed)
		}
		return
	}
	if c.CommandTemplate != "" {
		tmpl := c.CommandTemplate
		replacer := strings.NewReplacer(
			"{project}", root, "{version}", opts.Version,
			"{feature}", opts.Feature, "{hook_id}", c.HookID, "{hook_code}", c.HookCode,
		)
		command := replacer.Replace(tmpl)
		argv, err := SplitCommandWords(command)
		if err != nil || len(argv) == 0 {
			res.Execution = map[string]any{"command": command, "error": fmt.Sprint(err)}
			block(core.ReasonHookCommandFailed)
			return
		}
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}
		env := harness.IterationEnv(harness.IterationRequest{Stage: "hook"})
		env = append(env, "BEACON_HOOK_ID="+c.HookID, "BEACON_HOOK_CODE="+c.HookCode,
			"BEACON_PROJECT_ROOT="+root)
		out := harness.RunCommand(ctx, argv, root, env, timeout)
		res.Execution = map[string]any{
			"command": command, "argv": argv, "exit_code": out.ExitCode,
			"timed_out": out.TimedOut, "duration_ms": out.Duration.Milliseconds(),
			"stdout_tail": tailString(out.Stdout, 2000), "stderr_tail": tailString(out.Stderr, 2000),
		}
		if out.Err != nil || out.TimedOut || out.ExitCode != 0 {
			block(core.ReasonHookCommandFailed)
		}
		return
	}
	res.Execution = map[string]any{
		"kind": "skill", "skill": c.Skill,
		"note": "skill hooks execute host-side; the kernel records the confirmed trigger only",
	}
	res.SideEffectExecuted = false
}

// runPostReleaseInstallFollowup is the Go counterpart of Python's
// _run_beacon_post_release_local_install core: align the host
// skill directories (claude + agents roots) with the shipped
// skills. Python's wider followup (context setup, launcher setup,
// cli version probes, agent-eval) has no Go counterpart and is
// listed as residual in the batch report.
func runPostReleaseInstallFollowup(homeOverride string) map[string]any {
	home := homeOverride
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return map[string]any{"blocked": true, "reason": err.Error()}
		}
	}
	out := map[string]any{"hook_id": PostReleaseInstallHookID}
	blocked := false
	for host, dir := range map[string]string{
		"claude": filepath.Join(home, ".claude", "skills"),
		"agents": filepath.Join(home, ".agents", "skills"),
	} {
		res, err := skills.Install(dir, true)
		if err != nil {
			out[host] = map[string]any{"status": "blocked", "error": err.Error()}
			blocked = true
			continue
		}
		out[host] = map[string]any{
			"status": "pass", "target_dir": res.TargetDir,
			"skills": res.Skills, "files_written": len(res.Files),
		}
	}
	out["blocked"] = blocked
	if blocked {
		out["status"] = "blocked"
	} else {
		out["status"] = "pass"
	}
	return out
}

// DisableHook sets enabled: false on the contract file (Python
// disable_hook semantics).
func DisableHook(root, hookID string) (string, string, error) {
	abs, found := hookContractPath(root, hookID)
	if !found {
		return "", core.ReasonHookFileMissing, nil
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", core.ReasonHookFileMissing, nil
	}
	text := string(raw)
	enabledRe := regexp.MustCompile(`(?m)^enabled:\s*(true|false)\s*$`)
	switch {
	case enabledRe.MatchString(text):
		text = enabledRe.ReplaceAllString(text, "enabled: false")
	case strings.HasPrefix(text, "---\n"):
		text = strings.Replace(text, "---\n", "---\nenabled: false\n", 1)
	default:
		text = "---\nhook_id: " + hookID + "\nenabled: false\n---\n\n" + text
	}
	if err := os.WriteFile(abs, []byte(text), 0o644); err != nil {
		return "", "", err
	}
	rel, _ := filepath.Rel(root, abs)
	return filepath.ToSlash(rel), "", nil
}

// HookStatus is hook_status's payload: admission + latest run.
func HookStatus(root, hookID, feature string) map[string]any {
	adm := ValidateHookContract(root, hookID, feature, "")
	status := "ready"
	if adm.Blocked {
		status = "blocked"
	}
	latest := map[string]any{}
	if raw, err := os.ReadFile(filepath.Join(root, hookLatestRel(hookID))); err == nil {
		_ = json.Unmarshal(raw, &latest)
	}
	return map[string]any{
		"schema_version": "1.0", "hook_id": hookID, "feature": feature,
		"status": status, "blocked": adm.Blocked, "admission": adm,
		"latest_run": latest, "truth_authority": false,
	}
}

// HookTriggerEntry is one hook's outcome in a completion trigger.
type HookTriggerEntry struct {
	HookID     string `json:"hook_id"`
	Action     string `json:"action"` // executed | dry_run | awaiting_confirmation
	Status     string `json:"status"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// HookTriggerSummary is RunPostReleaseHooks' outcome.
type HookTriggerSummary struct {
	RunID string             `json:"run_id"`
	Hooks []HookTriggerEntry `json:"hooks"`
}

// RunPostReleaseHooks fires the project's enabled post_release
// hooks after a run completes: auto_allowed hooks execute (that is
// what the policy means), manual_confirmed hooks are reported as
// awaiting their explicit confirmation (never silently run),
// dry_run_only hooks produce planned evidence. Outcomes land in
// one run event; a hook failure never fails the completion.
func (e *Engine) RunPostReleaseHooks(runID string) (HookTriggerSummary, error) {
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return HookTriggerSummary{}, err
	}
	fctx, err := e.featureContext(run.FeatureID)
	if err != nil {
		return HookTriggerSummary{}, err
	}
	root := fctx.Project.RootPath
	summary := HookTriggerSummary{RunID: runID, Hooks: []HookTriggerEntry{}}
	for _, c := range ListHookContracts(root) {
		if c.HookCode != "post_release" || !c.Enabled {
			continue
		}
		adm := ValidateHookContract(root, c.HookID, fctx.Feature.Slug, "post_release")
		entry := HookTriggerEntry{HookID: c.HookID}
		switch {
		case adm.Blocked:
			entry.Action = "executed"
			res := RunHook(context.Background(), root, c.HookID, HookRunOptions{
				Version: fctx.Version.Version, Feature: fctx.Feature.Slug, HookCode: "post_release",
			})
			entry.Status, entry.ReasonCode = res.Status, res.ReasonCode
		case c.TriggerPolicy == "manual_confirmed":
			entry.Action = "awaiting_confirmation"
			entry.Status = "awaiting_confirmation"
			entry.ReasonCode = core.ReasonHookManualConfirm
		case c.TriggerPolicy == "dry_run_only":
			entry.Action = "dry_run"
			res := RunHook(context.Background(), root, c.HookID, HookRunOptions{
				Version: fctx.Version.Version, Feature: fctx.Feature.Slug, HookCode: "post_release", DryRun: true,
			})
			entry.Status, entry.ReasonCode = res.Status, res.ReasonCode
		default: // auto_allowed
			entry.Action = "executed"
			res := RunHook(context.Background(), root, c.HookID, HookRunOptions{
				Version: fctx.Version.Version, Feature: fctx.Feature.Slug, HookCode: "post_release", Confirm: true,
			})
			entry.Status, entry.ReasonCode = res.Status, res.ReasonCode
		}
		summary.Hooks = append(summary.Hooks, entry)
	}
	if len(summary.Hooks) > 0 {
		_ = e.appendEvent(run, EventPostReleaseHooks, map[string]any{
			"hooks": summary.Hooks, "feature": fctx.Feature.Slug,
		})
	}
	return summary, nil
}

func tailString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func relSlash(root, abs string) string {
	if rel, err := filepath.Rel(root, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return abs
}
