package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"sync"

	"beacon-go/internal/agentapi"
	"beacon-go/internal/llm"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/session"
	"beacon-go/internal/workspace"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend
var shellFS embed.FS

// App is the Wails binding surface. Every method delegates to the same
// agentapi object the web console dispatches to — no second copy of any
// behaviour lives here.
type App struct {
	mu  sync.Mutex
	ctx context.Context
	api *agentapi.API
}

// NewApp builds the binding over BEACON_PROJECT_ROOT (default: the
// process working directory, which `beacon ui` sets to the project it
// was invoked from).
func NewApp() (*App, error) {
	root := os.Getenv("BEACON_PROJECT_ROOT")
	if root == "" {
		root = "."
	}
	return newApp(agentapi.Options{Root: root})
}

// newApp is the testable constructor: it takes the store paths so the
// binding tests never touch the operator's real ledger.
func newApp(opts agentapi.Options) (*App, error) {
	app := &App{}
	opts.Emit = func(event string, payload any) {
		app.mu.Lock()
		ctx := app.ctx
		app.mu.Unlock()
		if ctx != nil {
			wailsRuntime.EventsEmit(ctx, event, payload)
		}
	}
	api, err := agentapi.New(opts)
	if err != nil {
		return nil, err
	}
	app.api = api
	return app, nil
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.api.Close()
}

// ---- bindings (thin, one line each) ----

// GetProjectInfo reports the workspace root and its registration state.
func (a *App) GetProjectInfo() agentapi.ProjectInfo { return a.api.GetProjectInfo() }

// GetSessions lists conversation threads.
func (a *App) GetSessions() ([]session.Session, error) { return a.api.GetSessions() }

// CreateSession starts a new thread.
func (a *App) CreateSession(title string) (*session.Session, error) {
	return a.api.CreateSession(title)
}

// GetSessionMessages replays one thread.
func (a *App) GetSessionMessages(id string) ([]session.ChatMessage, error) {
	return a.api.GetSessionMessages(id)
}

// GetFeatures lists registered requirement packages.
func (a *App) GetFeatures() ([]workspace.FeatureView, error) { return a.api.GetFeatures() }

// GetFeatureContent returns one package's Markdown and gate verdicts.
func (a *App) GetFeatureContent(slug string) (workspace.FeatureDetail, error) {
	return a.api.GetFeatureContent(slug)
}

// AlignIntentStream restates the request and streams tokens as events.
func (a *App) AlignIntentStream(sessionID, prompt string) (*orchestrator.Briefing, error) {
	return a.api.AlignIntentStream(sessionID, prompt)
}

// StartGoal creates the real package and goal run.
func (a *App) StartGoal(sessionID, slug, utterance string) (orchestrator.RunView, error) {
	return a.api.StartGoal(sessionID, slug, utterance)
}

// GetGoalStatus re-reads one run from the ledger.
func (a *App) GetGoalStatus(runID string) (orchestrator.RunView, error) {
	return a.api.GetGoalStatus(runID)
}

// ReleaseGoal evaluates the release gate and completes only when it passes.
func (a *App) ReleaseGoal(sessionID, runID string) (workspace.ReleaseResult, error) {
	return a.api.ReleaseGoal(sessionID, runID)
}

// Executors lists the agent backends this machine can run (and which
// ones cannot start here).
func (a *App) Executors() []workspace.ExecutorOption { return a.api.Executors() }

// UseExecutor binds an agent backend for subsequent advances.
func (a *App) UseExecutor(profileName string, argv []string) (string, error) {
	return a.api.UseExecutor(profileName, argv)
}

// AdvanceRun pushes the run one step. A park is a real answer, not a
// failure to hide: the shell renders the reason code as-is.
func (a *App) AdvanceRun(sessionID, runID string) (orchestrator.RunView, error) {
	return a.api.AdvanceRun(sessionID, runID)
}

// GetSettings returns the BYOK configuration.
func (a *App) GetSettings() *llm.Config { return a.api.GetSettings() }

// SaveSettings persists the configuration.
func (a *App) SaveSettings(cfg llm.Config) error { return a.api.SaveSettings(cfg) }

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatalf("beacon-desktop: %v", err)
	}
	assets, err := fs.Sub(shellFS, "frontend")
	if err != nil {
		log.Fatalf("beacon-desktop: embedded shell: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "Beacon Agent Workspace",
		Width:  1320,
		Height: 840,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "Beacon Agent Desktop",
				Message: "Truth-first multi-agent coding workspace",
			},
		},
	})
	if err != nil {
		log.Fatalf("beacon-desktop: %v", err)
	}
}
