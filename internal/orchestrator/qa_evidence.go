package orchestrator

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

type qaFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type qaCandidate struct {
	Schema       string   `json:"schema"`
	RunID        string   `json:"run_id"`
	FeatureSlug  string   `json:"feature_slug"`
	TruthSHA256  string   `json:"truth_sha256"`
	TestsSHA256  string   `json:"tests_sha256"`
	SourceCommit string   `json:"source_commit"`
	SourceFiles  []qaFile `json:"source_files"`
	BuildCommand string   `json:"build_command"`
	Maker        string   `json:"maker"`
	VCSModified  *bool    `json:"vcs_modified,omitempty"`
	Artifacts    []qaFile `json:"artifacts"`
}
type qaRegistration struct {
	Kind          string   `json:"kind"`
	Path          string   `json:"abs_path"`
	Hash          string   `json:"content_hash"`
	By            string   `json:"registered_by"`
	CandidateHash string   `json:"candidate_hash"`
	EvidenceFiles []qaFile `json:"evidence_files,omitempty"`
}
type qaCurrent struct {
	qaCandidate
	Hash, Path string
}
type qaProductReview struct {
	Schema        string   `json:"schema"`
	RunID         string   `json:"run_id"`
	CandidateHash string   `json:"candidate_hash"`
	Scope         string   `json:"scope"`
	Verdict       string   `json:"verdict"`
	Maker         string   `json:"maker"`
	Checker       string   `json:"checker"`
	CoveredTCs    []string `json:"covered_tcs"`
	EvidenceRefs  []string `json:"evidence_refs"`
}

func qaError(code, detail string) error { return core.NewError(code, "qa.evidence", detail) }
func qaRead(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
func qaHash(path string) (string, error) {
	i, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !i.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digestOf(string(b)), nil
}
func qaFileMatches(root string, f qaFile) error {
	h, err := qaHash(resolvePkgPath(root, f.Path))
	if err != nil {
		return err
	}
	if h != f.SHA256 || f.SHA256 == "" {
		return fmt.Errorf("hash mismatch: %s", f.Path)
	}
	return nil
}
func qaGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	b, err := cmd.Output()
	return string(b), err
}

