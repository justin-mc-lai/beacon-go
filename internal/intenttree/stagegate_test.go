package intenttree

import "testing"

func gateIndex(t *testing.T, statuses map[string]Status) string {
	t.Helper()
	dir := t.TempDir()
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: []IndexEntry{
		{ID: "root", Kind: KindIntent, Status: statuses["root"], Path: "root"},
		{ID: "m", ParentID: "root", Kind: KindModule, Status: statuses["m"], Path: "m"},
		{ID: "f", ParentID: "m", Kind: KindFeature, Status: statuses["f"], Path: "m/auth", Title: "auth"},
		{ID: "t", ParentID: "f", Kind: KindTouchpoint, Status: statuses["t"], Path: "m/auth/t"},
	}}
	if err := WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStageGateFeatureLifecycle(t *testing.T) {
	d := gateIndex(t, map[string]Status{"root": StatusDraft, "m": StatusDraft, "f": StatusDraft, "t": StatusDraft})
	if ok, _, _ := StageGate(d, "plan", "auth"); !ok {
		t.Fatal("plan must allow a valid draft tree")
	}
	if ok, reason, _ := StageGate(d, "truth", "auth"); ok || reason != "tree_feature_not_frozen" {
		t.Fatalf("draft feature truth gate = %v %q", ok, reason)
	}
	d = gateIndex(t, map[string]Status{"root": StatusDraft, "m": StatusDraft, "f": StatusFrozen, "t": StatusDraft})
	if ok, reason, _ := StageGate(d, "implement", "auth"); ok || reason != "tree_leaf_not_frozen" {
		t.Fatalf("draft leaf implement gate = %v %q", ok, reason)
	}
	d = gateIndex(t, map[string]Status{"root": StatusDraft, "m": StatusDraft, "f": StatusFrozen, "t": StatusFrozen})
	if ok, _, _ := StageGate(d, "implement", "auth"); !ok {
		t.Fatal("frozen leaves must allow implement")
	}
	if ok, reason, _ := StageGate(d, "qa", "auth"); ok || reason != "tree_leaf_not_verified" {
		t.Fatalf("unverified leaf qa gate = %v %q", ok, reason)
	}
	d = gateIndex(t, map[string]Status{"root": StatusVerified, "m": StatusVerified, "f": StatusVerified, "t": StatusVerified})
	if ok, _, _ := StageGate(d, "qa", "auth"); !ok {
		t.Fatal("verified leaves must allow qa")
	}
	if ok, _, _ := StageGate(d, "release", "auth"); !ok {
		t.Fatal("verified root must allow release")
	}
}

func TestStageGateNoTreeIsLegacyAllowed(t *testing.T) {
	if ok, _, _ := StageGate(t.TempDir(), "implement", "auth"); !ok {
		t.Fatal("missing tree must not block legacy projects")
	}
}
