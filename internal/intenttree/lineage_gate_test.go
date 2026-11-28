package intenttree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageGateCorruptLineageAndLegacy(t *testing.T) {
	root := t.TempDir()
	target := TreeDir(root, "docs/beacon/v0.2.0")
	if ok, _, err := StageGate(target, "qa", "feature"); err != nil || !ok {
		t.Fatalf("genuine legacy refused: %v %v", ok, err)
	}
	path := filepath.Join(root, "docs", "beacon", "v0.1.0", "tree", "lineage", "v0.2.0.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{`, `{"schema":"beacon-tree-delivery-lineage.v1","to_version":"v0.2.0","project_id":"p","target_version_id":"v","from_version":"v0.1.0","baseline_snapshot_id":"missing"}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		for _, stage := range []string{"plan", "truth", "implement", "qa", "release"} {
			if ok, _, err := StageGate(target, stage, "feature"); err == nil || ok {
				t.Fatalf("%s swallowed corrupt lineage %q: %v %v", stage, raw, ok, err)
			}
		}
	}
}
