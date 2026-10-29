package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"beacon-go/internal/core"
)

// ACP probe (AC-046, PROBE LEVEL ONLY).
//
// The Agent Client Protocol is JSON-RPC 2.0 over stdio: the client
// spawns the agent process, sends an `initialize` request, then
// opens a session with `session/new`. What is implemented here is
// exactly and only that two-step handshake probe:
//
//   - spawn the configured target command (argv, no shell);
//   - send an initialize JSON-RPC request on stdin and await the
//     response with the matching id (notifications interleaved are
//     tolerated), bounded by a timeout;
//   - then send a session/new request and await its response,
//     reporting whether a session id came back;
//   - report how far the handshake got: StoppedAt names the step
//     that failed ("initialize" or "session/new").
//
// What the probe does NOT cover: reverse requests
// (fs/terminal/permission) and session lifecycle beyond the first
// prompt. The probe result itself remains a probe — FullBackend is
// always false, and no "supports ACP" claim exists on this surface.
//
// session/prompt has a real implementation in acp_session.go:
// RunACPSession opens a session, sends one prompt and maps the agent's
// own answer onto the AC-042 result schema. AC-046 asks for an
// integration iteration against a REAL ACP agent binary, and this
// repository has no such target, so that evidence is still
// outstanding: the implementation can only be proven against a real
// agent, and until one is available the honest state is "implemented,
// not yet proven against a real target".

// ACPProbeResult is the probe's evidence record.
type ACPProbeResult struct {
	Probe                bool   `json:"probe"` // always true: this is a probe, not a backend
	Target               string `json:"target"`
	Command              string `json:"command"`
	Responded            bool   `json:"responded"` // initialize answered
	ProtocolVersion      int    `json:"protocol_version,omitempty"`
	HasAgentCapabilities bool   `json:"has_agent_capabilities"`
	HasAuthMethods       bool   `json:"has_auth_methods"`
	ImplementationName   string `json:"implementation_name,omitempty"`
	SessionCreated       bool   `json:"session_created"` // session/new answered with a session id
	SessionID            string `json:"session_id,omitempty"`
	StoppedAt            string `json:"stopped_at,omitempty"` // step that failed, when one did
	Detail               string `json:"detail,omitempty"`
	ReasonCode           string `json:"reason_code,omitempty"`
	FullBackend          bool   `json:"full_backend"` // always false
}

// ACP request ids. Fixed: one probe process, one request per step.
const (
	acpInitializeID = 1
	acpSessionNewID = 2
)

// rpcError is one JSON-RPC error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Message) }

// acpRPCClient is the probe's minimal line-delimited JSON-RPC
// reader: one goroutine pumps stdout lines into a channel, calls
// consume until the response with their id arrives.
type acpRPCClient struct {
	stdin  interface{ Write([]byte) (int, error) }
	lines  chan string
	closed chan struct{}
}

