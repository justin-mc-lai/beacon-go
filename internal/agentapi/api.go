// Package agentapi is the shared backend of the two UI shells: the Wails
// desktop app and the single-binary web console. Both bind exactly these
// methods, so the two surfaces can never drift into different behaviour
// (which is how the web copy and the desktop copy diverged before).
package agentapi

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"beacon-go/internal/llm"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/session"
	"beacon-go/internal/workspace"
)

// Options configures New. Empty paths resolve through the package
// defaults (global ledger, ~/.beacon/sessions.db).
type Options struct {
	Root        string
	StorePath   string
	SessionPath string
	// Emit pushes a streaming/event payload to the shell. The desktop
	// shell forwards to Wails events, the web shell to SSE. Nil is safe.
	Emit func(event string, payload any)
}

// API is the one object both shells expose.
type API struct {
	Root string

	mu       sync.RWMutex
	cfg      *llm.Config
	cos      *orchestrator.ChiefOfStaff
	sessions *session.SQLiteStore
	ws       *workspace.Service
	emit     func(string, any)
}

// ProjectInfo tells the shell which root it is looking at and whether
// the ledger already knows it.
type ProjectInfo struct {
	Root       string `json:"root"`
	Version    string `json:"version"`
	Registered bool   `json:"registered"`
}