func (e *Engine) validateQACandidate(run store.Run, path, by string) (qaCurrent, error) {
	var c qaCandidate
	bad := func(err error) (qaCurrent, error) { return qaCurrent{}, qaError("qa_candidate_invalid", err.Error()) }
	if err := qaRead(path, &c); err != nil {
		return bad(err)
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return bad(err)
	}
	root := fc.Project.RootPath
	if c.Schema != "beacon-qa-candidate.v1" || c.RunID != run.ID || c.FeatureSlug != fc.Feature.Slug || strings.TrimSpace(c.Maker) == "" || c.Maker != strings.TrimSpace(by) || strings.TrimSpace(c.BuildCommand) == "" || len(c.Artifacts) == 0 || len(c.SourceFiles) == 0 {
		return bad(fmt.Errorf("candidate schema, run, feature, maker, build_command or files invalid"))
	}
	pkg := e.packageFiles(fc)
	for _, f := range []qaFile{{pkg.Truth.Path, c.TruthSHA256}, {pkg.Tests.Path, c.TestsSHA256}} {
		if err := qaFileMatches(root, f); err != nil {
			return bad(err)
		}
	}
	_, gitErr := qaGit(root, "rev-parse", "--show-toplevel")
	if gitErr != nil {
		for dir := root; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				return bad(fmt.Errorf("git source provenance is unreadable; cannot fall back to a non-git snapshot"))
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	changed := false
	if gitErr == nil {
		commit, err := qaGit(root, "rev-parse", "--verify", c.SourceCommit+"^{commit}")
		if err != nil || strings.TrimSpace(commit) != c.SourceCommit {
			return bad(fmt.Errorf("source_commit must be an exact existing implementation commit"))
		}
	} else if c.SourceCommit != "" {
		return bad(fmt.Errorf("non-git source snapshot requires empty source_commit"))
	}
	for _, f := range c.SourceFiles {
		p := filepath.Clean(f.Path)
		if filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) || strings.HasPrefix(p, "docs/") || strings.HasPrefix(p, ".beacon/") || strings.HasPrefix(p, "evidence/") || strings.Contains(p, "/evidence/") {
			return bad(fmt.Errorf("source path must be repository source: %s", p))
		}
		switch filepath.Ext(p) {
		case ".go", ".py", ".sh", ".js", ".ts", ".tsx", ".jsx", ".rs", ".c", ".h", ".cpp", ".java", ".rb":
		default:
			return bad(fmt.Errorf("not a source file: %s", p))
		}
		real, err := filepath.EvalSymlinks(filepath.Join(root, p))
		if err != nil {
			return bad(err)
		}
		rel, err := filepath.Rel(root, real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return bad(fmt.Errorf("source escapes project"))
		}
		if err := qaFileMatches(root, f); err != nil {
			return bad(err)
		}
		if gitErr == nil {
			b, err := qaGit(root, "show", c.SourceCommit+":"+filepath.ToSlash(p))
			if err != nil || digestOf(b) != f.SHA256 {
				return bad(fmt.Errorf("source does not match implementation commit: %s", p))
			}
			diff, err := qaGit(root, "diff-tree", "--root", "--no-commit-id", "--name-only", "-r", c.SourceCommit, "--", p)
			if err != nil {
				return bad(err)
			}
			if strings.TrimSpace(diff) != "" && !strings.HasSuffix(p, "_test.go") && !strings.Contains(p, "/testdata/") {
				changed = true
			}
		}
	}
	if gitErr == nil && !changed {
		return bad(fmt.Errorf("source_commit changes no declared implementation source (documentation-only commit is not a candidate)"))
	}
	for _, f := range c.Artifacts {
		if !filepath.IsAbs(f.Path) {
			return bad(fmt.Errorf("artifact path must be absolute"))
		}
		if err := qaFileMatches(root, f); err != nil {
			return bad(err)
		}
		if bi, err := buildinfo.ReadFile(f.Path); err == nil {
			rev, modified := "", ""
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" {
					rev = s.Value
				}
				if s.Key == "vcs.modified" {
					modified = s.Value
				}
			}
			if rev != "" {
				if rev != c.SourceCommit {
					return bad(fmt.Errorf("Go VCS revision differs from source_commit"))
				}
				if c.VCSModified == nil || fmt.Sprint(*c.VCSModified) != modified {
					return bad(fmt.Errorf("record exact Go vcs_modified in candidate"))
				}
			} else if gitErr == nil && !strings.Contains(c.BuildCommand, "-buildvcs=false") {
				return bad(fmt.Errorf("Go binary lacks VCS metadata: explicitly record -buildvcs=false limitation"))
			}
		}
	}
	h, err := qaHash(path)
	if err != nil {
		return bad(err)
	}
	return qaCurrent{qaCandidate: c, Hash: h, Path: path}, nil
}
func (e *Engine) currentQACandidate(run store.Run) (qaCurrent, error) {
	events, err := e.store.ListEventsByRun(run.ID)
	if err != nil {
		return qaCurrent{}, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != EventArtifactRegistered {
			continue
		}
		var p qaRegistration
		_ = json.Unmarshal(events[i].Payload, &p)
		if p.Kind != "qa_candidate" {
			continue
		}
		c, err := e.validateQACandidate(run, p.Path, p.By)
		if err != nil {
			return c, err
		}
		if c.Hash != p.Hash {
			return qaCurrent{}, qaError("qa_candidate_stale", "registered manifest changed; register a new revision")
		}
		return c, nil
	}
	return qaCurrent{}, qaError("qa_candidate_missing", "register the current run's qa_candidate first")
}
func (e *Engine) qaRegisteredFile(runID, path string) (store.ArtifactRef, error) {
	arts, err := e.store.Artifacts().ListByRun(runID)
	if err != nil {
		return store.ArtifactRef{}, err
	}
	for i := len(arts) - 1; i >= 0; i-- {
		a := arts[i]
		if a.AbsPath == path {
			if stale, why := store.ArtifactStaleness(a); stale {
				return a, qaError("qa_evidence_stale", path+": "+why)
			}
			return a, nil
		}
	}
	return store.ArtifactRef{}, qaError("qa_evidence_unregistered", path)
}
func (e *Engine) currentQAReview(run store.Run, m *runMeta, c qaCurrent) (qaProductReview, qaRegistration, error) {
	events, err := e.store.ListEventsByRun(run.ID)
	if err != nil {
		return qaProductReview{}, qaRegistration{}, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.EventType != EventArtifactRegistered {
			continue
		}
		var p qaRegistration
		_ = json.Unmarshal(ev.Payload, &p)
		if p.Kind != "qa_report" || p.CandidateHash != "" && p.CandidateHash != c.Hash {
			continue
		}
		var r qaProductReview
		bad := func(detail string) (qaProductReview, qaRegistration, error) {
			return r, p, qaError("qa_product_review_invalid", detail)
		}
		if err := qaRead(p.Path, &r); err != nil {
			return bad(err.Error())
		}
		if a, err := e.qaRegisteredFile(run.ID, p.Path); err != nil || a.ContentHash != p.Hash {
			return bad("report is no longer a live registered artifact")
		}
		h, err := qaHash(p.Path)
		if err != nil || h != p.Hash {
			return bad("registered product report changed")
		}
		if r.Schema != "beacon-product-qa.v1" || r.RunID != run.ID || r.CandidateHash != c.Hash || r.Scope != "product" || r.Verdict != "pass" || r.Maker != c.Maker || strings.TrimSpace(r.Checker) == "" || r.Checker != strings.TrimSpace(r.Checker) || r.Checker == c.Maker || r.Checker == m.ExecutorID || r.Checker != strings.TrimSpace(p.By) || len(r.EvidenceRefs) == 0 {
			return bad("latest report must be independent product/pass bound to this candidate and registered by checker")
		}
		fc, err := e.featureContext(run.FeatureID)
		if err != nil {
			return bad(err.Error())
		}
		covered := map[string]bool{}
		for _, tc := range r.CoveredTCs {
			covered[tc] = true
		}
		rows := gate.ParseCoverageRows(e.packageFiles(fc).Tests.Text)
		if len(rows) == 0 {
			return bad("no TC contract")
		}
		for _, row := range rows {
			if !covered[row.TC] {
				return bad("missing TC coverage: " + row.TC)
			}
		}
		for _, ref := range r.EvidenceRefs {
			path := resolvePkgPath(fc.Project.RootPath, ref)
			if _, err := e.qaRegisteredFile(run.ID, path); err != nil {
				return bad(err.Error())
			}
			bound := false
			for _, f := range p.EvidenceFiles {
				if f.Path == path {
					bound = true
					if err := qaFileMatches(fc.Project.RootPath, f); err != nil {
						return bad(err.Error())
					}
				}
			}
			if !bound {
				return bad("review evidence hash was not registered with report: " + path)
			}
		}
		return r, p, nil
	}
	return qaProductReview{}, qaRegistration{}, qaError("qa_product_review_missing", "no current independent product QA report")
}

