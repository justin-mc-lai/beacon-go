// Version manifest integration (phases 0-2).
//
// A version is a manifest under docs/beacon/<v>/manifest.json. Packages
// are snapshotted under docs/beacon/packages/<slug>/<hash>/. Opening a
// version inherits unchanged lakes by reference; freezing/refreezing a
// package updates only that lake's hash. Seas are version-scoped lake
// graphs derived from the program host runs.
package orchestrator

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
	buildversion "beacon-go/internal/version"
	"beacon-go/internal/versionmanifest"
)

// BaselineVersion snapshots the current packages of one docs line and
// writes its manifest. It does not move or rewrite any existing file.
func (e *Engine) BaselineVersion(root, version string) (versionmanifest.Manifest, error) {
	root = store.ResolveRootPath(root)
	ctxs, err := e.store.ListFeatureContexts(root)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	m := versionmanifest.Manifest{
		Schema:    versionmanifest.Schema,
		Version:   version,
		Runtime:   buildversion.Version,
		Status:    versionmanifest.StatusBaseline,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, ctx := range ctxs {
		if ctx.Version.Version != version {
			continue
		}
		pkg := e.packageFiles(ctx)
		files := map[string][]byte{
			"truth.md":       []byte(pkg.Truth.Text),
			"tests.md":       []byte(pkg.Tests.Text),
			"tasks.md":       []byte(pkg.Tasks.Text),
			"restatement.md": []byte(pkg.Restatement.Text),
		}
		if pkg.Evidence.Present {
			files["evidence.md"] = []byte(pkg.Evidence.Text)
		}
		hash := versionmanifest.HashContract(files["truth.md"], files["tests.md"], files["tasks.md"], files["restatement.md"])
		if _, err := versionmanifest.WritePackage(root, ctx.Feature.Slug, hash, version, files, nil); err != nil {
			return versionmanifest.Manifest{}, err
		}
		m.Lakes = append(m.Lakes, versionmanifest.Lake{
			Slug: ctx.Feature.Slug, Package: hash, Status: versionmanifest.LakeBaseline,
		})
	}
	if len(m.Lakes) == 0 {
		return versionmanifest.Manifest{}, core.NewError(ReasonFeatureNotFound, "version.Baseline",
			"no feature packages for "+version+" under "+root)
	}
	sort.Slice(m.Lakes, func(i, j int) bool { return m.Lakes[i].Slug < m.Lakes[j].Slug })
	m.Seas = e.buildSeas(root, m)
	if err := versionmanifest.Save(versionmanifest.Path(root, version), m); err != nil {
		return versionmanifest.Manifest{}, err
	}
	return m, nil
}

// OpenVersion opens the next docs line by inheriting the parent's lakes
// and seas by reference. It registers the version in the store and
// makes it current; no package files are copied.
func (e *Engine) OpenVersion(root, from, level string) (versionmanifest.Manifest, error) {
	return e.openVersion(root, from, level, false)
}

func (e *Engine) BumpProjectVersion(root, from, level string) (versionmanifest.Manifest, error) {
	return e.openVersion(root, from, level, true)
}

func (e *Engine) openVersion(root, from, level string, allowInitial bool) (versionmanifest.Manifest, error) {
	root = store.ResolveRootPath(root)
	project, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	versions, err := e.store.Projects().ListVersions(project.ID)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	registered := allowInitial && len(versions) == 0 && from == "v0.0.0"
	for _, version := range versions {
		if version.Version == from {
			registered = true
		}
	}
	if !registered {
		return versionmanifest.Manifest{}, core.NewError("version_not_registered", "version.Open", "parent version "+from+" is not registered under "+root)
	}
	parent, err := versionmanifest.LoadVersion(root, from)
	if err != nil {
		_, statErr := os.Lstat(versionmanifest.Path(root, from))
		if !allowInitial || !errors.Is(err, os.ErrNotExist) || !errors.Is(statErr, os.ErrNotExist) {
			return versionmanifest.Manifest{}, core.NewError("manifest_missing", "version.Open",
				"cannot load manifest for "+from+" under "+root+": "+err.Error())
		}
	} else {
		if err := versionmanifest.Validate(parent); err != nil {
			return versionmanifest.Manifest{}, err
		}
		if parent.Version != from {
			return versionmanifest.Manifest{}, core.NewError("manifest_version_mismatch", "version.Open", "parent manifest version does not match "+from)
		}
	}
	next, err := BumpDocsVersion(from, level)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	if _, err := os.Lstat(versionmanifest.Path(root, next)); err == nil {
		return versionmanifest.Manifest{}, core.NewError("version_exists", "version.Open",
			"manifest for "+next+" already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return versionmanifest.Manifest{}, err
	}
	child := versionmanifest.Inherit(parent, next)
	// Realign Runtime with the current binary: a parent line may carry a
	// stale value, and the docs version axis evolves independently.
	child.Runtime = buildversion.Version
	if err := versionmanifest.Save(versionmanifest.Path(root, next), child); err != nil {
		return versionmanifest.Manifest{}, err
	}
	if err := e.ensureStoreVersion(root, next); err != nil {
		return versionmanifest.Manifest{}, err
	}
	return child, nil
}

func (e *Engine) ensureStoreVersion(root, version string) error {
	p, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		return err
	}
	versions, err := e.store.Projects().ListVersions(p.ID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if v.Version == version {
			return e.store.Projects().SetCurrent(v.ID)
		}
	}
	_, err = e.store.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID, Version: version,
		DocsRoot:  filepath.ToSlash(filepath.Join("docs", "beacon", version)),
		IsCurrent: true,
	})
	return err
}

