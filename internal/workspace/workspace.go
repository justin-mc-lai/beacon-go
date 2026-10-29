// Package workspace is the one backend surface the desktop app and the
// web console both bind to. It owns the three things a UI may honestly
// claim: which project it is looking at, which requirement packages
// exist, and which goal run is actually in the ledger.
//
// Nothing here invents state. A missing registration, a missing package,
// a gate that has not passed, or an engine with no executor registered
// all come back as errors or as real ledger facts — never as a rendered
// success.
package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

// Options configures Open. Root defaults to the process working
// directory; StorePath defaults to the global ledger (BEACON_STORE_PATH
// or ~/.beacon/beacon-go.db). Tests inject both.
type Options struct {
	Root      string
	Version   string
	StorePath string
}

// Service is a handle over one project root plus the global ledger.
type Service struct {
	Root    string
	Version string

	st  *store.Store
	eng *orchestrator.Engine
}

// FeatureView is one requirement package as the UI may show it.
type FeatureView struct {
	Slug        string `json:"slug"`
	Version     string `json:"version"`
	Title       string `json:"title"`
	Stage       string `json:"stage"`
	Status      string `json:"status"`
	Frozen      bool   `json:"frozen"`
	Registered  bool   `json:"registered"`
	HasTruth    bool   `json:"has_truth"`
	HasTests    bool   `json:"has_tests"`
	HasTasks    bool   `json:"has_tasks"`
	HasEvidence bool   `json:"has_evidence"`
}

// FeatureDetail is the right-hand context panel payload.
type FeatureDetail struct {
	FeatureView
	TruthMD  string             `json:"truth_md"`
	TestsMD  string             `json:"tests_md"`
	TasksMD  string             `json:"tasks_md"`
	Evidence string             `json:"evidence_md"`
	Gates    []store.GateResult `json:"gates"`
}

// ReleaseResult reports what the release gate actually said. Completed
// is true only when the engine closed the run to done.
type ReleaseResult struct {
	RunID     string   `json:"run_id"`
	Verdict   string   `json:"verdict"`
	Reasons   []string `json:"reason_codes"`
	Completed bool     `json:"completed"`
	Detail    string   `json:"detail,omitempty"`
}

// Open opens the service. The project does not have to be registered
// yet — EnsureRegistered does that on first write.
func Open(opts Options) (*Service, error) {
	root := store.ResolveRootPath(opts.Root)
	if root == "" {
		return nil, errors.New("workspace: empty project root")
	}
	path := opts.StorePath
	if path == "" {
		p, err := store.DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	st, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	svc := &Service{
		Root:    root,
		Version: opts.Version,
		st:      st,
		eng:     orchestrator.NewEngine(st),
	}
	// A registered project already knows its current version; surface it
	// so the shell can show it before any write happens.
	if svc.Version == "" {
		if p, err := st.Projects().GetByRootPath(root); err == nil {
			if v, err := st.Projects().CurrentVersion(p.ID); err == nil {
				svc.Version = v.Version
			}
		}
	}
	return svc, nil
}

// Close releases the ledger handle.
func (s *Service) Close() error { return s.st.Close() }

// Store exposes the ledger handle for read-only callers (spine views).
func (s *Service) Store() *store.Store { return s.st }

// Registered reports whether this root already exists in the ledger.
func (s *Service) Registered() bool {
	_, err := s.st.Projects().GetByRootPath(s.Root)
	return err == nil
}

// EnsureRegistered registers the project root (idempotent) and makes
// sure at least one docs version is recorded. Registering is a local,
// reversible bookkeeping write, which is why the UI may do it when the
// operator explicitly starts a goal.
func (s *Service) EnsureRegistered() (store.Project, error) {
	p, err := s.st.Projects().GetByRootPath(s.Root)
	if errors.Is(err, sql.ErrNoRows) {
		p, err = s.st.Projects().Register(store.Project{
			RootPath: s.Root,
			Name:     filepath.Base(s.Root),
		})
		if err != nil {
			return store.Project{}, err
		}
	} else if err != nil {
		return store.Project{}, err
	}
	if err := s.ensureVersion(p); err != nil {
		return store.Project{}, err
	}
	return p, nil
}

func (s *Service) ensureVersion(p store.Project) error {
	versions, err := s.st.Projects().ListVersions(p.ID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if s.Version != "" && v.Version == s.Version && !v.IsCurrent {
			return s.st.Projects().SetCurrent(v.ID)
		}
		if s.Version == "" && v.IsCurrent {
			s.Version = v.Version
			return nil
		}
	}
	if s.Version == "" {
		v, err := s.detectVersion()
		if err != nil {
			return err
		}
		s.Version = v
	}
	for _, v := range versions {
		if v.Version == s.Version {
			return nil
		}
	}
	_, err = s.st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID,
		Version:   s.Version,
		DocsRoot:  filepath.ToSlash(filepath.Join("docs", "beacon", s.Version)),
		IsCurrent: true,
	})
	return err
}

