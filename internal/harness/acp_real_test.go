//go:build acpreal

// Real-ACP-agent evidence for AC-046 / AC-006.
//
// Point BEACON_ACP_TARGET at a real ACP agent binary (its argv, space
// separated) to run one genuine session/prompt iteration:
//
//   BEACON_ACP_TARGET="my-acp-agent --stdio" \
//     go test ./internal/harness/ -run TestAcpRealIteration -tags=acpreal -v
//
// Without that variable the test skips — and a skip is not a pass, so
// AC-006 stays open until this runs green against a real agent.

package harness

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAcpRealIteration(t *testing.T) {
	target := strings.TrimSpace(os.Getenv("BEACON_ACP_TARGET"))
	if target == "" {
		t.Skip("BEACON_ACP_TARGET is not set: no real ACP agent to drive (skip is not a pass)")
	}
	argv := strings.Fields(target)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	res, err := RunACPSession(ctx, argv, "Reply with exactly the word OK and nothing else.", "", 4*time.Minute)
	if err != nil {
		t.Fatalf("real ACP session failed: %v", err)
	}
	if res.SessionID == "" {
		t.Error("real ACP session returned no session id")
	}
	if res.StopReason == "" {
		t.Error("real ACP session returned no stop reason")
	}
	if strings.TrimSpace(res.Content) == "" {
		t.Error("real ACP session returned no content")
	}
	t.Logf("acp: agent=%s session=%s stop=%s content=%q", res.AgentName, res.SessionID, res.StopReason, res.Content)
}