// FreezeVersion locks a draft manifest. A changed/new lake must carry a
// passing qa gate; inherited lakes are covered by their parent release.
func (e *Engine) FreezeVersion(root, version string) (versionmanifest.Manifest, error) {
	root = store.ResolveRootPath(root)
	m, err := versionmanifest.LoadVersion(root, version)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	if m.Status != versionmanifest.StatusDraft {
		return m, core.NewError("version_not_draft", "version.Freeze",
			"manifest "+version+" is "+m.Status+", not draft")
	}
	for i := range m.Lakes {
		l := &m.Lakes[i]
		if l.Status != versionmanifest.LakeChanged && l.Status != versionmanifest.LakeNew && l.Status != versionmanifest.LakeDirty {
			continue
		}
		feat, err := e.store.Features().FindBySlug(l.Slug)
		if err != nil {
			return m, core.NewError("version_lake_missing", "version.Freeze", "lake "+l.Slug+" has no feature package")
		}
		if !e.gatePass(feat.ID, "qa") {
			return m, core.NewError("version_lake_qa_missing", "version.Freeze",
				"lake "+l.Slug+" is "+l.Status+" without a passing qa gate")
		}
		if !e.featureHasVerifier(feat.ID) {
			return m, core.NewError("version_lake_verifier_missing", "version.Freeze",
				"lake "+l.Slug+" is "+l.Status+" without an independent verifier")
		}
		if l.Gates == nil {
			l.Gates = map[string]string{}
		}
		l.Gates["qa"] = "pass"
	}
	for _, c := range m.Changes {
		if c.Status != "closed" {
			return m, core.NewError("version_change_open", "version.Freeze",
				"change "+c.ChangeID+" is still open")
		}
	}
	m.Status = versionmanifest.StatusFrozen
	m.FrozenAt = time.Now().UTC().Format(time.RFC3339)
	if err := versionmanifest.Save(versionmanifest.Path(root, version), m); err != nil {
		return versionmanifest.Manifest{}, err
	}
	return m, nil
}

// ReleaseVersion binds a frozen manifest to its artifacts.
func (e *Engine) ReleaseVersion(root, version string) (versionmanifest.Manifest, error) {
	root = store.ResolveRootPath(root)
	m, err := versionmanifest.LoadVersion(root, version)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	if m.Status != versionmanifest.StatusFrozen {
		return m, core.NewError("version_not_frozen", "version.Release",
			"manifest "+version+" is "+m.Status+", not frozen")
	}
	for _, l := range m.Lakes {
		if l.Status != versionmanifest.LakeChanged && l.Status != versionmanifest.LakeNew && l.Status != versionmanifest.LakeDirty {
			continue
		}
		feat, err := e.store.Features().FindBySlug(l.Slug)
		if err != nil || !e.gatePass(feat.ID, "release") {
			return m, core.NewError("version_lake_release_missing", "version.Release",
				"lake "+l.Slug+" has no passing release gate")
		}
	}
	m.Runtime = buildversion.Version
	m.Status = versionmanifest.StatusReleased
	m.ReleasedAt = time.Now().UTC().Format(time.RFC3339)
	if err := versionmanifest.Save(versionmanifest.Path(root, version), m); err != nil {
		return versionmanifest.Manifest{}, err
	}
	return m, nil
}