// detectVersion picks the newest docs/beacon/<version> directory. It
// fails closed instead of guessing a version that is not on disk.
func (s *Service) detectVersion() (string, error) {
	base := filepath.Join(s.Root, "docs", "beacon")
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", core.NewError(orchestrator.ReasonFeatureNotFound, "workspace.detectVersion",
			"no docs/beacon directory under "+s.Root+"; create a requirement package first")
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			found = append(found, e.Name())
		}
	}
	if len(found) == 0 {
		return "", core.NewError(orchestrator.ReasonFeatureNotFound, "workspace.detectVersion",
			"no version directory under "+base)
	}
	sort.Strings(found)
	return found[len(found)-1], nil
}

// Features lists the packages the ledger knows about for this root. A
// project that was never registered falls back to a read-only disk scan
// and marks each package `registered=false` — the UI shows what exists
// without pretending it is tracked.
func (s *Service) Features() ([]FeatureView, error) {
	ctxs, err := s.st.ListFeatureContexts(s.Root)
	if err != nil {
		return nil, err
	}
	out := make([]FeatureView, 0, len(ctxs))
	for _, c := range ctxs {
		v := s.viewOf(c)
		v.Registered = true
		out = append(out, v)
	}
	if len(out) > 0 {
		return out, nil
	}
	return s.scanPackages()
}

