package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/llm"
)

func TestChiefOfStaffIndirectPrompt(t *testing.T) {
	mockClient := &llm.MockClient{
		Handler: func(req llm.Request) string {
			if req.Role != llm.RoleChiefOfStaff {
				return "wrong role"
			}
			return "Restated: Add WeChat Pay gateway with refund webhook verification."
		},
	}

	cos := NewChiefOfStaff(mockClient, "test-model")
	brief, err := cos.AlignIntent(context.Background(), "帮我加微信支付和退款")
	if err != nil {
		t.Fatalf("unexpected error from AlignIntent: %v", err)
	}

	if brief.OriginalPrompt != "帮我加微信支付和退款" {
		t.Errorf("unexpected original prompt: %s", brief.OriginalPrompt)
	}
	if !strings.Contains(brief.RestatedIntent, "WeChat Pay") {
		t.Errorf("expected restated intent to contain 'WeChat Pay', got: %s", brief.RestatedIntent)
	}
	// The model returned no structured tail, so the brief must carry no
	// risks/boundaries — the old build filled these with two constants
	// and every plan card therefore claimed the same invariants.
	if len(brief.IdentifiedRisks) != 0 || len(brief.Boundaries) != 0 {
		t.Errorf("risks/boundaries must stay empty when the model states none, got %v / %v",
			brief.IdentifiedRisks, brief.Boundaries)
	}
	if !brief.Degraded {
		t.Error("a prose-only answer must be marked degraded")
	}
	// A Chinese-only utterance still needs a valid ASCII package slug.
	if len(brief.ProposedSlugs) != 1 || !slugHead.MatchString(brief.ProposedSlugs[0]) {
		t.Errorf("expected one valid derived slug, got %v", brief.ProposedSlugs)
	}
	if brief.Confirmed {
		t.Errorf("briefing must start unconfirmed until user approval")
	}
}

// When the model does return the structured tail, its fields are used
// verbatim (and nothing is invented on top).
func TestChiefOfStaffParsesStructuredTail(t *testing.T) {
	mockClient := &llm.MockClient{
		Handler: func(req llm.Request) string {
			return "Restated: refunds must be idempotent.\n```json\n" +
				`{"slug":"refund-idempotency","risks":["duplicate callbacks"],"boundaries":["no schema change"]}` +
				"\n```\n"
		},
	}
	brief, err := NewChiefOfStaff(mockClient, "test-model").AlignIntent(context.Background(), "退款幂等")
	if err != nil {
		t.Fatalf("AlignIntent: %v", err)
	}
	if brief.ProposedSlugs[0] != "refund-idempotency" {
		t.Errorf("slug = %v", brief.ProposedSlugs)
	}
	if len(brief.IdentifiedRisks) != 1 || brief.IdentifiedRisks[0] != "duplicate callbacks" {
		t.Errorf("risks = %v", brief.IdentifiedRisks)
	}
	if len(brief.Boundaries) != 1 || brief.Boundaries[0] != "no schema change" {
		t.Errorf("boundaries = %v", brief.Boundaries)
	}
	if brief.Degraded {
		t.Error("a parsed tail is not degraded")
	}
	if strings.Contains(brief.RestatedIntent, "```") {
		t.Error("the JSON tail must not leak into the restatement")
	}
}

func TestMultiAgentPipelineRolesAndGate(t *testing.T) {
	// 验证专职 Worker Agent 的不可替代分工（Architect -> Coder -> QA）
	// 以及 QA 验资必须卡住状态机（AC-AGENT-003, AC-AGENT-006）

	// 1. Coder 完成实现，但尚未进行机器验资（nil evidence）
	err := core.ValidateEvidenceTransition(core.StageImplement, core.StageQa, nil)
	if err == nil {
		t.Fatalf("state machine must reject transition to QA when evidence is nil")
	}

	// 2. Coder 试图伪造通过，但 ExitCode != 0
	invalidEv := &core.EvidencePayload{
		AC:           "AC-001",
		Status:       "failed",
		ExitCode:     1,
		EvidenceHash: "bad-hash",
	}
	err = core.ValidateEvidenceTransition(core.StageImplement, core.StageQa, invalidEv)
	if err == nil {
		t.Fatalf("state machine must reject transition when exit code is non-zero")
	}

	// 3. QA Agent 独立运行 Harness 生成机器证据（ExitCode == 0, Status == passed）
	validEv := &core.EvidencePayload{
		AC:           "AC-001",
		Status:       "passed",
		ExitCode:     0,
		EvidenceHash: fmt.Sprintf("sha256-%s", "valid-qa-run-evidence"),
		TestCases:    []string{"TestPaymentExecution", "TestRefundWebhook"},
	}
	err = core.ValidateEvidenceTransition(core.StageImplement, core.StageQa, validEv)
	if err != nil {
		t.Fatalf("valid QA evidence must pass state transition: %v", err)
	}
}
