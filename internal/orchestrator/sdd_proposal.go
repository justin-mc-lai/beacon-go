package orchestrator

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

func lakeProposalReadiness(p LakeProposal) string {
	if p.PackageContract == SDDContract {
		return "sdd_content_validated_draft"
	}
	return "legacy_shape_only"
}

var sddProposalTCRef = regexp.MustCompile(`\bTC-[A-Za-z0-9_-]+\b`)

// Proposals reuse the package's structural gate, not the signed/grounded
// review gate: the result is still a draft, never a freeze acknowledgement.
func validateSDDProposalLake(p LakeProposal, lake LakeProposalLake) error {
	if !slugRe.MatchString(lake.Slug) {
		return sddProposalError(lake.Slug, "invalid package slug")
	}
	if strings.TrimSpace(lake.Truth) == "" || strings.TrimSpace(lake.Tasks) == "" {
		return sddProposalError(lake.Slug, "truth and tasks content are required")
	}
	scalars, _, ok := gate.ParseFrontmatter(lake.Truth)
	if !ok || !strings.HasPrefix(lake.Truth, "---\n") || scalars["package_contract"] != SDDContract {
		return sddProposalError(lake.Slug, "truth must declare package_contract: truth-tasks in frontmatter")
	}
	for _, text := range []string{lake.Truth, lake.Tasks} {
		meta, refs, present := gate.ParseFrontmatter(text)
		if !present {
			continue
		}
		for key, ref := range refs {
			if (key != "truth" && key != "tasks") || filepath.IsAbs(ref) || filepath.ToSlash(filepath.Clean(ref)) != ref ||
				ref == ".." || strings.HasPrefix(ref, "../") || filepath.Base(ref) != key+".md" {
				return sddProposalError(lake.Slug, "invalid two-file canonical_refs."+key)
			}
		}
		if meta["slug"] != "" && meta["slug"] != lake.Slug {
			return sddProposalError(lake.Slug, "content slug differs from lake slug")
		}
		if meta["package_contract"] != "" && meta["package_contract"] != SDDContract {
			return sddProposalError(lake.Slug, "content contract differs from proposal contract")
		}
		if meta["status"] != "" && meta["status"] != "draft" {
			return sddProposalError(lake.Slug, "proposal content must be draft")
		}
		if meta["intent_confirmed"] != "" && meta["intent_confirmed"] != "false" {
			return sddProposalError(lake.Slug, "selection cannot confirm intent")
		}
		if meta["version"] != "" {
			norm, err := NormalizeDocsVersion(meta["version"])
			if err != nil || norm != meta["version"] {
				return sddProposalError(lake.Slug, "content version must be a canonical docs version")
			}
			if p.Version != "" && p.Version != meta["version"] {
				return sddProposalError(lake.Slug, "content version differs from proposal version")
			}
			if scalars["version"] != "" && scalars["version"] != meta["version"] {
				return sddProposalError(lake.Slug, "truth and tasks versions differ")
			}
		}
	}
	if p.Version != "" {
		norm, err := NormalizeDocsVersion(p.Version)
		if err != nil || norm != p.Version {
			return sddProposalError(lake.Slug, "proposal version must be a canonical docs version")
		}
	}
	pkg := gate.PackageFiles{
		Slug: lake.Slug, Contract: SDDContract,
		Truth: gate.PackageFile{Path: "truth.md", Text: lake.Truth, Present: true},
		Tasks: gate.PackageFile{Path: "tasks.md", Text: lake.Tasks, Present: true},
	}
	if findings := sddFreezeFindings(pkg); len(findings) > 0 {
		return sddProposalError(lake.Slug, findings[0].ReasonCode+": "+findings[0].Detail)
	}
	// DefinedACs also accepts matrix mentions for legacy packages. Draft SDD
	// proposals must instead bind to actual Acceptance Criteria definitions.
	criteria := gate.ParseACRestates(lake.Truth)
	if len(criteria) == 0 {
		return sddProposalError(lake.Slug, "define at least one Acceptance Criteria row")
	}
	defined := make(map[string]bool, len(criteria))
	for ac := range criteria {
		defined[ac] = true
	}
	dag, err := gate.ParseTaskDAG(lake.Tasks, defined)
	if err != nil {
		return sddProposalError(lake.Slug, err.Error())
	}
	for _, task := range dag.Nodes {
		for _, command := range []string{task.Command, task.Verify} {
			argv, err := SplitCommandWords(command)
			if err != nil || len(argv) == 0 || argv[0] == "" || strings.HasPrefix(argv[0], "-") ||
				sddProposalTCRef.FindString(argv[0]) == argv[0] || gate.IsDocsOnlyCommand(command) ||
				filepath.Base(argv[0]) == "true" || argv[0] == ":" || argv[0] == "exit" {
				return sddProposalError(lake.Slug, task.ID+" Command and Verify must be executable argv commands, not TC identities")
			}
		}
	}
	rows, _, _ := gate.ParseCoverageMatrix(lake.Truth)
	knownTCs := make(map[string]bool, len(rows))
	for _, row := range rows {
		if !defined[row.AC] {
			return sddProposalError(lake.Slug, row.TC+" references undefined "+row.AC)
		}
		knownTCs[row.TC] = true
	}
	for _, text := range []string{lake.Truth, lake.Tasks} {
		for _, tc := range sddProposalTCRef.FindAllString(text, -1) {
			if !knownTCs[tc] {
				return sddProposalError(lake.Slug, "unknown test case reference "+tc)
			}
		}
	}
	return nil
}