// New builds the API. A missing LLM key is not fatal: alignment returns
// a real error, which the UI shows instead of a fabricated brief.
func New(opts Options) (*API, error) {
	root := opts.Root
	if root == "" {
		return nil, errors.New("agentapi: empty project root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	cfg, err := llm.LoadConfig()
	if err != nil {
		return nil, err
	}
	sess, err := session.NewStore(opts.SessionPath)
	if err != nil {
		return nil, err
	}
	ws, err := workspace.Open(workspace.Options{Root: abs, StorePath: opts.StorePath})
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	api := &API{
		Root:     abs,
		cfg:      cfg,
		sessions: sess,
		ws:       ws,
		emit:     opts.Emit,
	}
	api.rebuildChief()
	return api, nil
}

// Close releases both stores.
func (a *API) Close() error {
	err := a.ws.Close()
	if cerr := a.sessions.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}

func (a *API) rebuildChief() {
	cli, model, _ := a.cfg.GetClientForRole(llm.RoleChiefOfStaff)
	a.cos = orchestrator.NewChiefOfStaff(cli, model)
}

func (a *API) chief() *orchestrator.ChiefOfStaff {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cos
}

func (a *API) send(event string, payload any) {
	if a.emit != nil {
		a.emit(event, payload)
	}
}

// GetProjectInfo reports the workspace root and its registration state.
func (a *API) GetProjectInfo() ProjectInfo {
	return ProjectInfo{Root: a.Root, Version: a.ws.Version, Registered: a.ws.Registered()}
}

// GetSessions lists conversation threads.
func (a *API) GetSessions() ([]session.Session, error) { return a.sessions.ListSessions() }

// CreateSession starts a new thread.
func (a *API) CreateSession(title string) (*session.Session, error) {
	return a.sessions.CreateSession(title)
}

// GetSessionMessages returns one thread's turns for replay.
func (a *API) GetSessionMessages(sessionID string) ([]session.ChatMessage, error) {
	return a.sessions.GetSessionMessages(sessionID)
}

// GetFeatures lists requirement packages from the ledger.
func (a *API) GetFeatures() ([]workspace.FeatureView, error) { return a.ws.Features() }

// GetFeatureContent returns the Markdown + gate verdicts of one package.
func (a *API) GetFeatureContent(slug string) (workspace.FeatureDetail, error) {
	return a.ws.Detail(slug)
}

// AlignIntentStream restates the request and streams tokens to the shell.
func (a *API) AlignIntentStream(sessionID, prompt string) (*orchestrator.Briefing, error) {
	if sessionID == "" {
		sess, err := a.sessions.CreateSession("新需求任务")
		if err != nil {
			return nil, err
		}
		sessionID = sess.ID
	}
	if err := a.sessions.AppendMessage(session.ChatMessage{
		SessionID: sessionID, Role: "user", Content: prompt,
	}); err != nil {
		return nil, err
	}
	brief, err := a.chief().AlignIntentStream(context.Background(), prompt, func(chunk string) error {
		a.send("chat:stream:chunk", map[string]string{"session_id": sessionID, "chunk": chunk})
		return nil
	})
	if err != nil {
		// The user turn stays recorded; the error is surfaced, not hidden.
		return nil, err
	}
	payload, _ := json.Marshal(map[string]any{
		"slug":     brief.ProposedSlugs[0],
		"degraded": brief.Degraded,
	})
	if err := a.sessions.AppendMessage(session.ChatMessage{
		SessionID: sessionID, Role: "chief", Content: brief.RestatedIntent,
		CardType: "plan_brief", CardPayload: string(payload),
	}); err != nil {
		return nil, err
	}
	return brief, nil
}

// StartGoal creates the real package (if absent) and the real goal run,
// then records a goal_run card carrying the ledger facts.
func (a *API) StartGoal(sessionID, slug, utterance string) (orchestrator.RunView, error) {
	view, err := a.ws.StartGoal(slug, utterance, "desktop-operator")
	if err != nil {
		return orchestrator.RunView{}, err
	}
	payload, _ := json.Marshal(goalCard{
		RunID:     view.RunID,
		Slug:      view.FeatureSlug,
		Status:    view.Status,
		LoopStage: view.LoopStage,
		Stages:    view.StagePlan,
	})
	_ = a.sessions.AppendMessage(session.ChatMessage{
		SessionID: sessionID, Role: "system", Content: "goal run created: " + view.RunID,
		CardType: "goal_run", CardPayload: string(payload),
	})
	return view, nil
}

// GetGoalStatus re-reads one run from the ledger.
func (a *API) GetGoalStatus(runID string) (orchestrator.RunView, error) {
	return a.ws.Goal(runID)
}

// ReleaseGoal evaluates the release gate and completes only when it passes.
func (a *API) ReleaseGoal(sessionID, runID string) (workspace.ReleaseResult, error) {
	res, err := a.ws.Release(runID, "desktop-operator")
	if err != nil {
		return workspace.ReleaseResult{}, err
	}
	payload, _ := json.Marshal(res)
	_ = a.sessions.AppendMessage(session.ChatMessage{
		SessionID: sessionID, Role: "system", Content: "release verdict: " + res.Verdict,
		CardType: "release_verdict", CardPayload: string(payload),
	})
	return res, nil
}

// GetSettings returns the BYOK configuration.
func (a *API) GetSettings() *llm.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// SaveSettings persists the configuration and rebuilds the LLM client.
func (a *API) SaveSettings(cfg llm.Config) error {
	if err := cfg.SaveConfig(); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = &cfg
	a.rebuildChief()
	return nil
}

// Executors lists the agent backends this machine can actually run.
// One whose command is missing is returned with Available=false so the
// shell can grey it out instead of offering a backend that cannot start.
func (a *API) Executors() []workspace.ExecutorOption { return a.ws.Executors() }

// UseExecutor binds an agent backend for later advances. It returns the
// executor id now registered; an unknown profile is an error, never a
// silent fallback to another backend.
func (a *API) UseExecutor(profileName string, argv []string) (string, error) {
	return a.ws.UseExecutor(profileName, argv)
}

// AdvanceRun pushes the run one step and records the real outcome,
// park included: with no executor bound the run stays blocked with
// stage_executor_not_registered, which the shell must show as a park
// rather than as completion.
func (a *API) AdvanceRun(sessionID, runID string) (orchestrator.RunView, error) {
	view, err := a.ws.Advance(runID)
	if err != nil {
		return orchestrator.RunView{}, err
	}
	payload, _ := json.Marshal(map[string]any{
		"run_id":      view.RunID,
		"status":      view.Status,
		"loop_stage":  view.LoopStage,
		"reason_code": view.ReasonCode,
		"blocked":     view.Blocked,
	})
	_ = a.sessions.AppendMessage(session.ChatMessage{
		SessionID: sessionID, Role: "system", Content: "advance: " + view.LoopStage,
		CardType: "run_advance", CardPayload: string(payload),
	})
	return view, nil
}

// goalCard is the persisted shape of a goal_run card, so replay shows
// exactly the stages the ledger held at creation time.
type goalCard struct {
	RunID     string                        `json:"run_id"`
	Slug      string                        `json:"slug"`
	Status    string                        `json:"status"`
	LoopStage string                        `json:"loop_stage"`
	Stages    []orchestrator.StagePlanEntry `json:"stages"`
}
