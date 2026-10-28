package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/llm"
	"beacon-go/internal/orchestrator"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed frontend/index.html
var assets embed.FS

// App represents the Wails desktop backend service.
type App struct {
	ctx     context.Context
	cfg     *llm.Config
	cos     *orchestrator.ChiefOfStaff
	workdir string
}

func NewApp() *App {
	cfg, _ := llm.LoadConfig()
	cli, model, _ := cfg.GetClientForRole(llm.RoleChiefOfStaff)
	cos := orchestrator.NewChiefOfStaff(cli, model)
	cwd, _ := os.Getwd()
	return &App{
		cfg:     cfg,
		cos:     cos,
		workdir: cwd,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetSettings retrieves the current BYOK and Multi-Agent role config.
func (a *App) GetSettings() *llm.Config {
	return a.cfg
}

// SaveSettings updates and persists the BYOK and role configuration.
func (a *App) SaveSettings(cfg llm.Config) error {
	if err := cfg.SaveConfig(); err != nil {
		return err
	}
	a.cfg = &cfg
	cli, model, _ := a.cfg.GetClientForRole(llm.RoleChiefOfStaff)
	a.cos = orchestrator.NewChiefOfStaff(cli, model)
	return nil
}

// AlignIntent aligns user prompt using Chief of Staff.
func (a *App) AlignIntent(prompt string) (*orchestrator.Briefing, error) {
	return a.cos.AlignIntent(a.ctx, prompt)
}

// FeatureItem represents a feature record for the UI.
type FeatureItem struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
	Title   string `json:"title"`
}

// GetFeatures returns scanned feature items.
func (a *App) GetFeatures() []FeatureItem {
	var list []FeatureItem
	baseDir := filepath.Join(a.workdir, "docs", "beacon")
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && filepath.Base(filepath.Dir(path)) == "features" {
			slug := filepath.Base(path)
			ver := filepath.Base(filepath.Dir(filepath.Dir(path)))
			list = append(list, FeatureItem{
				Slug:    slug,
				Version: ver,
				Title:   strings.ReplaceAll(slug, "-", " "),
			})
		}
		return nil
	})
	return list
}

// GetFeatureContent reads the Markdown documents of a feature.
func (a *App) GetFeatureContent(slug string) map[string]string {
	res := map[string]string{}
	baseDir := filepath.Join(a.workdir, "docs", "beacon")
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && filepath.Base(path) == slug && filepath.Base(filepath.Dir(path)) == "features" {
			truth, _ := os.ReadFile(filepath.Join(path, "truth.md"))
			res["truth_md"] = string(truth)

			tests, _ := os.ReadFile(filepath.Join(path, "tests.md"))
			res["tests_md"] = string(tests)

			evDir := filepath.Join(a.workdir, ".beacon", "evidence", "implement", slug)
			if files, err := os.ReadDir(evDir); err == nil && len(files) > 0 {
				ev, _ := os.ReadFile(filepath.Join(evDir, files[0].Name()))
				res["evidence_json"] = string(ev)
			}
		}
		return nil
	})
	return res
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Beacon Agent Workspace",
		Width:  1280,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 255},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "Beacon Agent Desktop",
				Message: "Industrial Multi-Agent Coding Workspace",
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