func sddProposalError(slug, detail string) error {
	return core.NewError(core.ReasonLakeProposalInvalid, "program.propose", "lake "+slug+": "+detail)
}

type sddProposalDraft struct {
	lake     LakeProposalLake
	truth    string
	tasks    string
	pkgRel   string
	absDir   string
	existing bool
}

// Preflight every selected package before writing any of them. An existing
// registered package is reusable only when both its contract and bytes agree.
func (e *Engine) selectSDDProposal(host store.FeatureContext, p LakeProposal, selected map[string]bool) error {
	if _, err := e.requireCanonicalBranch(host.Project.RootPath, "program.select"); err != nil {
		return err
	}
	drafts := make([]sddProposalDraft, 0, len(selected))
	for _, lake := range p.Lakes {
		if !selected[lake.Slug] {
			continue
		}
		draft, err := e.prepareSDDProposalDraft(host, p, lake)
		if err != nil {
			return err
		}
		drafts = append(drafts, draft)
	}
	for _, draft := range drafts {
		if draft.existing {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(draft.absDir), 0o755); err != nil {
			return err
		}
		if err := os.Mkdir(draft.absDir, 0o755); err != nil {
			return err
		}
		for _, file := range []struct{ name, text string }{{"truth.md", draft.truth}, {"tasks.md", draft.tasks}} {
			f, err := os.OpenFile(filepath.Join(draft.absDir, file.name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				return err
			}
			_, writeErr := f.WriteString(file.text)
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		truthPath := filepath.Join(draft.pkgRel, "truth.md")
		feat, err := e.store.Features().CreatePackage(store.FeaturePackage{
			ProjectVersionID: host.Version.ID, Slug: draft.lake.Slug,
			TruthPath: truthPath, TestsPath: truthPath, Status: "draft",
		})
		if err != nil {
			return err
		}
		if err := updateLifecycleMonotonic(e.store.Features(), store.Lifecycle{
			FeatureID: feat.ID, Stage: core.StageThink, Status: core.StatusNotStarted,
		}); err != nil {
			return err
		}
		if err := e.appendFeatureEvent(feat.ID, EventTruthCreated, map[string]any{
			"slug": draft.lake.Slug, "version": host.Version.Version,
			"package_dir": filepath.ToSlash(draft.pkgRel), "proposal_id": p.ProposalID,
			"package_contract": SDDContract,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) prepareSDDProposalDraft(host store.FeatureContext, p LakeProposal, lake LakeProposalLake) (sddProposalDraft, error) {
	draft := sddProposalDraft{lake: lake, truth: lake.Truth, tasks: lake.Tasks}
	if err := validateSDDProposalLake(p, lake); err != nil {
		return draft, err
	}
	draft.pkgRel = filepath.Join(host.Version.DocsRoot, "features", lake.Slug)
	var err error
	draft.absDir, err = safeSDDProposalPath(host.Project.RootPath, draft.pkgRel)
	if err != nil {
		return draft, sddProposalError(lake.Slug, err.Error())
	}
	if err := validateSDDProposalLocation(host, p, lake, draft.pkgRel); err != nil {
		return draft, err
	}
	meta, _, _ := gate.ParseFrontmatter(draft.truth)
	for _, field := range []struct{ key, value string }{
		{"slug", lake.Slug}, {"version", host.Version.Version}, {"status", "draft"}, {"intent_confirmed", "false"},
	} {
		if meta[field.key] == "" {
			draft.truth, _ = setFrontmatterScalar(draft.truth, field.key, field.value)
		}
	}
	if existing, err := e.store.Features().GetBySlug(host.Version.ID, lake.Slug); err == nil {
		ctx := store.FeatureContext{Project: host.Project, Version: host.Version, Feature: existing}
		pkg := e.packageFiles(ctx)
		if existing.TruthPath != filepath.Join(draft.pkgRel, "truth.md") ||
			pkg.Contract != SDDContract || !pkg.Truth.Present || !pkg.Tasks.Present ||
			pkg.Truth.Text != draft.truth || pkg.Tasks.Text != draft.tasks {
			return draft, sddProposalError(lake.Slug, "existing registered package conflicts with selected SDD draft")
		}
		draft.existing = true
		return draft, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return draft, err
	}
	if _, err := os.Lstat(draft.absDir); err == nil {
		return draft, sddProposalError(lake.Slug, "unregistered package directory already exists")
	} else if !os.IsNotExist(err) {
		return draft, err
	}
	return draft, nil
}

func validateSDDProposalLocation(host store.FeatureContext, p LakeProposal, lake LakeProposalLake, pkgRel string) error {
	if p.Version != "" && p.Version != host.Version.Version {
		return sddProposalError(lake.Slug, "proposal version differs from the program's registered version")
	}
	for _, text := range []string{lake.Truth, lake.Tasks} {
		scalars, refs, _ := gate.ParseFrontmatter(text)
		if scalars["version"] != "" && scalars["version"] != host.Version.Version {
			return sddProposalError(lake.Slug, "content version differs from the program's registered version")
		}
		for key, ref := range refs {
			want := filepath.ToSlash(filepath.Join(pkgRel, key+".md"))
			if (key != "truth" && key != "tasks") || ref != want {
				return sddProposalError(lake.Slug, "canonical_refs."+key+" must be "+want)
			}
		}
	}
	return nil
}

// Reject traversal and symlink escapes, including through existing ancestors of
// a new package directory. Selection never writes outside its registered root.
func safeSDDProposalPath(root, rel string) (string, error) {
	if filepath.IsAbs(rel) || filepath.Clean(rel) != rel || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe package path %q", rel)
	}
	rootAbs, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(rootAbs, rel)
	for parent := abs; ; parent = filepath.Dir(parent) {
		resolved, err := filepath.EvalSymlinks(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		within, err := filepath.Rel(rootAbs, resolved)
		if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("package path escapes project root")
		}
		return abs, nil
	}
}

// Program freeze consumes the real package gates; it never manufactures a
// per-package frozen marker, confirmation, or reviewer on the human's behalf.
func (e *Engine) checkSDDProgramFreeze(run store.Run, m *runMeta, slugs []string) error {
	host, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return err
	}
	proposalSDD := m.LakeProposal != nil && m.LakeProposal.PackageContract == SDDContract
	if !ok {
		if proposalSDD {
			return core.NewError(ReasonTruthFreezeBlocked, "program.freeze", "SDD program has no registered feature context")
		}
		return nil
	}
	for _, slug := range slugs {
		feat, err := e.store.Features().GetBySlug(host.Version.ID, slug)
		if err != nil {
			if !proposalSDD && errors.Is(err, sql.ErrNoRows) {
				continue // Legacy plans historically allow unregistered lake names.
			}
			return core.NewError(ReasonTruthFreezeBlocked, "program.freeze", "lake "+slug+" has no registered package")
		}
		ctx := store.FeatureContext{Project: host.Project, Version: host.Version, Feature: feat}
		pkg := e.packageFiles(ctx)
		if pkg.Contract != SDDContract {
			if proposalSDD {
				return core.NewError(ReasonTruthFreezeBlocked, "program.freeze", "lake "+slug+" is not an SDD package")
			}
			continue
		}
		fact, err := e.FrozenFact(ctx)
		if err != nil {
			return err
		}
		if !fact.Frozen {
			return core.NewError(ReasonTruthFreezeBlocked, "program.freeze", "lake "+slug+" must pass its own truth freeze first")
		}
		recs, err := e.trailRecords(ctx.Feature.ID)
		if err != nil {
			return err
		}
		view, pass, err := e.freezePrecheck(ctx, "program.freeze", recs.freeze == nil)
		if err != nil {
			return err
		}
		if !pass {
			return core.NewError(ReasonTruthFreezeBlocked, "program.freeze", "lake "+slug+": "+strings.Join(view.BlockReasons, ", "))
		}
	}
	return nil
}
