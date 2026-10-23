// Version-level release check, engine half (R2b, parity audit
// H7): gathers the per-feature facts of one project version and
// runs gate.EvaluateVersionRelease. This is Python release.py
// `check`'s object — the whole version — beside the per-feature
// release GATE (evalRelease), which is untouched. The verdict is
// recorded as one release_version_check event per feature (the
// trail is per-feature; there is no version table to write to and
// none is invented).
package orchestrator

import (
	"encoding/json"
	"path"
	"path/filepath"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// ReleaseVersionCheck runs the version-level check for one project
// version under one release profile. An invalid profile is refused
// with release_profile_invalid (select_release_profile's block).
func (e *Engine) ReleaseVersionCheck(projectRoot, version, profile string) (gate.VersionReleaseResult, error) {
	sel := gate.SelectReleaseProfile(profile)
	if sel.Blocked {
		return gate.VersionReleaseResult{}, core.NewError(core.ReasonReleaseProfileInvalid,
			"release.ReleaseVersionCheck", "unknown release profile "+sel.Profile+" (full|hotfix|docs_only)")
	}
	root := store.ResolveRootPath(projectRoot)
	contexts, err := e.store.ListFeatureContexts(root)
	if err != nil {
		return gate.VersionReleaseResult{}, err
	}
	var feats []store.FeatureContext
	for _, c := range contexts {
		if version == "" || c.Version.Version == version {
			feats = append(feats, c)
		}
	}
	facts := gate.VersionReleaseFacts{Version: version, VersionRegistered: len(feats) > 0}
	if version == "" && len(feats) > 0 {
		facts.Version = feats[0].Version.Version
	}
	// Metadata contract analogue: the store IS the project
	// metadata — registration consistency means every feature of
	// the version resolves to the same project + version rows.
	facts.MetadataOK = len(feats) > 0
	facts.MetadataDetail = "registration rows consistent"
	if len(feats) > 0 {
		pid, vid := feats[0].Project.ID, feats[0].Version.ID
		for _, c := range feats[1:] {
			if c.Project.ID != pid || c.Version.ID != vid {
				facts.MetadataOK = false
				facts.MetadataDetail = "feature contexts disagree on project/version rows"
			}
		}
	} else {
		facts.MetadataDetail = "no feature contexts resolve for this version"
	}
	for _, ctx := range feats {
		ff, err := e.versionFeatureFacts(ctx)
		if err != nil {
			return gate.VersionReleaseResult{}, err
		}
		facts.Features = append(facts.Features, ff)
	}
	// Version surface governance analogue: the project's topology
	// gate fact (S3) — not applicable for non-git projects.
	if len(feats) > 0 {
		first := feats[0]
		pkgRel := path.Dir(filepath.ToSlash(first.Feature.TruthPath))
		tf := e.topologyFactForProject(first.Project.ID, first.Project.RootPath, pkgRel)
		switch tf.Status {
		case TopologyNotApplicable:
			facts.TopologyApplicable = false
			facts.TopologyDetail = tf.Detail
		default:
			facts.TopologyApplicable = true
			facts.TopologyPass = tf.Status == "pass"
			facts.TopologyDetail = tf.Detail
		}
	}
	res := gate.EvaluateVersionRelease(facts, sel)
	// Trail: one event per feature carrying the version verdict.
	for _, ctx := range feats {
		_, _ = e.store.AppendEvent(store.LifecycleEvent{
			FeatureID: ctx.Feature.ID,
			EventType: "release_version_check",
			Payload: mustJSON(map[string]any{
				"version": res.Version, "profile": res.Profile, "status": res.Status,
				"failed_checks": res.FailedChecks, "warning_checks": res.WarningChecks,
			}),
		})
	}
	return res, nil
}

// versionFeatureFacts gathers one feature's version-check facts.
func (e *Engine) versionFeatureFacts(ctx store.FeatureContext) (gate.VersionFeatureFacts, error) {
	ff := gate.VersionFeatureFacts{Slug: ctx.Feature.Slug, ACCoverage: -1}
	frozen, err := e.FrozenFact(ctx)
	if err != nil {
		return ff, err
	}
	ff.Frozen = frozen.Frozen
	ff.PendingChange = frozen.ChangePending
	ff.MaterialsStale = len(e.staleMaterialSurfaces(ctx)) > 0

	rep, pkg, _, err := e.integrityReportLive(ctx, "")
	if err != nil {
		return ff, err
	}
	ff.IntegrityVerdict = string(rep.Verdict)
	ff.IntegrityBlocked = rep.Verdict == gate.VerdictBlocked
	// Coverage: ACs with a TC over total ACs, from the package
	// itself (the integrity report's ac_without_tc findings name
	// the uncovered ones).
	if pkg.Truth.Present {
		acs := gate.ExtractACDescriptions(pkg.Truth.Text)
		if len(acs) > 0 {
			uncovered := 0
			for _, f := range rep.Findings {
				if f.ReasonCode == core.ReasonIntegrityACWithoutTC {
					uncovered++
				}
			}
			ff.ACCoverage = float64(len(acs)-uncovered) / float64(len(acs))
		}
	}

	// Four-gate composition: the latest verdict of each stage gate
	// must be pass (design only where a run requires it).
	latest, err := e.store.Gates().LatestByType(ctx.Feature.ID)
	if err != nil {
		return ff, err
	}
	byType := map[string]store.GateResult{}
	for _, g := range latest {
		byType[g.GateType] = g
	}
	_, m, has, err := e.latestGoalRun(ctx, "")
	if err != nil {
		return ff, err
	}
	need := []string{string(gate.GateTruthFreeze), string(gate.GateImplement), string(gate.GateQA)}
	if has && designRequired(m) {
		need = append(need, string(gate.GateDesign))
	}
	ff.GatesPass = true
	var missing []string
	for _, t := range need {
		g, ok := byType[t]
		if !ok || g.Verdict != string(gate.VerdictPass) {
			ff.GatesPass = false
			missing = append(missing, t)
		}
	}
	if !ff.GatesPass {
		ff.GatesDetail = "gates not passing: " + joinComma(missing)
	}
	// Human confirmation: the newest run's flag or the
	// release_confirmed event (evalRelease's caliber).
	if has {
		ff.HumanConfirmed = m.ReleaseHumanConfirmed
	}
	if !ff.HumanConfirmed {
		events, err := e.store.ListEvents(ctx.Feature.ID)
		if err != nil {
			return ff, err
		}
		for _, ev := range events {
			if ev.EventType == "release_confirmed" {
				ff.HumanConfirmed = true
			}
		}
	}
	// QA evidence across the feature's runs.
	runs, err := e.store.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return ff, err
	}
	for _, run := range runs {
		arts, err := e.store.Artifacts().ListByRun(run.ID)
		if err != nil {
			return ff, err
		}
		for _, a := range arts {
			if !qaKind(a.Kind) {
				continue
			}
			ff.QAArtifactCount++
			if stale, _ := store.ArtifactStaleness(a); stale {
				ff.QAStaleCount++
			}
		}
	}
	return ff, nil
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}
