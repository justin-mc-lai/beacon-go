// truth-version-iteration: a docs version is a delivery line. Greenfield
// opens v0.0.1, same iteration shares current, explicit lines must exist,
// runtime-shaped lines are not docs versions.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func TestDocsVersionNormalizeAndUnregistered(t *testing.T) {
	if got, err := NormalizeDocsVersion("1.2.3"); err != nil || got != "v1.2.3" {
		t.Fatalf("normalize 1.2.3 = %q %v", got, err)
	}
	if got, err := NormalizeDocsVersion("v1.2.3"); err != nil || got != "v1.2.3" {
		t.Fatalf("normalize v1.2.3 = %q %v", got, err)
	}
	if _, err := NormalizeDocsVersion("banana"); err == nil {
		t.Fatal("invalid docs version must be refused")
	}
	if _, err := NormalizeDocsVersion("2.0.0-go.12"); err == nil {
		t.Fatal("runtime-shaped version must not be a docs version")
	}
	env := newTestEnv(t, "frozen")
	if _, err := env.eng.CreateTruthPackage(env.root, "v9.9.9", "feat-x"); err == nil {
		t.Fatal("unregistered explicit version must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonTruthVersionMissing {
		t.Fatalf("reason = %q", rc)
	}
}

func TestTruthCreateGreenfieldDefaultsToV001(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	root := filepath.Join(t.TempDir(), "green")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := st.Projects().Register(store.Project{RootPath: store.ResolveRootPath(root), Name: "green"})
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(st)
	v, err := eng.CreateTruthPackage(root, "", "first-feature")
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "v0.0.1" {
		t.Fatalf("greenfield version = %q", v.Version)
	}
	if !strings.Contains(filepath.ToSlash(v.TruthPath), "docs/beacon/v0.0.1/features/first-feature") {
		t.Fatalf("greenfield path = %s", v.TruthPath)
	}
	versions, err := st.Projects().ListVersions(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].Version != "v0.0.1" || !versions[0].IsCurrent {
		t.Fatalf("greenfield versions = %+v", versions)
	}
}

func TestTruthCreateUsesCurrentVersion(t *testing.T) {
	env := newTestEnv(t, "frozen")
	v, err := env.eng.CreateTruthPackage(env.root, "", "feat-b")
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "v2.0.0" {
		t.Fatalf("current version = %q", v.Version)
	}
}