// targetManifestPath prefers the project's current draft line. That is
// how a change to an inherited v2.0.0 lake lands in the open v2.1.0
// manifest instead of rewriting the baseline.
func (e *Engine) targetManifestPath(ctx store.FeatureContext) string {
	fallback := versionmanifest.Path(ctx.Project.RootPath, ctx.Version.Version)
	p, err := e.store.Projects().GetByRootPath(ctx.Project.RootPath)
	if err != nil {
		return fallback
	}
	cur, err := e.store.Projects().CurrentVersion(p.ID)
	if err != nil {
		return fallback
	}
	cand := versionmanifest.Path(ctx.Project.RootPath, cur.Version)
	if m, err := versionmanifest.Load(cand); err == nil && m.Status == versionmanifest.StatusDraft {
		return cand
	}
	return fallback
}

// RecordVersionPackage updates the current draft manifest after a
// freeze/refreeze. A missing manifest is legacy behavior: nothing to do.
func (e *Engine) RecordVersionPackage(ctx store.FeatureContext, hash string, changeIDs []string) error {
	if hash == "" {
		return nil
	}
	path := e.targetManifestPath(ctx)
	m, err := versionmanifest.Load(path)
	if err != nil {
		return nil
	}
	if m.Status != versionmanifest.StatusDraft {
		return nil
	}
	idx := -1
	for i := range m.Lakes {
		if m.Lakes[i].Slug == ctx.Feature.Slug {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.Lakes = append(m.Lakes, versionmanifest.Lake{Slug: ctx.Feature.Slug, Status: versionmanifest.LakeNew})
		idx = len(m.Lakes) - 1
	}
	m.Lakes[idx].Package = hash
	m.Lakes[idx].Status = versionmanifest.LakeChanged
	for _, id := range changeIDs {
		m.Lakes[idx].Changes = appendUnique(m.Lakes[idx].Changes, id)
		for i := range m.Changes {
			if m.Changes[i].ChangeID == id {
				m.Changes[i].Status = "closed"
			}
		}
	}
	return versionmanifest.Save(path, m)
}

// MarkVersionChange marks a lake dirty and records the change at the
// version level. It is a no-op for legacy trees with no draft manifest.
func (e *Engine) MarkVersionChange(ctx store.FeatureContext, changeID string) error {
	if changeID == "" {
		return nil
	}
	path := e.targetManifestPath(ctx)
	m, err := versionmanifest.Load(path)
	if err != nil {
		return nil
	}
	if m.Status != versionmanifest.StatusDraft {
		return nil
	}
	found := false
	for i := range m.Lakes {
		if m.Lakes[i].Slug == ctx.Feature.Slug {
			m.Lakes[i].Status = versionmanifest.LakeDirty
			m.Lakes[i].Changes = appendUnique(m.Lakes[i].Changes, changeID)
			found = true
		}
	}
	if !found {
		m.Lakes = append(m.Lakes, versionmanifest.Lake{
			Slug: ctx.Feature.Slug, Status: versionmanifest.LakeDirty,
			Changes: []string{changeID},
		})
	}
	dup := false
	for _, c := range m.Changes {
		if c.ChangeID == changeID {
			dup = true
			break
		}
	}
	if !dup {
		m.Changes = append(m.Changes, versionmanifest.Change{
			ChangeID: changeID, Lakes: []string{ctx.Feature.Slug}, Source: "truth", Status: "open",
		})
	}
	return versionmanifest.Save(path, m)
}

func appendUnique(list []string, want string) []string {
	for _, v := range list {
		if v == want {
			return list
		}
	}
	return append(list, want)
}

// buildSeas derives the version-scoped sea list from program host runs.
func (e *Engine) buildSeas(root string, m versionmanifest.Manifest) []versionmanifest.Sea {
	lakes := map[string]string{}
	for _, l := range m.Lakes {
		lakes[l.Slug] = l.Package
	}
	runs, err := e.store.Runs().ListAll()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []versionmanifest.Sea
	for _, run := range runs {
		if len(run.Metadata) == 0 {
			continue
		}
		var meta struct {
			Program *struct {
				Slug   string `json:"slug"`
				Frozen bool   `json:"frozen"`
				Lakes  []struct {
					Slug      string   `json:"slug"`
					DependsOn []string `json:"depends_on"`
				} `json:"lakes"`
			} `json:"program"`
			LakeProposal *struct {
				ProposalID string `json:"proposal_id"`
			} `json:"lake_proposal"`
		}
		if err := json.Unmarshal(run.Metadata, &meta); err != nil || meta.Program == nil || meta.Program.Slug == "" {
			continue
		}
		feat, err := e.store.Features().GetByID(run.FeatureID)
		if err != nil {
			continue
		}
		// The run may be bound to an older feature row or another
		// project registration; the sea belongs to this version when its
		// host slug is part of the manifest's lake set.
		if _, ok := lakes[feat.Slug]; !ok {
			continue
		}
		if seen[meta.Program.Slug] {
			continue
		}
		seen[meta.Program.Slug] = true
		sea := versionmanifest.Sea{Slug: meta.Program.Slug, HostFeature: feat.Slug, Frozen: meta.Program.Frozen}
		if meta.LakeProposal != nil {
			sea.Proposal = meta.LakeProposal.ProposalID
		}
		for _, l := range meta.Program.Lakes {
			sea.Lakes = append(sea.Lakes, versionmanifest.SeaLake{
				Slug: l.Slug, Package: lakes[l.Slug], DependsOn: l.DependsOn,
			})
		}
		out = append(out, sea)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ---- phase 3: seas in the manifest ----

// effectiveProgramPlan prefers the current draft manifest's sea graph.
// Run bindings (goal run ids, admissions) still come from the snapshot.
func (e *Engine) effectiveProgramPlan(run store.Run, m *runMeta) (*ProgramState, string) {
	if m.Program == nil {
		return m.Program, "run"
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return m.Program, "run"
	}
	man, err := versionmanifest.Load(e.targetManifestPath(fc))
	if err != nil {
		return m.Program, "run"
	}
	var sea *versionmanifest.Sea
	for i := range man.Seas {
		if man.Seas[i].Slug == m.Program.Slug {
			sea = &man.Seas[i]
			break
		}
	}
	if sea == nil {
		return m.Program, "run"
	}
	cp := *m.Program
	cp.Frozen = sea.Frozen
	cp.Lakes = nil
	for _, sl := range sea.Lakes {
		l := ProgramLake{Slug: sl.Slug, DependsOn: sl.DependsOn}
		if old := m.Program.lake(sl.Slug); old != nil {
			l = *old
			l.DependsOn = sl.DependsOn
		}
		cp.Lakes = append(cp.Lakes, l)
	}
	return &cp, "manifest"
}

// syncProgramSea writes the plan's lake graph into the current draft
// manifest. Legacy trees without a manifest are a no-op.
func (e *Engine) syncProgramSea(run store.Run, m *runMeta) {
	if m.Program == nil {
		return
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return
	}
	path := e.targetManifestPath(fc)
	man, err := versionmanifest.Load(path)
	if err != nil || man.Status != versionmanifest.StatusDraft {
		return
	}
	lakePkg := map[string]string{}
	for _, l := range man.Lakes {
		lakePkg[l.Slug] = l.Package
	}
	sea := versionmanifest.Sea{Slug: m.Program.Slug, HostFeature: fc.Feature.Slug, Frozen: m.Program.Frozen}
	if m.LakeProposal != nil {
		sea.Proposal = m.LakeProposal.ProposalID
	}
	for _, l := range m.Program.Lakes {
		sea.Lakes = append(sea.Lakes, versionmanifest.SeaLake{
			Slug: l.Slug, Package: lakePkg[l.Slug], DependsOn: l.DependsOn,
		})
	}
	found := false
	for i := range man.Seas {
		if man.Seas[i].Slug == sea.Slug {
			man.Seas[i] = sea
			found = true
			break
		}
	}
	if !found {
		man.Seas = append(man.Seas, sea)
	}
	_ = versionmanifest.Save(path, man)
}

// ---- phase 4: formal gates + compatibility ----

func (e *Engine) gatePass(featureID, gateType string) bool {
	g, err := e.store.Gates().Latest(featureID, gateType)
	return err == nil && g.Verdict == "pass"
}

func (e *Engine) featureHasVerifier(featureID string) bool {
	runs, err := e.store.Runs().ListByFeature(featureID)
	if err != nil || len(runs) == 0 {
		return false
	}
	var meta struct {
		Verifier *struct {
			Present bool `json:"present"`
		} `json:"verifier"`
	}
	if json.Unmarshal(runs[len(runs)-1].Metadata, &meta) != nil {
		return false
	}
	return meta.Verifier != nil && meta.Verifier.Present
}

// CompatVersion materializes inherited lakes as ordinary files under
// docs/beacon/<v>/features/<slug>/ so older readers keep working. It
// is opt-in: the manifest remains the source of truth.
func (e *Engine) CompatVersion(root, version string, write bool) ([]string, error) {
	root = store.ResolveRootPath(root)
	man, err := versionmanifest.LoadVersion(root, version)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range man.Lakes {
		if l.Package == "" {
			continue
		}
		dst := filepath.Join(root, "docs", "beacon", version, "features", l.Slug)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		src := versionmanifest.PackageDir(root, l.Slug, l.Package)
		for _, name := range []string{"truth.md", "tests.md", "tasks.md", "restatement.md", "evidence.md"} {
			body, err := os.ReadFile(filepath.Join(src, name))
			if err != nil {
				continue
			}
			if write {
				if err := os.MkdirAll(dst, 0o755); err != nil {
					return out, err
				}
				if err := os.WriteFile(filepath.Join(dst, name), body, 0o644); err != nil {
					return out, err
				}
			}
		}
		out = append(out, dst)
	}
	return out, nil
}

// ---- phase 5: version plan ----

type VersionPlan struct {
	From           string   `json:"from"`
	SuggestedLevel string   `json:"suggested_level"`
	NextVersion    string   `json:"next_version"`
	Draft          bool     `json:"draft"`
	LakesChanged   int      `json:"lakes_changed"`
	LakesNew       int      `json:"lakes_new"`
	OpenChanges    int      `json:"open_changes"`
	Reasons        []string `json:"reasons"`
}

func (e *Engine) PlanVersion(root, from string) (VersionPlan, error) {
	root = store.ResolveRootPath(root)
	if from == "" {
		p, err := e.store.Projects().GetByRootPath(root)
		if err != nil {
			return VersionPlan{}, err
		}
		cur, err := e.store.Projects().CurrentVersion(p.ID)
		if err != nil {
			return VersionPlan{}, err
		}
		from = cur.Version
	}
	man, err := versionmanifest.LoadVersion(root, from)
	if err != nil {
		return VersionPlan{}, err
	}
	out := VersionPlan{From: man.Version, Draft: man.Status == versionmanifest.StatusDraft}
	for _, l := range man.Lakes {
		switch l.Status {
		case versionmanifest.LakeChanged, versionmanifest.LakeDirty:
			out.LakesChanged++
		case versionmanifest.LakeNew:
			out.LakesNew++
		}
	}
	for _, c := range man.Changes {
		if c.Status != "closed" {
			out.OpenChanges++
		}
	}
	level := "patch"
	if out.LakesNew > 0 {
		level = "minor"
	}
	out.SuggestedLevel = level
	if next, err := BumpDocsVersion(man.Version, level); err == nil {
		out.NextVersion = next
	}
	switch {
	case out.LakesNew > 0:
		out.Reasons = append(out.Reasons, "new lakes present: minor")
	case out.LakesChanged > 0 || out.OpenChanges > 0:
		out.Reasons = append(out.Reasons, "changed lakes or open changes: patch")
	default:
		out.Reasons = append(out.Reasons, "no open change; a new version would be a no-op until a change lands")
	}
	if out.Draft {
		out.Reasons = append(out.Reasons, "current manifest is already draft; finish it before opening another line")
	}
	return out, nil
}

// ReconcileVersion closes manifest changes whose store package no longer
// reports them pending. It is the repair path after a refreeze written
// by a build that did not carry the change id into the manifest.
func (e *Engine) ReconcileVersion(root, version string) (versionmanifest.Manifest, error) {
	root = store.ResolveRootPath(root)
	m, err := versionmanifest.LoadVersion(root, version)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	if m.Status != versionmanifest.StatusDraft {
		return m, core.NewError("version_not_draft", "version.Reconcile",
			"manifest "+version+" is "+m.Status+", not draft")
	}
	for i := range m.Changes {
		c := &m.Changes[i]
		if c.Status == "closed" || len(c.Lakes) == 0 {
			continue
		}
		feat, err := e.store.Features().FindBySlug(c.Lakes[0])
		if err != nil {
			continue
		}
		pending, err := e.pendingChangeIDs(feat.ID)
		if err != nil {
			continue
		}
		found := false
		for _, id := range pending {
			if id == c.ChangeID {
				found = true
			}
		}
		if !found {
			c.Status = "closed"
		}
	}
	if err := versionmanifest.Save(versionmanifest.Path(root, version), m); err != nil {
		return versionmanifest.Manifest{}, err
	}
	return m, nil
}

// SyncSeas rebuilds the draft manifest's sea list from all program host
// runs whose host slug is part of this version. It repairs seas that
// were created while their feature row was bound to an older line.
func (e *Engine) SyncSeas(root, version string) (versionmanifest.Manifest, error) {
	root = store.ResolveRootPath(root)
	m, err := versionmanifest.LoadVersion(root, version)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	if m.Status != versionmanifest.StatusDraft {
		return m, core.NewError("version_not_draft", "version.SyncSeas",
			"manifest "+version+" is "+m.Status+", not draft")
	}
	m.Seas = e.buildSeas(root, m)
	if err := versionmanifest.Save(versionmanifest.Path(root, version), m); err != nil {
		return versionmanifest.Manifest{}, err
	}
	return m, nil
}
