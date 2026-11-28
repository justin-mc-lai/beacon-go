package intenttree

import "testing"

func TestDeprecatedLeafNotVerifiedOrActive(t *testing.T) {
	dir := t.TempDir()
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusFrozen},
		{ID: "m", ParentID: "r", Kind: KindModule, Status: StatusFrozen},
		{ID: "f", ParentID: "m", Kind: KindFeature, Path: "f", Status: StatusFrozen},
		{ID: "dead", ParentID: "f", Kind: KindTouchpoint, Status: StatusDeprecated},
	}}
	if err := WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	rep, err := RollupVerified(nil, dir, "dead")
	if err != nil || rep.Verified || rep.Status != string(StatusDeprecated) {
		t.Fatalf("tombstone certified: %+v %v", rep, err)
	}
	if _, err := RollupVerified(nil, dir, "f"); err == nil {
		t.Fatal("zero active children certified parent")
	}
	if err := ValidateFeatureDelivery(nil, dir, "f"); err != nil {
		t.Fatal(err)
	}
}
