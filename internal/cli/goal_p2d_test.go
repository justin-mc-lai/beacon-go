package cli

import (
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
)

// seedP2DProject seeds a project whose version carries the host
// feature plus three frozen lake features.
func seedP2DProject(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "p2d-proj")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "p2d-proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"feat-a", "lake-a", "lake-b", "lake-c"} {
		f, err := st.Features().CreatePackage(store.FeaturePackage{
			ProjectVersionID: v.ID, Slug: slug, TruthPath: "truth.md", TestsPath: "tests.md",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// P2d CLI chain: version -> program plan(freeze)/admit/route ->
// hotfix build/validate -> multi-version -> memory -> axis +
// drift after a program change.
