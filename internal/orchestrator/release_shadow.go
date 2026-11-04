// Release policy + auto_shadow (issue #8, shadow step only).
//
// The human release gate is an invariant. A shadow run answers one
// question — "if a policy signed instead of a human, would this
// release pass, and if not, which check blocks it?" — and records
// the answer as an event. It never sets ReleaseHumanConfirmed, so
// `goal complete` keeps requiring the human.
package orchestrator

import (
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

const (
	ReleasePolicyHuman      = "human"
	ReleasePolicyAutoShadow = "auto_shadow"
	ReleasePolicyAuto       = "auto" // recognised only to refuse it explicitly
)

func releasePolicyOrHuman(policy string) string {
	if strings.TrimSpace(policy) == "" {
		return ReleasePolicyHuman
	}
	return policy
}

// ReleaseShadowView is the recorded shadow verdict.
type ReleaseShadowView struct {
	Policy                    string   `json:"policy"`
	Verdict                   string   `json:"verdict"` // pass | blocked
	ReasonCodes               []string `json:"reason_codes,omitempty"`
	EvaluatedAt               string   `json:"evaluated_at"`
	HumanConfirmationBypassed bool     `json:"human_confirmation_bypassed"`
}

// NormalizeReleasePolicy validates the run-level policy. "" means
// human. auto is refused explicitly: the shadow step is deliberately
// the only thing shipped.
func NormalizeReleasePolicy(policy string) (string, error) {
	switch strings.TrimSpace(policy) {
	case "", ReleasePolicyHuman:
		return ReleasePolicyHuman, nil
	case ReleasePolicyAutoShadow:
		return ReleasePolicyAutoShadow, nil
	case ReleasePolicyAuto:
		return "", core.NewError(core.ReasonReleasePolicyUnsupported, "goal.releasePolicy",
			"release_policy=auto is not implemented; only auto_shadow (preview only) is available")
	default:
		return "", core.NewError(core.ReasonReleasePolicyUnsupported, "goal.releasePolicy",
			"unknown release_policy "+policy+" (human | auto_shadow)")
	}
}

// ReleaseShadow evaluates the release prerequisites with a policy
// signature instead of a human one, records the verdict on the run
// and as an event, and leaves the human gate untouched.
func (e *Engine) ReleaseShadow(runID, evaluator string) (ReleaseShadowView, error) {
	if strings.TrimSpace(evaluator) == "" {
		return ReleaseShadowView{}, core.NewError(core.ReasonGateEvaluatorRequired, "goal.ReleaseShadow",
			"an evaluator identity is required")
	}
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ReleaseShadowView{}, err
	}
	policy := m.ReleasePolicy
	if policy == "" {
		policy = ReleasePolicyHuman
	}
	if policy != ReleasePolicyAutoShadow {
		return ReleaseShadowView{}, core.NewError(core.ReasonReleaseShadowNotConfigured, "goal.ReleaseShadow",
			"run "+runID+" release_policy is "+policy+"; set auto_shadow to enable the shadow preview")
	}
	ctx, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return ReleaseShadowView{}, err
	}
	if !ok {
		return ReleaseShadowView{}, core.NewError(ReasonProjectNotRegistered, "goal.ReleaseShadow",
			"no feature context for run "+runID)
	}
	// Fresh facts every call: a cached verdict would miss a stale
	// artifact or a gate that regressed since the last preview.
	facts, err := e.releaseFacts(ctx)
	if err != nil {
		return ReleaseShadowView{}, err
	}
	facts.HumanConfirmed = true
	facts.HumanConfirmedBy = "policy:auto_shadow"
	ev := gate.EvaluateRelease(facts, evaluator)
	view := ReleaseShadowView{
		Policy: policy, Verdict: string(ev.Verdict), ReasonCodes: ev.ReasonCodes,
		EvaluatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
		HumanConfirmationBypassed: true,
	}
	m.ReleaseShadow = &view
	if err := e.appendEvent(run, "release_shadow_evaluated", map[string]any{
		"policy": policy, "verdict": view.Verdict, "reason_codes": view.ReasonCodes,
	}); err != nil {
		return ReleaseShadowView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ReleaseShadowView{}, err
	}
	return view, nil
}
