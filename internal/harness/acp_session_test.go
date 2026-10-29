package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// acpStubAgent is a protocol-level stand-in for an ACP agent: it speaks
// the same JSON-RPC over stdio (initialize, session/new, session/prompt)
// and echoes the prompt back. It proves the CLIENT side is correct.
//
// It is NOT evidence for AC-046: AC-046 asks for an iteration against
// a REAL ACP agent binary. This stub only shows that Beacon sends the
// right requests and reads the right answers.
const acpStubAgent = `import sys, json
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except Exception:
        continue
    mid = msg.get("id")
    method = msg.get("method")
    if method == "initialize":
        resp = {"jsonrpc": "2.0", "id": mid, "result": {"protocolVersion": 1, "agentCapabilities": {}, "agentInfo": {"name": "stub"}}}
    elif method == "session/new":
        resp = {"jsonrpc": "2.0", "id": mid, "result": {"sessionId": "sess-stub-1"}}
    elif method == "session/prompt":
        try:
            text = msg["params"]["prompt"][0]["text"]
        except Exception:
            text = ""
        resp = {"jsonrpc": "2.0", "id": mid, "result": {"stopReason": "end_turn", "content": [{"type": "text", "text": "ACK:" + text}]}}
    else:
        continue
    sys.stdout.write(json.dumps(resp) + "\n")
    sys.stdout.flush()
`

func writeACPStub(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not on PATH; cannot run the ACP protocol stub")
	}
	p := filepath.Join(t.TempDir(), "acp_stub.py")
	if err := os.WriteFile(p, []byte(acpStubAgent), 0o644); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return p
}

// TestAcpSessionDrivesTheProtocolCorrectly: the client sends
// initialize, session/new and one session/prompt, and reads the agent's
// own answer back. This is client-side correctness against a protocol
// stub — the real-agent evidence AC-046 asks for is separate.
func TestAcpSessionDrivesTheProtocolCorrectly(t *testing.T) {
	stub := writeACPStub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := RunACPSession(ctx, []string{"python3", stub}, "do the thing", t.TempDir(), 30*time.Second)
	if err != nil {
		t.Fatalf("ACP session against the protocol stub failed: %v", err)
	}
	if res.SessionID != "sess-stub-1" {
		t.Errorf("session id = %q, want sess-stub-1", res.SessionID)
	}
	if res.StopReason != "end_turn" {
		t.Errorf("stop reason = %q, want end_turn", res.StopReason)
	}
	if !strings.Contains(res.Content, "ACK:do the thing") {
		t.Errorf("the prompt did not reach the agent: content = %q", res.Content)
	}
	if res.AgentName == "" {
		t.Error("the session result carries no agent name")
	}
}

// TestAcpIterateMapsSessionOntoResult: the backend's Iterate maps the
// session onto the AC-042 result schema rather than inventing one.
func TestAcpIterateMapsSessionOntoResult(t *testing.T) {
	stub := writeACPStub(t)
	dir := t.TempDir()
	ctxPath := filepath.Join(dir, "stage-context.json")
	if err := os.WriteFile(ctxPath, []byte(`{"stage":"implement","prompt":"implement the thing"}`), 0o644); err != nil {
		t.Fatalf("write stage context: %v", err)
	}
	b := NewAcpBackend([]string{"python3", stub}, 30*time.Second)
	res, err := b.Iterate(context.Background(), IterationRequest{
		Stage: "implement", Workdir: dir, ContextPath: ctxPath,
	})
	if err != nil {
		t.Fatalf("ACP iterate failed: %v", err)
	}
	if !res.Success {
		t.Errorf("iterate reported success=false: %+v", res)
	}
	if !strings.Contains(res.Summary, "implement the thing") {
		t.Errorf("the stage-context prompt did not reach the agent: %q", res.Summary)
	}
	if res.AgentName == "" {
		t.Error("iterate carries no agent name")
	}
}

// TestAcpRefusesWithoutTarget (AC-007): with no usable target, the
// call fails with a reason code. It never returns a made-up result.
func TestAcpRefusesWithoutTarget(t *testing.T) {
	ctx := context.Background()

	// 1. no target at all
	if _, err := RunACPSession(ctx, nil, "hello", t.TempDir(), time.Second); err == nil {
		t.Error("an unconfigured target must fail, not succeed")
	}

	// 2. a command that does not exist
	_, err := RunACPSession(ctx, []string{"definitely-not-a-real-acp-agent-xyz"}, "hello", t.TempDir(), time.Second)
	if err == nil {
		t.Fatal("a missing target must fail, not succeed")
	}
	if !strings.Contains(err.Error(), "command not found") && !strings.Contains(err.Error(), "not configured") {
		t.Errorf("missing target error should name the cause: %v", err)
	}

	// 3. a target that starts but never answers: the call must time out
	// rather than report an empty success.
	silent := filepath.Join(t.TempDir(), "silent.sh")
	if err := os.WriteFile(silent, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("write silent agent: %v", err)
	}
	if err := os.Chmod(silent, 0o755); err != nil {
		t.Fatalf("chmod silent agent: %v", err)
	}
	start := time.Now()
	if _, err := RunACPSession(ctx, []string{silent}, "hello", t.TempDir(), 2*time.Second); err == nil {
		t.Fatal("an agent that never answers must fail, not succeed")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("the call took %s; the timeout did not bound it", elapsed)
	}

	// 4. the backend refuses too, and never fabricates a result.
	b := NewAcpBackend([]string{"definitely-not-a-real-acp-agent-xyz"}, time.Second)
	res, err := b.Iterate(ctx, IterationRequest{Stage: "implement"})
	if err == nil {
		t.Fatal("iterate against a missing target must fail")
	}
	if res.Success {
		t.Error("iterate returned a successful result against a missing target")
	}
}
