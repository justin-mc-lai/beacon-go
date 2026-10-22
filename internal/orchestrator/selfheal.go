// Self-heal (P2b, G-19): the plan_goal_self_heal band, with the step
// table ported verbatim from the progressive map's
// goal_transversal.self_heal (pre: two auto checks; mid_on_block: one
// auto repair + one manual check; post_release: one human-confirmed
// install). PlanSelfHeal only plans. ExecuteSelfHeal executes ONLY
// the auto steps, and only where a real action exists in this
// codebase:
//
//   - verify-context: the run resolves and the store schema contract
//     verifies (AC-161) — a real check, executed.
//   - verify-version-surfaces: the feature's project has exactly one
//     current version recorded — a real check, executed.
//   - repair-evidence-manifest: rebuilds the evidence manifest
//     summary (artifact pointers + event trail digest) and appends
//     one evidence_manifest_repaired event — a real repair record.
//   - workspace-admit: manual in the source table; reported as
//     planned, never executed here.
//   - post-release-local-install: requires human confirmation; NEVER
//     auto-executed, in any mode.
//
// Steps without a real backing action are reported planned, never
// dressed up as executed.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// postReleaseCoverageDetail names the agents the post-release
// install step covers. The v1 defect this guards against
// (beacon-post-release-hook-agent-coverage-v163): the hook loop
// hardcoded ("codex", "claude") and silently skipped the other
// host agents, whose skills went stale without a block. Here the
// coverage list is DERIVED from the single harness profile
// registry at plan time — there is no second list to drift. If a
// profile is added to the registry, this coverage follows it.
func postReleaseCoverageDetail() string {
	names := make([]string, 0, len(harness.BuiltinProfiles()))
	for _, p := range harness.BuiltinProfiles() {
		names = append(names, p.Name)
	}
	return "agent coverage derived from the harness profile registry (no hardcoded list): " +
		strings.Join(names, ", ") + "; drift on any covered agent blocks the (human-confirmed) install step"
}

// Self-heal step statuses.
const (
	HealPlanned              = "planned"
	HealExecuted             = "executed"
	HealFailed               = "failed"
	HealRequiresConfirmation = "requires_confirmation"
	HealManualOnly           = "manual_only"
)

// SelfHealStep is one step of a self-heal plan (map fields verbatim,
// plus execution bookkeeping).
type SelfHealStep struct {
	ID                        string `json:"id"`
	Command                   string `json:"command"`
	UserFacing                string `json:"user_facing"`
	Auto                      bool   `json:"auto"`
	RequiresHumanConfirmation bool   `json:"requires_human_confirmation"`
	Status                    string `json:"status"`
	Detail                    string `json:"detail,omitempty"`
}

// SelfHealPlan is one phase's plan (optionally after execution).
type SelfHealPlan struct {
	RunID      string         `json:"run_id"`
	Phase      string         `json:"phase"` // pre | mid | post
	Executed   bool           `json:"executed"`
	Steps      []SelfHealStep `json:"steps"`
	ReasonCode string         `json:"reason_code,omitempty"`
}

// selfHealTable is the static port of the map's self_heal table.
var selfHealTable = map[string][]SelfHealStep{
	"pre": {
		{ID: "verify-context", Command: "beacon doctor verify-context -p . --strict --json",
			UserFacing: "检查项目上下文是否就绪", Auto: true},
		{ID: "verify-version-surfaces", Command: "beacon doctor verify-version-surfaces -p . --strict --json",
			UserFacing: "检查版本表面是否一致", Auto: true},
	},
	"mid": {
		{ID: "repair-evidence-manifest", Command: "beacon doctor repair-evidence-manifest -p . --json",
			UserFacing: "尝试修复证据清单", Auto: true},
		{ID: "workspace-admit", Command: "beacon workspace admit -p . --json",
			UserFacing: "检查工作区准入", Auto: false},
	},
	"post": {
		{ID: "post-release-local-install", Command: "beacon hooks run beacon-post-release-local-install -p . --confirm",
			UserFacing: "发布后安装/对齐本机环境（需确认）", Auto: false, RequiresHumanConfirmation: true},
	},
}

// deriveHealPhase picks the phase from run state when the caller does
// not pin one: blocked -> mid; done -> post; otherwise pre.
func deriveHealPhase(m *runMeta) string {
	switch {
	case m.LoopStage == string(core.LoopDone):
		return "post"
	case m.Blocked:
		return "mid"
	default:
		return "pre"
	}
}