// scanPackages lists docs/beacon/<version>/features/* without touching
// the ledger. It is the unregistered-project view.
func (s *Service) scanPackages() ([]FeatureView, error) {
	version := s.Version
	if version == "" {
		v, err := s.detectVersion()
		if err != nil {
			return []FeatureView{}, nil // no docs tree at all: genuinely empty
		}
		version = v
	}
	dir := filepath.Join(s.Root, "docs", "beacon", version, "features")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []FeatureView{}, nil
	}
	out := []FeatureView{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pkg := filepath.Join(dir, e.Name())
		truth := readFile(filepath.Join(pkg, "truth.md"))
		out = append(out, FeatureView{
			Slug:        e.Name(),
			Version:     version,
			Title:       strings.ReplaceAll(e.Name(), "-", " "),
			Status:      "unregistered",
			Frozen:      isFrozenText(truth),
			HasTruth:    truth != "",
			HasTests:    fileExists(filepath.Join(pkg, "tests.md")),
			HasTasks:    fileExists(filepath.Join(pkg, "tasks.md")),
			HasEvidence: fileExists(filepath.Join(pkg, "evidence.md")),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// isFrozenText reads the status marker a truth package carries in its
// header. It is a display hint only — freezing authority stays in the
// ledger once the package is adopted.
func isFrozenText(truth string) bool {
	head := truth
	if len(head) > 1200 {
		head = head[:1200]
	}
	return strings.Contains(strings.ToUpper(head), "FROZEN")
}

func (s *Service) viewOf(c store.FeatureContext) FeatureView {
	v := FeatureView{
		Slug:    c.Feature.Slug,
		Version: c.Version.Version,
		Title:   strings.ReplaceAll(c.Feature.Slug, "-", " "),
		Frozen:  c.Feature.Status == "frozen",
		Status:  c.Feature.Status,
	}
	if lc, err := s.st.Features().GetLifecycle(c.Feature.ID); err == nil {
		v.Stage = string(lc.Stage)
		v.Status = string(lc.Status)
	}
	dir := s.packageDir(c.Feature.TruthPath)
	v.HasTruth = fileExists(filepath.Join(dir, "truth.md"))
	v.HasTests = fileExists(filepath.Join(dir, "tests.md"))
	v.HasTasks = fileExists(filepath.Join(dir, "tasks.md"))
	v.HasEvidence = fileExists(filepath.Join(dir, "evidence.md"))
	return v
}

// Detail reads one package's Markdown plus its latest gate verdicts.
// An unregistered package is read from disk (no gate verdicts exist).
func (s *Service) Detail(slug string) (FeatureDetail, error) {
	ctx, err := s.st.ResolveFeature(s.Root, slug, "")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s.detailFromDisk(slug)
		}
		return FeatureDetail{}, err
	}
	dir := s.packageDir(ctx.Feature.TruthPath)
	d := FeatureDetail{
		FeatureView: s.viewOf(ctx),
		TruthMD:     readFile(filepath.Join(dir, "truth.md")),
		TestsMD:     readFile(filepath.Join(dir, "tests.md")),
		TasksMD:     readFile(filepath.Join(dir, "tasks.md")),
		Evidence:    readFile(filepath.Join(dir, "evidence.md")),
	}
	if gates, err := s.st.Gates().LatestByType(ctx.Feature.ID); err == nil {
		d.Gates = gates
	}
	return d, nil
}

// detailFromDisk is the read path for a package the ledger does not
// track yet. It never invents gate verdicts.
func (s *Service) detailFromDisk(slug string) (FeatureDetail, error) {
	version := s.Version
	if version == "" {
		v, err := s.detectVersion()
		if err != nil {
			return FeatureDetail{}, err
		}
		version = v
	}
	dir := filepath.Join(s.Root, "docs", "beacon", version, "features", slug)
	if !fileExists(filepath.Join(dir, "truth.md")) && !fileExists(filepath.Join(dir, "tests.md")) {
		return FeatureDetail{}, core.NewError(orchestrator.ReasonFeatureNotFound, "workspace.Detail",
			"feature "+slug+" is neither registered nor present on disk")
	}
	truth := readFile(filepath.Join(dir, "truth.md"))
	return FeatureDetail{
		FeatureView: FeatureView{
			Slug: slug, Version: version, Title: strings.ReplaceAll(slug, "-", " "),
			Status: "unregistered", Frozen: isFrozenText(truth),
			HasTruth: truth != "", HasTests: fileExists(filepath.Join(dir, "tests.md")),
			HasTasks:    fileExists(filepath.Join(dir, "tasks.md")),
			HasEvidence: fileExists(filepath.Join(dir, "evidence.md")),
		},
		TruthMD:  truth,
		TestsMD:  readFile(filepath.Join(dir, "tests.md")),
		TasksMD:  readFile(filepath.Join(dir, "tasks.md")),
		Evidence: readFile(filepath.Join(dir, "evidence.md")),
		Gates:    []store.GateResult{},
	}, nil
}

// StartGoal is the only write path the UI has: it makes sure the
// requirement package exists, then creates a real goal run. The run is
// created even when no stage executor is registered — the engine will
// block honestly at the first stage instead of pretending to progress.
func (s *Service) StartGoal(slug, utterance, executorID string) (orchestrator.RunView, error) {
	if !slugRe.MatchString(slug) {
		return orchestrator.RunView{}, fmt.Errorf("workspace: %q is not a valid feature slug", slug)
	}
	if _, err := s.EnsureRegistered(); err != nil {
		return orchestrator.RunView{}, err
	}
	ctx, err := s.st.ResolveFeature(s.Root, slug, s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		// A package already on disk is adopted, never re-created over.
		if s.packageDirExists(slug) {
			ctx, err = s.adopt(slug)
		} else if _, cerr := s.eng.CreateTruthPackage(s.Root, s.Version, slug); cerr != nil {
			return orchestrator.RunView{}, cerr
		} else {
			ctx, err = s.st.ResolveFeature(s.Root, slug, s.Version)
		}
	}
	if err != nil {
		return orchestrator.RunView{}, err
	}
	return s.eng.CreateRun(orchestrator.CreateRunOptions{
		ProjectRoot: s.Root,
		Version:     ctx.Version.Version,
		FeatureSlug: slug,
		Utterance:   utterance,
		ExecutorID:  executorID,
	})
}

// adopt registers an existing on-disk package in the ledger so runs can
// reference it. The Markdown is never rewritten.
func (s *Service) adopt(slug string) (store.FeatureContext, error) {
	p, err := s.st.Projects().GetByRootPath(s.Root)
	if err != nil {
		return store.FeatureContext{}, err
	}
	versions, err := s.st.Projects().ListVersions(p.ID)
	if err != nil {
		return store.FeatureContext{}, err
	}
	var ver store.ProjectVersion
	for _, v := range versions {
		if v.Version == s.Version || (s.Version == "" && v.IsCurrent) {
			ver = v
			break
		}
	}
	if ver.ID == "" {
		return store.FeatureContext{}, core.NewError(orchestrator.ReasonFeatureNotFound,
			"workspace.adopt", "no docs version recorded for "+s.Root)
	}
	pkgRel := filepath.ToSlash(filepath.Join(ver.DocsRoot, "features", slug))
	feat, err := s.st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: ver.ID,
		Slug:             slug,
		TruthPath:        filepath.Join(pkgRel, "truth.md"),
		TestsPath:        filepath.Join(pkgRel, "tests.md"),
		Status:           "imported",
	})
	if err != nil {
		return store.FeatureContext{}, err
	}
	_ = s.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: feat.ID, Stage: core.StageThink, Status: core.StatusNotStarted,
	})
	return store.FeatureContext{Project: p, Version: ver, Feature: feat}, nil
}

