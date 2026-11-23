package versionmanifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInheritKeepsHashesWithoutCopy(t *testing.T) {
	parent := Manifest{
		Schema: Schema, Version: "v2.0.0", Status: StatusReleased,
		Lakes: []Lake{
			{Slug: "a", Package: "hash-a", Status: LakeBaseline},
			{Slug: "b", Package: "hash-b", Status: LakeBaseline},
		},
		Seas: []Sea{{Slug: "s", HostFeature: "a", Frozen: true, Lakes: []SeaLake{{Slug: "a"}, {Slug: "b", DependsOn: []string{"a"}}}}},
	}
	child := Inherit(parent, "v2.0.1")
	if child.Parent != "v2.0.0" || child.Status != StatusDraft {
		t.Fatalf("child = %+v", child)
	}
	if len(child.Lakes) != 2 || child.Lakes[0].Package != "hash-a" || child.Lakes[0].Status != LakeInherited {
		t.Fatalf("lakes = %+v", child.Lakes)
	}
	if len(child.Seas) != 1 || len(child.Seas[0].Lakes) != 2 {
		t.Fatalf("seas = %+v", child.Seas)
	}
}

func TestValidateRejectsThinManifest(t *testing.T) {
	bad := Manifest{Schema: Schema, Version: "v2.0.0", Status: StatusDraft,
		Lakes: []Lake{{Slug: "a"}}}
	if err := Validate(bad); err == nil {
		t.Fatal("lake without package must be rejected")
	}
	dup := Manifest{Schema: Schema, Version: "v2.0.0", Status: StatusDraft,
		Lakes: []Lake{{Slug: "a", Package: "h"}, {Slug: "a", Package: "h2"}}}
	if err := Validate(dup); err == nil {
		t.Fatal("duplicate lake must be rejected")
	}
}

func TestWritePackageAndSaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	hash := HashContract([]byte("truth"), []byte("tests"), []byte("tasks"), []byte("rest"))
	meta, err := WritePackage(root, "a", hash, "v2.0.0", map[string][]byte{
		"truth.md": []byte("truth"), "tests.md": []byte("tests"),
		"tasks.md": []byte("tasks"), "restatement.md": []byte("rest"),
	}, []string{"chg_1"})
	if err != nil {
		t.Fatal(err)
	}
	if meta.ContractHash != hash || meta.Files["truth.md"] == "" {
		t.Fatalf("meta = %+v", meta)
	}
	if _, err := os.Stat(filepath.Join(PackageDir(root, "a", hash), "package.json")); err != nil {
		t.Fatal(err)
	}
	// Idempotent: the same bytes and hash write the same files.
	if _, err := WritePackage(root, "a", hash, "v2.0.0", map[string][]byte{
		"truth.md": []byte("truth"), "tests.md": []byte("tests"),
		"tasks.md": []byte("tasks"), "restatement.md": []byte("rest"),
	}, []string{"chg_1"}); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Schema: Schema, Version: "v2.0.0", Status: StatusBaseline,
		Lakes: []Lake{{Slug: "a", Package: hash, Status: LakeBaseline}}}
	path := Path(root, "v2.0.0")
	if err := Save(path, m); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Version != "v2.0.0" || len(got.Lakes) != 1 {
		t.Fatalf("load = %+v err=%v", got, err)
	}
}
