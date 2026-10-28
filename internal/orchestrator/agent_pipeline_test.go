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
	if len(brief.IdentifiedRisks) == 0 || len(brief.Boundaries) == 0 {
		t.Errorf("expected non-empty risks and boundaries")
	}
	if brief.Confirmed {
		t.Errorf("briefing must start unconfirmed until user approval")
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