func (s *Service) packageDirExists(slug string) bool {
	version := s.Version
	if version == "" {
		if v, err := s.detectVersion(); err == nil {
			version = v
		}
	}
	return fileExists(filepath.Join(s.Root, "docs", "beacon", version, "features", slug, "truth.md"))
}

// Goal returns the current ledger view of one run.
func (s *Service) Goal(runID string) (orchestrator.RunView, error) {
	return s.eng.Status(runID)
}

// Release evaluates the release gate for a run and, only when that gate
// passes, asks the engine to complete it. A blocked verdict is a normal
// result (Completed=false with the real reason codes), not an error.
func (s *Service) Release(runID, operator string) (ReleaseResult, error) {
	view, err := s.eng.Status(runID)
	if err != nil {
		return ReleaseResult{}, err
	}
	ctx, err := s.st.ResolveFeature(s.Root, view.FeatureSlug, "")
	if err != nil {
		return ReleaseResult{}, err
	}
	if operator == "" {
		operator = "human"
	}
	ev, err := s.eng.EvaluateGate(ctx, "release", runID, "human:"+operator)
	if err != nil {
		return ReleaseResult{}, err
	}
	res := ReleaseResult{
		RunID:   runID,
		Verdict: string(ev.Verdict),
		Reasons: ev.ReasonCodes,
	}
	if res.Verdict != "pass" {
		return res, nil
	}
	if _, err := s.eng.Complete(runID, orchestrator.CompleteOptions{
		HumanConfirmed: true,
		ConfirmedBy:    operator,
	}); err != nil {
		res.Detail = err.Error()
		if rc, ok := core.ReasonCodeOf(err); ok {
			res.Reasons = append(res.Reasons, rc)
		}
		return res, nil
	}
	res.Completed = true
	return res, nil
}

// Advance pushes the run exactly one step through the engine. Whatever
// the engine answers is what the caller gets back: with no executor
// registered the run parks with stage_executor_not_registered, and that
// park is reported as a real result instead of being rendered as progress.
func (s *Service) Advance(runID string) (orchestrator.RunView, error) {
	return s.eng.Tick(context.Background(), runID)
}

// ExecutorOption is one agent backend the UI may offer. Available is
// false when its command is not resolvable on this machine — the UI
// must show that rather than offering a backend that cannot run.
type ExecutorOption struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Argv        []string `json:"argv"`
	Available   bool     `json:"available"`
	Detail      string   `json:"detail,omitempty"`
}

// Executors lists the built-in agent backends and whether each one's
// command actually resolves here.
func (s *Service) Executors() []ExecutorOption {
	profs := harness.BuiltinProfiles()
	out := make([]ExecutorOption, 0, len(profs))
	for _, p := range profs {
		opt := ExecutorOption{Name: p.Name, Description: p.Description, Argv: p.Argv}
		if len(p.Argv) > 0 {
			if path, err := exec.LookPath(p.Argv[0]); err == nil {
				opt.Available = true
				opt.Detail = path
			} else {
				opt.Detail = "not on PATH"
			}
		}
		out = append(out, opt)
	}
	return out
}

// UseExecutor binds a stage executor built from a built-in agent
// profile, optionally overriding its command template. It returns the
// executor id that is now registered. An unknown profile is an error —
// the caller never silently falls back to some other backend.
func (s *Service) UseExecutor(profileName string, argv []string) (string, error) {
	prof, ok := harness.LookupProfile(profileName)
	if !ok {
		return "", fmt.Errorf("workspace: unknown executor profile %q", profileName)
	}
	if len(argv) > 0 {
		prof = prof.WithCommand(argv)
	}
	ex := orchestrator.NewProfileExecutor(s.eng, prof, 0)
	s.eng.RegisterExecutorForAllStages(ex)
	return ex.ID(), nil
}

func (s *Service) packageDir(truthPath string) string {
	rel := filepath.FromSlash(filepath.Dir(truthPath))
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(s.Root, rel)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
