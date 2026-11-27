package intenttree

import (
	"beacon-go/internal/core"
	"strings"
	"testing"
)

func TestRenderHTMLTreeShowsGateVerdictPanel(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: "aa"},
		{ID: "f", ParentID: "r", Kind: KindFeature, Status: StatusImplemented, Title: "feature", Path: "root/feature", Fingerprint: "bb"},
	}}
	bodies := map[string]Node{
		"r": {ID: "r", Kind: KindIntent, Title: "root"},
		"f": {ID: "f", Kind: KindFeature, Title: "feature", Status: StatusImplemented,
			Implementation:       &ImplementationRecord{By: "dev"},
			VerificationEvidence: &EvidenceRef{ArtifactID: "art_verify"}},
	}
	rc := RenderContext{
		GateVerdicts: map[string][]GateVerdictStatus{
			"f": {{GateType: "qa", Verdict: "blocked", ReasonCodes: []string{"evidence_missing"}, EvaluatedAt: "2026-11-27T00:00:00Z"}},
		},
	}
	out := string(RenderHTMLTreeWithContext(idx, bodies, nil, rc))
	for _, want := range []string{"gate-panel", "Gate 判定", "qa", "blocked", "evidence_missing", "art_verify"} {
		if !strings.Contains(out, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLTreeShowsEmptyGatePanel(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: "aa"},
	}}
	out := string(RenderHTMLTreeWithContext(idx, map[string]Node{"r": {ID: "r", Kind: KindIntent, Title: "root"}}, nil, RenderContext{}))
	for _, want := range []string{"gate-panel", "Gate 判定", "reason code", core.ReasonGateNoResult} {
		if !strings.Contains(out, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}