func normalizeHealPhase(phase string, m *runMeta) (string, error) {
	switch phase {
	case "":
		return deriveHealPhase(m), nil
	case "pre":
		return "pre", nil
	case "mid", "mid_on_block":
		return "mid", nil
	case "post", "post_release":
		return "post", nil
	default:
		return "", core.NewError("invalid_self_heal_phase", "goal.PlanSelfHeal", "phase must be pre|mid|post")
	}
}

// PlanSelfHeal returns the phase plan without executing anything:
// auto steps are marked planned, the manual step manual_only, the
// post-release step requires_confirmation.
func (e *Engine) PlanSelfHeal(runID, phase string) (SelfHealPlan, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return SelfHealPlan{}, err
	}
	ph, err := normalizeHealPhase(phase, m)
	if err != nil {
		return SelfHealPlan{}, err
	}
	plan := SelfHealPlan{RunID: runID, Phase: ph}
	for _, step := range selfHealTable[ph] {
		s := step
		switch {
		case s.RequiresHumanConfirmation:
			s.Status = HealRequiresConfirmation
		case !s.Auto:
			s.Status = HealManualOnly
		default:
			s.Status = HealPlanned
		}
		if s.ID == "post-release-local-install" {
			s.Detail = postReleaseCoverageDetail()
		}
		plan.Steps = append(plan.Steps, s)
	}
	return plan, nil
}

// ExecuteSelfHeal runs the plan's auto steps that have a real backing
// action and records one self_heal_executed event with the per-step
// outcomes. Non-auto steps are never executed.
func (e *Engine) ExecuteSelfHeal(runID, phase string) (SelfHealPlan, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return SelfHealPlan{}, err
	}
	plan, err := e.PlanSelfHeal(runID, phase)
	if err != nil {
		return SelfHealPlan{}, err
	}
	plan.Executed = true
	for i := range plan.Steps {
		s := &plan.Steps[i]
		if !s.Auto {
			continue // manual / human-confirmed steps keep their status
		}
		detail, execErr := e.executeHealStep(run, m, s.ID)
		if execErr != nil {
			s.Status = HealFailed
			s.Detail = execErr.Error()
		} else {
			s.Status = HealExecuted
			s.Detail = detail
		}
	}
	statuses := map[string]string{}
	for _, s := range plan.Steps {
		statuses[s.ID] = s.Status
	}
	_ = e.appendEvent(run, "self_heal_executed", map[string]any{
		"phase": plan.Phase, "step_statuses": statuses,
	})
	return plan, nil
}

// executeHealStep performs the real backing action of one auto step.
func (e *Engine) executeHealStep(run store.Run, m *runMeta, stepID string) (string, error) {
	switch stepID {
	case "verify-context":
		if err := e.store.VerifySchemaContract(); err != nil {
			return "", err
		}
		if _, err := e.store.Features().GetByID(run.FeatureID); err != nil {
			return "", err
		}
		return "schema contract verified; feature context resolves", nil
	case "verify-version-surfaces":
		feat, err := e.store.Features().GetByID(run.FeatureID)
		if err != nil {
			return "", err
		}
		ver, err := e.store.Projects().GetVersionByID(feat.ProjectVersionID)
		if err != nil {
			return "", err
		}
		cur, err := e.store.Projects().CurrentVersion(ver.ProjectID)
		if err != nil || cur.ID == "" {
			return "", core.NewError("version_surface_unresolved", "goal.selfHeal",
				"project has no current version recorded")
		}
		return fmt.Sprintf("current version surface: %s", cur.Version), nil
	case "repair-evidence-manifest":
		arts, err := e.store.Artifacts().ListByRun(run.ID)
		if err != nil {
			return "", err
		}
		events, err := e.store.ListEventsByRun(run.ID)
		if err != nil {
			return "", err
		}
		hashes := make([]string, 0, len(arts))
		for _, a := range arts {
			hashes = append(hashes, a.ContentHash)
		}
		sort.Strings(hashes)
		h := sha256.New()
		h.Write([]byte(strings.Join(hashes, ",")))
		h.Write([]byte(fmt.Sprintf("|events=%d", len(events))))
		manifestHash := hex.EncodeToString(h.Sum(nil))
		if err := e.appendEvent(run, "evidence_manifest_repaired", map[string]any{
			"artifact_count": len(arts), "event_count": len(events),
			"manifest_hash": manifestHash,
		}); err != nil {
			return "", err
		}
		return fmt.Sprintf("manifest rebuilt: %d artifact refs, %d events, sha256 %s", len(arts), len(events), manifestHash[:12]), nil
	default:
		return "", core.NewError(core.ReasonSelfHealManualOnly, "goal.selfHeal", "no auto action for step "+stepID)
	}
}