// currentQAEligibility is the live predicate shared by formal QA and every
// advancement path. Formal records are evidence bindings, never cached permission.
func (e *Engine) currentQAEligibility(run store.Run, m *runMeta, requireVerifier bool, tcChecks ...*[]gate.CheckItem) (fp string, eligibilityErr error) {
	var checks []gate.CheckItem
	defer func() {
		checks = e.qaTCEligibilityChecks(run, checks, eligibilityErr)
		for _, dst := range tcChecks {
			*dst = checks
		}
	}()
	c, err := e.currentQACandidate(run)
	if err != nil {
		return "", err
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return "", err
	}
	root := fc.Project.RootPath
	events, err := e.store.ListEventsByRun(run.ID)
	if err != nil {
		return "", err
	}
	var invocation struct {
		CandidateHash string `json:"candidate_hash"`
		ReportPath    string `json:"report_path"`
		ReportHash    string `json:"report_hash"`
	}
	found := false
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType == EventAutoTestExecuted {
			_ = json.Unmarshal(events[i].Payload, &invocation)
			found = true
			break
		}
	}
	if !found || invocation.CandidateHash != c.Hash {
		return "", qaError("qa_execution_missing", "latest invocation is not bound to current candidate")
	}
	abs := resolvePkgPath(root, invocation.ReportPath)
	h, err := qaHash(abs)
	if err != nil || h != invocation.ReportHash {
		return "", qaError("qa_execution_stale", "latest invocation report missing or changed")
	}
	var report AutoTestReport
	if err := qaRead(abs, &report); err != nil {
		return "", err
	}
	if _, err := e.qaRegisteredFile(run.ID, abs); err != nil {
		return "", err
	}
	rows := gate.ParseCoverageRows(e.packageFiles(fc).Tests.Text)
	checks, err = e.validateQATCResults(run, root, c.Hash, invocation.ReportPath, report, rows)
	if err != nil {
		return "", err
	}
	parts := []string{c.Hash, invocation.ReportHash}
	for _, f := range report.RawFiles {
		parts = append(parts, f.SHA256)
	}
	r, p, err := e.currentQAReview(run, m, c)
	if err != nil {
		return "", err
	}
	parts = append(parts, p.Hash)
	for _, ref := range r.EvidenceRefs {
		a, err := e.qaRegisteredFile(run.ID, resolvePkgPath(root, ref))
		if err != nil {
			return "", err
		}
		parts = append(parts, a.ContentHash)
	}
	if requireVerifier {
		if m.Verifier == nil || !m.Verifier.Present || m.Verifier.Source != r.Checker || len(m.Verifier.Refs) == 0 {
			return "", qaError(ReasonVerifierRequired, "attach current independent report verifier")
		}
		for _, ref := range m.Verifier.Refs {
			path := resolvePkgPath(root, ref)
			allowed := path == p.Path
			for _, ev := range r.EvidenceRefs {
				allowed = allowed || path == resolvePkgPath(root, ev)
			}
			if !allowed {
				return "", qaError(ReasonVerifierRequired, "verifier ref is not current report or its evidence")
			}
		}
	}
	decl := gate.ParseSurfaceDeclaration(e.packageFiles(fc).Truth.Text)
	if decl.EffectiveStrict() {
		if len(decl.Surfaces) == 0 {
			return "", qaError("qa_surface_evidence_miss:surfaces_undeclared", "strict truth declares no surfaces")
		}
		for _, surface := range decl.Surfaces {
			kind := gate.SurfaceEvidenceKind(surface)
			ok := false
			arts, _ := e.store.Artifacts().ListByRun(run.ID)
			for _, a := range arts {
				if a.Kind == kind {
					if stale, _ := store.ArtifactStaleness(a); !stale {
						ok = true
						parts = append(parts, a.ContentHash)
					}
				}
			}
			if !ok {
				return "", qaError(gate.SurfaceEvidenceReason(surface), "missing live surface evidence")
			}
		}
	}
	return digestOf(parts...), nil
}
func (e *Engine) requireCurrentFormalQA(run store.Run, m *runMeta) error {
	if m.Blocked && m.BlockedStreak >= 3 {
		return qaError(ReasonBlockedStreak, "hard blocker requires explicit recovery")
	}
	fp, err := e.currentQAEligibility(run, m, true)
	if err != nil {
		return err
	}
	events, err := e.store.ListEventsByRun(run.ID)
	if err != nil {
		return err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != EventGateEvaluated {
			continue
		}
		var p struct {
			Gate        string `json:"gate_type"`
			Verdict     string `json:"verdict"`
			Fingerprint string `json:"qa_fingerprint"`
		}
		_ = json.Unmarshal(events[i].Payload, &p)
		if p.Gate != "qa" {
			continue
		}
		if p.Verdict == "pass" && p.Fingerprint == fp {
			return nil
		}
		break
	}
	return qaError(ReasonQAGateNotPassed, "current evidence requires a passing formal QA gate bound to this run")
}

func (e *Engine) qaEvaluation(evaluator, fp string, err error, tcChecks ...[]gate.CheckItem) gate.Evaluation {
	check := gate.CheckItem{Check: "current_product_qa", Applicable: true, OK: err == nil, Detail: "current candidate has executed tests, independent product review and live verifier"}
	v := gate.Evaluation{GateType: gate.GateQA, Verdict: gate.VerdictPass, Evaluator: evaluator}
	if err != nil {
		check.ReasonCode, _ = core.ReasonCodeOf(err)
		if check.ReasonCode == "" {
			check.ReasonCode = "qa_evidence_invalid"
		}
		check.Detail = err.Error()
		v.Verdict = gate.VerdictBlocked
		v.ReasonCodes = []string{check.ReasonCode}
	}
	v.Checks = []gate.CheckItem{check}
	for _, checks := range tcChecks {
		v.Checks = append(v.Checks, checks...)
	}
	return v
}
