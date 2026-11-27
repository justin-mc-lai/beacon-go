package cli

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func TestPortableIntentTreeRead(t *testing.T) {
	t.Run("local_without_store", func(t *testing.T) {
		proj := treeTestEnv(t)
		mustRun(t, "plan", "init", "--project", proj, "--title", "Portable project")
		want := mustRun(t, "plan", "tree", "--project", proj, "--json")
		if doc := parseDoc(t, want); doc["version"] != "v0.1.0" || doc["node_count"] != float64(1) || doc["root_id"] == "" {
			t.Fatalf("invalid reference tree: %s", want)
		}
		assertKeys(t, parseDoc(t, want), "schema", "version", "root_id", "tree_dir", "node_count", "counts", "status_counts", "nodes", "unlinked", "tree_native", "delivered_count")
		if len(parseDoc(t, want)) != 11 {
			t.Fatalf("unexpected JSON fields: %s", want)
		}
		missing := filepath.Join(t.TempDir(), "missing", "store.db")
		t.Setenv(store.EnvStorePath, missing)
		before := hashTreeFiles(t, proj)
		for _, group := range []string{"plan", "truth"} {
			for _, opts := range [][]string{nil, {"--version", "0.1.0"}} {
				args := append([]string{group, "tree", "--project", proj, "--json"}, opts...)
				if got := mustRun(t, args...); got != want {
					t.Fatalf("%v: JSON contract changed\ngot %s\nwant %s", args, got, want)
				}
			}
		}
		if hashTreeFiles(t, proj) != before {
			t.Fatal("read modified project files")
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("read must not create ledger: %v", err)
		}
		checkPortableRejection(t, proj, "no current version", []string{"--version", "current"})
	})
	t.Run("local_no_tree", func(t *testing.T) {
		other := treeTestEnv(t)
		proj := t.TempDir()
		checkPortableRejection(t, proj, "no intent tree", nil)
		if out, err := runCLI(t, "projects", "show", "--project", proj); err == nil {
			t.Fatalf("read registered target project: %s (other project %s)", out, other)
		}
		t.Setenv(store.EnvStorePath, filepath.Join(t.TempDir(), "missing.db"))
		checkPortableRejection(t, proj, "no intent tree", nil)
	})
	t.Run("local_legacy_store", func(t *testing.T) {
		proj := treeTestEnv(t)
		seedPortableTree(t, proj, "v0.1.0")
		path := storePathFromEnv(t)
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("PRAGMA user_version = 1")
		closeErr := db.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("prepare legacy ledger: %v, %v", err, closeErr)
		}
		checkPortableRead(t, proj, "v0.1.0", "--version", "v0.1.0")
	})
	t.Run("version_registered_current", func(t *testing.T) {
		proj := treeTestEnv(t)
		seedPortableTree(t, proj, "v0.1.0")
		mustRun(t, "projects", "bump", "--project", proj, "--level", "minor")
		seedPortableTree(t, proj, "v0.2.0")
		checkPortableRead(t, proj, "v0.2.0")
		checkPortableRead(t, proj, "v0.2.0", "--version", "current")
		checkPortableRead(t, proj, "v0.1.0", "--version", "v0.1.0")
	})
	t.Run("version_registered_docs_root", func(t *testing.T) {
		proj := treeTestEnv(t)
		seedPortableTree(t, proj, "v0.1.0")
		treeTestEnv(t) // Use a separate ledger to register the same project with a custom root.
		mustRun(t, "projects", "register", "--path", proj, "--version", "v0.1.0", "--docs-root", "custom/requirements")
		mustRun(t, "plan", "init", "--project", proj, "--title", "Custom root")
		assertPortableReadOnly(t, proj, func(group string) {
			for _, opts := range [][]string{nil, {"--version", "v0.1.0"}} {
				out := mustRun(t, append([]string{group, "tree", "--project", proj, "--json"}, opts...)...)
				if got := parseDoc(t, out)["tree_dir"]; got != filepath.Join(store.ResolveRootPath(proj), "custom", "requirements", "tree") {
					t.Fatalf("%s tree: registered docs root ignored, got %v", group, got)
				}
			}
		})
	})
	t.Run("version_unregistered", func(t *testing.T) {
		proj := treeTestEnv(t)
		seedPortableTree(t, proj, "v0.1.0")
		treeTestEnv(t) // The active ledger now knows only the other project.
		checkPortableRead(t, proj, "v0.1.0")
	})
	t.Run("version_reject_ambiguous", func(t *testing.T) {
		proj := treeTestEnv(t)
		seedPortableTree(t, proj, "v0.1.0")
		mustRun(t, "projects", "bump", "--project", proj, "--level", "minor")
		seedPortableTree(t, proj, "v0.2.0")
		t.Setenv(store.EnvStorePath, filepath.Join(t.TempDir(), "missing.db"))
		checkPortableRejection(t, proj, "v0.1.0, v0.2.0", nil)
		checkPortableRead(t, proj, "v0.2.0", "--version", "v0.2.0")
	})
	t.Run("version_reject_explicit", func(t *testing.T) {
		for _, withStore := range []bool{true, false} {
			proj := treeTestEnv(t)
			seedPortableTree(t, proj, "v0.1.0")
			if !withStore {
				t.Setenv(store.EnvStorePath, filepath.Join(t.TempDir(), "missing.db"))
			}
			checkPortableRead(t, proj, "v0.1.0")
			args := []string{"--version", "v0.9.0"}
			checkPortableRejection(t, proj, "no intent tree for version v0.9.0", args)
			dir := filepath.Join(proj, "docs", "beacon", "v0.9.0", "tree")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
			checkPortableRejection(t, proj, "unexpected end of JSON input", args)
		}
	})
}

func seedPortableTree(t *testing.T, project, version string) {
	t.Helper()
	mustRun(t, "plan", "init", "--project", project, "--version", version, "--title", "Portable "+version)
}

func checkPortableRead(t *testing.T, project, version string, opts ...string) {
	t.Helper()
	assertPortableReadOnly(t, project, func(group string) {
		out := mustRun(t, append([]string{group, "tree", "--project", project, "--json"}, opts...)...)
		doc := parseDoc(t, out)
		if doc["version"] != version || doc["node_count"] != float64(1) {
			t.Fatalf("%s tree: wanted %s, got %s", group, version, out)
		}
	})
}

func checkPortableRejection(t *testing.T, project, reason string, opts []string) {
	t.Helper()
	assertPortableReadOnly(t, project, func(group string) {
		out, err := runCLI(t, append([]string{group, "tree", "--project", project, "--json"}, opts...)...)
		if err == nil || !strings.Contains(err.Error(), reason) || out != "" {
			t.Fatalf("%s tree: wanted rejection %q and no tree, got %v and %q", group, reason, err, out)
		}
	})
}

func assertPortableReadOnly(t *testing.T, project string, check func(group string)) {
	t.Helper()
	path := storePathFromEnv(t)
	want, err := os.ReadFile(path)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		t.Fatal(err)
	}
	before := hashTreeFiles(t, project)
	for _, group := range []string{"plan", "truth"} {
		check(group)
		got, err := os.ReadFile(path)
		if (missing && !os.IsNotExist(err)) || (!missing && (err != nil || !bytes.Equal(got, want))) {
			t.Fatalf("%s tree changed the ledger: %v", group, err)
		}
		if hashTreeFiles(t, project) != before {
			t.Fatalf("%s tree changed project files", group)
		}
	}
}
