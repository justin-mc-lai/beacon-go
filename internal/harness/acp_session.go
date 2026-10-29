package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"beacon-go/internal/core"
)

// ACP session iteration (AC-046).
//
// The probe in acp.go proves a target can answer `initialize` and
// `session/new`. This file goes one step further and drives a real
// session: it opens the session, sends one `session/prompt`, and maps
// the agent's own answer onto the AC-042 IterationResult.
//
// Nothing here invents a result. A missing target, a refused handshake
// or a prompt that is never answered returns an error with a reason
// code — never a fabricated success.

// acpSessionPromptID is the request id of the session/prompt call.
const acpSessionPromptID = 3

// ACPSessionResult is what one real ACP session iteration produced.
type ACPSessionResult struct {
	SessionID  string `json:"session_id"`
	StopReason string `json:"stop_reason"`
	Content    string `json:"content"`
	AgentName  string `json:"agent_name"`
}

// acpPromptText is the text part of an ACP prompt block.
type acpPromptText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// RunACPSession drives one real ACP session against argv: initialize,
// session/new, then a single session/prompt. It returns what the agent
// actually answered.
//
// Every failure path returns an error. A caller that gets a nil error
// has a real answer from a real agent.
func RunACPSession(ctx context.Context, argv []string, prompt string, workdir string, timeout time.Duration) (ACPSessionResult, error) {
	out := ACPSessionResult{}
	if len(argv) == 0 {
		return out, core.NewError(core.ReasonExecutorNotConfigured,
			"harness.RunACPSession", "no ACP target command configured")
	}
	if strings.TrimSpace(prompt) == "" {
		return out, core.NewError(core.ReasonACPNotSupported,
			"harness.RunACPSession", "empty prompt: an ACP session needs something to say")
	}
	resolved, err := exec.LookPath(argv[0])
	if err != nil {
		if _, statErr := os.Stat(argv[0]); statErr == nil {
			return out, core.NewError(core.ReasonExecutorCommandNotExec,
				"harness.RunACPSession", argv[0]+" exists but is not executable")
		}
		return out, core.NewError(core.ReasonExecutorCommandMissing,
			"harness.RunACPSession", "command not found: "+argv[0])
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, resolved, argv[1:]...)
	// A killed target may leave a child holding the stdout/stderr pipes
	// open, which would make Wait block for the child's whole lifetime.
	// WaitDelay bounds that: after the context is done the pipes are
	// closed and Wait returns.
	cmd.WaitDelay = 3 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return out, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return out, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return out, core.NewError(core.ReasonExecutorNotConfigured,
			"harness.RunACPSession", "could not start ACP target: "+err.Error())
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()

	client := newACPRPCClient(stdin, stdout)

	// Step 1: initialize.
	raw, rpcErr, callErr := client.call(runCtx, acpInitializeID, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": "beacon-go-session", "version": "2.0.0-go.9"},
	})
	if callErr != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "initialize failed: "+callErr.Error()+" "+stderr.String())
	}
	if rpcErr != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "initialize rejected: "+rpcErr.Error())
	}
	var initResult struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	_ = json.Unmarshal(raw, &initResult)

	// Step 2: session/new.
	raw, rpcErr, callErr = client.call(runCtx, acpSessionNewID, "session/new", map[string]any{
		"cwd":        workdir,
		"mcpServers": []any{},
	})
	if callErr != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "session/new failed: "+callErr.Error()+" "+stderr.String())
	}
	if rpcErr != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "session/new rejected: "+rpcErr.Error())
	}
	var newResult struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &newResult); err != nil || newResult.SessionID == "" {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "session/new returned no session id")
	}
	out.SessionID = newResult.SessionID

	// Step 3: session/prompt — the actual iteration.
	raw, rpcErr, callErr = client.call(runCtx, acpSessionPromptID, "session/prompt", map[string]any{
		"sessionId": out.SessionID,
		"prompt":    []acpPromptText{{Type: "text", Text: prompt}},
	})
	if callErr != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "session/prompt failed: "+callErr.Error()+" "+stderr.String())
	}
	if rpcErr != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "session/prompt rejected: "+rpcErr.Error())
	}
	var promptResult struct {
		StopReason string          `json:"stopReason"`
		Content    []acpPromptText `json:"content"`
	}
	if err := json.Unmarshal(raw, &promptResult); err != nil {
		return out, core.NewError(core.ReasonACPProbeFailed,
			"harness.RunACPSession", "session/prompt result unparseable: "+err.Error())
	}
	out.StopReason = promptResult.StopReason
	var parts []string
	for _, c := range promptResult.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	out.Content = strings.Join(parts, "\n")
	out.AgentName = "acp:" + filepath_Base(argv[0])
	return out, nil
}

// filepath_Base is a tiny helper kept local so this file does not pull
// path/filepath into the ACP flow's import set for one call.
func filepath_Base(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// newACPRPCClient wires the line-delimited JSON-RPC reader over a
// process's pipes. Kept separate from the probe so the probe's
// behaviour is untouched.
func newACPRPCClient(stdin io.WriteCloser, stdout io.Reader) *acpRPCClient {
	c := &acpRPCClient{stdin: stdin, lines: make(chan string, 64), closed: make(chan struct{})}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
		close(c.closed)
	}()
	return c
}

// acpPromptFromContext reads the prompt out of a stage-context file.
// When the file is absent or carries no prompt, the fallback is the
// stage name — never a fabricated answer.
func acpPromptFromContext(contextPath, stage string) string {
	if contextPath != "" {
		if raw, err := os.ReadFile(contextPath); err == nil {
			var sc struct {
				Prompt string `json:"prompt"`
			}
			if json.Unmarshal(raw, &sc) == nil && strings.TrimSpace(sc.Prompt) != "" {
				return sc.Prompt
			}
		}
	}
	if stage != "" {
		return "Execute the " + stage + " stage."
	}
	return ""
}