func (c *acpRPCClient) call(ctx context.Context, id int, method string, params map[string]any) (json.RawMessage, *rpcError, error) {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})
	if err != nil {
		return nil, nil, err
	}
	if _, err := c.stdin.Write(append(req, '\n')); err != nil {
		return nil, nil, fmt.Errorf("could not write %s request: %w", method, err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("no %s response within the probe timeout", method)
		case <-c.closed:
			return nil, nil, fmt.Errorf("target closed stdout without a %s response", method)
		case line := <-c.lines:
			var msg struct {
				ID     *int            `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *rpcError       `json:"error"`
			}
			if err := json.Unmarshal([]byte(line), &msg); err != nil || msg.ID == nil {
				continue // notification or noise: keep waiting for our id
			}
			if *msg.ID != id {
				continue
			}
			if msg.Error != nil {
				return nil, msg.Error, nil
			}
			return msg.Result, nil, nil
		}
	}
}

// ProbeACP runs the two-step handshake (initialize, then
// session/new) against argv and reports what actually happened. A
// target that answers nothing, answers garbage, or errors a step
// yields a result with Responded/SessionCreated marking how far it
// got and Detail spelling out the failure — a probe failure is
// data, not a crash, so it is reported in the result, not returned
// as an error. An unspawnable command is the only error return
// (typed, fail-closed like every other executor).
func ProbeACP(ctx context.Context, argv []string, timeout time.Duration) (ACPProbeResult, error) {
	res := ACPProbeResult{Probe: true, FullBackend: false}
	if len(argv) == 0 {
		return res, core.NewError(core.ReasonExecutorNotConfigured,
			"harness.ProbeACP", "no ACP target command configured")
	}
	res.Command = argv[0]
	res.Target = argv[0]
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	resolved, err := exec.LookPath(argv[0])
	if err != nil {
		if _, statErr := os.Stat(argv[0]); statErr == nil {
			return res, core.NewError(core.ReasonExecutorCommandNotExec,
				"harness.ProbeACP", argv[0]+" exists but is not executable")
		}
		return res, core.NewError(core.ReasonExecutorCommandMissing,
			"harness.ProbeACP", "command not found: "+argv[0])
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, resolved, argv[1:]...)
	// Bound the wait: a killed target may leave a child holding the
	// stdout pipe open, which would otherwise block Wait for that
	// child's whole lifetime.
	cmd.WaitDelay = 3 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return res, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return res, err
	}
	if err := cmd.Start(); err != nil {
		return res, err
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()
	client := &acpRPCClient{stdin: stdin, lines: make(chan string, 64), closed: make(chan struct{})}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			client.lines <- sc.Text()
		}
		close(client.closed)
	}()

	// Step 1: initialize.
	raw, rpcErr, callErr := client.call(runCtx, acpInitializeID, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		// Version literal mirrors cli.Version (harness cannot import cli:
		// dependency direction is cli -> harness). Bump with the release.
		"clientInfo": map[string]any{"name": "beacon-go-probe", "version": "2.0.0-go.4"},
	})
	if callErr != nil {
		res.ReasonCode = core.ReasonACPProbeFailed
		res.StoppedAt = "initialize"
		res.Detail = callErr.Error()
		return res, nil
	}
	if rpcErr != nil {
		res.ReasonCode = core.ReasonACPProbeFailed
		res.StoppedAt = "initialize"
		res.Detail = "initialize rejected: " + rpcErr.Error()
		return res, nil
	}
	var initResult struct {
		ProtocolVersion   int             `json:"protocolVersion"`
		AgentCapabilities json.RawMessage `json:"agentCapabilities"`
		AuthMethods       json.RawMessage `json:"authMethods"`
		AgentInfo         *struct {
			Name string `json:"name"`
		} `json:"agentInfo"`
	}
	if err := json.Unmarshal(raw, &initResult); err != nil {
		res.ReasonCode = core.ReasonACPProbeFailed
		res.StoppedAt = "initialize"
		res.Detail = "initialize result unparseable: " + err.Error()
		return res, nil
	}
	res.Responded = true
	res.ProtocolVersion = initResult.ProtocolVersion
	res.HasAgentCapabilities = len(initResult.AgentCapabilities) > 0 && string(initResult.AgentCapabilities) != "null"
	res.HasAuthMethods = len(initResult.AuthMethods) > 0 && string(initResult.AuthMethods) != "null"
	if initResult.AgentInfo != nil {
		res.ImplementationName = initResult.AgentInfo.Name
	}

	// Step 2: session/new. The probe asks for a session but never
	// prompts it: creating the session proves the target speaks
	// the session surface; driving it is the unimplemented part.
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}
	raw, rpcErr, callErr = client.call(runCtx, acpSessionNewID, "session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
	})
	if callErr != nil {
		res.ReasonCode = core.ReasonACPProbeFailed
		res.StoppedAt = "session/new"
		res.Detail = "initialize answered; session/new failed: " + callErr.Error() +
			"; session/prompt is NOT implemented (probe only)"
		return res, nil
	}
	if rpcErr != nil {
		res.ReasonCode = core.ReasonACPProbeFailed
		res.StoppedAt = "session/new"
		res.Detail = "initialize answered; session/new rejected: " + rpcErr.Error() +
			"; session/prompt is NOT implemented (probe only)"
		return res, nil
	}
	var sessionResult struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &sessionResult); err != nil || sessionResult.SessionID == "" {
		res.ReasonCode = core.ReasonACPProbeFailed
		res.StoppedAt = "session/new"
		res.Detail = "initialize answered; session/new result carried no sessionId" +
			"; session/prompt is NOT implemented (probe only)"
		return res, nil
	}
	res.SessionCreated = true
	res.SessionID = sessionResult.SessionID
	res.Detail = "initialize + session/new handshake answered (session id returned); " +
		"session/prompt is NOT implemented (probe only)"
	return res, nil
}

// AcpBackend is the AC-046 slot, at probe level only. It satisfies
// the AgentBackend SHAPE so surfaces can reference it, but Iterate
// refuses — honestly, with a reason code — instead of fabricating an
// iteration. HealthCheck runs the two-step handshake probe.
type AcpBackend struct {
	argv    []string
	timeout time.Duration
}

// NewAcpBackend binds the probe to a target command.
func NewAcpBackend(argv []string, timeout time.Duration) *AcpBackend {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &AcpBackend{argv: argv, timeout: timeout}
}

// Name implements AgentBackend: acp:<target>.
func (b *AcpBackend) Name() string {
	if len(b.argv) == 0 {
		return "acp:unconfigured"
	}
	return "acp:" + b.argv[0]
}

// Iterate drives one real ACP session against the configured target:
// initialize, session/new, then a single session/prompt. The agent's
// own answer becomes the iteration result.
//
// A missing target, a refused handshake or an unanswered prompt comes
// back as an error with a reason code — the backend never fabricates
// an IterationResult. The prompt is taken from the stage-context file
// when one was handed over; otherwise it falls back to the stage name.
func (b *AcpBackend) Iterate(ctx context.Context, req IterationRequest) (IterationResult, error) {
	prompt := acpPromptFromContext(req.ContextPath, req.Stage)
	res, err := RunACPSession(ctx, b.argv, prompt, req.Workdir, b.timeout)
	if err != nil {
		return IterationResult{}, err
	}
	success := res.StopReason == "" || res.StopReason == "end_turn" || res.StopReason == "stop"
	return IterationResult{
		Success:        success,
		Summary:        res.Content,
		StopSignal:     res.StopReason,
		AgentName:      res.AgentName,
		StdoutText:     res.Content,
		KeyChangesMade: nil,
	}, nil
}

// HealthCheck runs the two-step handshake probe.
func (b *AcpBackend) HealthCheck(ctx context.Context) HealthStatus {
	res, err := ProbeACP(ctx, b.argv, b.timeout)
	if err != nil {
		return HealthStatus{Backend: b.Name(), OK: false, Detail: err.Error()}
	}
	return HealthStatus{Backend: b.Name(), OK: res.Responded && res.SessionCreated, Detail: res.Detail}
}

// Abort: no session is ever driven at probe level; a created probe
// session dies with its process.
func (b *AcpBackend) Abort(ctx context.Context) error { return nil }

// ProbeTimeout exposes the probe's bounded wait (used in help text).
func (b *AcpBackend) ProbeTimeout() time.Duration { return b.timeout }
