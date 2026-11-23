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
	"os"
	"path/filepath"
	"sort"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
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
	root = store.ResolveRootPath(root)
	parent, err := versionmanifest.LoadVersion(root, from)
	if err != nil {
		return versionmanifest.Manifest{}, core.NewError("manifest_missing", "version.Open",
			"no manifest for "+from+" under "+root+"; run `beacon version baseline --version "+from+"` first")
	}
	next, err := BumpDocsVersion(from, level)
	if err != nil {
		return versionmanifest.Manifest{}, err
	}
	if _, err := os.Stat(versionmanifest.Path(root, next)); err == nil {
		return versionmanifest.Manifest{}, core.NewError("version_exists", "version.Open",
			"manifest for "+next+" already exists")
	}
	child := versionmanifest.Inherit(parent, next)
	if err := e.ensureStoreVersion(root, next); err != nil {
		return versionmanifest.Manifest{}, err
	}
	if err := versionmanifest.Save(versionmanifest.Path(root, next), child); err != nil {
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
	for _, l := range m.Lakes {
		if l.Status == versionmanifest.LakeChanged || l.Status == versionmanifest.LakeNew || l.Status == versionmanifest.LakeDirty {
			if l.Gates == nil || l.Gates["qa"] != "pass" {
				return m, core.NewError("version_lake_qa_missing", "version.Freeze",
					"lake "+l.Slug+" is "+l.Status+" without a passing qa gate")
			}
		}
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
	proj, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		return nil
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
		pv, err := e.store.Projects().GetVersionByID(feat.ProjectVersionID)
		if err != nil || pv.ProjectID != proj.ID || pv.Version != m.Version {
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
