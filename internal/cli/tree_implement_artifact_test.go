package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func TestImplementReportRejectsEmptyArtifactWithoutWrites(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "whitespace": " \t\r\n", "unicode_whitespace": "\u3000\u00a0"} {
		t.Run(name, func(t *testing.T) {
			project := treeTestEnv(t)
			seedFreezeTree(t, project, false)
			if _, err := freeze(t, project, "t1"); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(project, "docs/beacon/v0.1.0/tree")
			artifact := filepath.Join(project, "empty.txt")
			if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(cwd, artifact)
			if err != nil {
				t.Fatal(err)
			}
			before := implementationTreeFiles(t, dir)
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			cachedBefore, err := st.Tree().GetTreeNode("t1")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range []string{"service", "cli"} {
				for _, path := range []string{artifact, relative} {
					if entry == "service" {
						_, err = intenttree.ReportImplementation(st, dir, "t1", "maker", "done", path, "")
					} else {
						_, err = runCLI(t, "implement", "report", "--project", project, "--version", "v0.1.0", "--node", "t1", "--by", "maker", "--artifact", path, "--json")
					}
					if err == nil || !strings.Contains(err.Error(), "tree_artifact_required") {
						t.Fatalf("%s accepted blank artifact %q: %v", entry, path, err)
					}
				}
			}
			after := implementationTreeFiles(t, dir)
			if len(before) != len(after) {
				t.Fatal("rejected report changed the tree file set")
			}
			for path, raw := range before {
				if !bytes.Equal(raw, after[path]) {
					t.Fatalf("rejected report modified %s", path)
				}
			}
			cached, err := st.Tree().GetTreeNode("t1")
			if err != nil {
				t.Fatal(err)
			}
			if cached != cachedBefore || cached.Status != string(intenttree.StatusFrozen) {
				t.Fatalf("rejected report changed cached node: %+v -> %+v", cachedBefore, cached)
			}
		})
	}
}

func TestImplementReportRetryValidatesArtifact(t *testing.T) {
	project := treeTestEnv(t)
	seedFreezeTree(t, project, false)
	if _, err := freeze(t, project, "t1"); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(project, "proof.txt")
	if err := os.WriteFile(artifact, []byte("observed test output\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"implement", "report", "--project", project, "--version", "v0.1.0", "--node", "t1", "--by", "maker", "--artifact", artifact, "--json"}
	if got := parseDoc(t, mustRun(t, args...)); got["status"] != "implemented" || got["already"] != false {
		t.Fatalf("first report: %v", got)
	}
	if got := parseDoc(t, mustRun(t, args...)); got["already"] != true {
		t.Fatalf("idempotent report: %v", got)
	}
	dir := filepath.Join(project, "docs/beacon/v0.1.0/tree")
	before := implementationTreeFiles(t, dir)
	if err := os.WriteFile(artifact, []byte("different evidence\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), intenttree.ReasonEvidenceHashMismatch) {
		t.Fatalf("retry accepted tampered artifact: %v", err)
	}
	copyPath := filepath.Join(project, "original-copy.txt")
	if err := os.WriteFile(copyPath, []byte("observed test output\n"), 0600); err != nil {
		t.Fatal(err)
	}
	copyArgs := append([]string{}, args...)
	copyArgs[len(copyArgs)-2] = copyPath
	if _, err := runCLI(t, copyArgs...); err == nil || !strings.Contains(err.Error(), intenttree.ReasonEvidenceHashMismatch) {
		t.Fatalf("retry accepted replacement path while recorded artifact was tampered: %v", err)
	}
	if err := os.WriteFile(artifact, []byte(" \n\t"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "tree_artifact_required") {
		t.Fatalf("retry accepted blank artifact: %v", err)
	}
	after := implementationTreeFiles(t, dir)
	if len(before) != len(after) {
		t.Fatal("rejected retry changed tree file set")
	}
	for path, raw := range before {
		if !bytes.Equal(raw, after[path]) {
			t.Fatalf("rejected retry modified %s", path)
		}
	}
}

func implementationTreeFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		files[path] = raw
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestLifecycleCacheFailureRollsBackAndRetries(t *testing.T) {
	for _, operation := range []string{"implement", "freeze", "root_freeze", "update"} {
		t.Run(operation, func(t *testing.T) {
			project := treeTestEnv(t)
			seedFreezeTree(t, project, false)
			if operation == "root_freeze" {
				for _, nodeID := range []string{"t1", "t2", "f", "m"} {
					if _, err := freeze(t, project, nodeID); err != nil {
						t.Fatal(err)
					}
				}
			}
			if operation == "implement" {
				if _, err := freeze(t, project, "t1"); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(project, "docs/beacon/v0.1.0/tree")
			nodeID := "t1"
			if operation == "root_freeze" {
				nodeID = "root"
			}
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			artifact := filepath.Join(project, "proof.txt")
			if err := os.WriteFile(artifact, []byte("observed implementation output\n"), 0600); err != nil {
				t.Fatal(err)
			}
			invoke := func() error {
				switch operation {
				case "implement":
					_, err := runCLI(t, "implement", "report", "--project", project, "--node", "t1", "--by", "maker", "--artifact", artifact, "--json")
					return err
				case "freeze", "root_freeze":
					_, err := freeze(t, project, nodeID)
					return err
				default:
					_, err := intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{Title: "new accepted title"})
					return err
				}
			}
			before := hashTreeFiles(t, dir)
			cachedBefore, err := st.Tree().GetTreeNode(nodeID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.DB().Exec(`CREATE TRIGGER lifecycle_cache_failure AFTER UPDATE ON tree_nodes BEGIN SELECT RAISE(FAIL, 'injected cache failure'); END`); err != nil {
				t.Fatal(err)
			}
			if err := invoke(); err == nil {
				t.Fatal("cache failure accepted")
			}
			cached, err := st.Tree().GetTreeNode(nodeID)
			if err != nil || cached != cachedBefore || hashTreeFiles(t, dir) != before {
				t.Fatalf("failed operation changed canonical or cache: %+v %v", cached, err)
			}
			if _, err := st.DB().Exec("DROP TRIGGER lifecycle_cache_failure"); err != nil {
				t.Fatal(err)
			}
			if err := invoke(); err != nil {
				t.Fatal(err)
			}
			node, err := intenttree.ReadNode(dir, nodeID)
			if err != nil {
				t.Fatal(err)
			}
			cached, err = st.Tree().GetTreeNode(nodeID)
			if err != nil || cached.Status != string(node.Status) || cached.Title != node.Title || cached.Fingerprint != node.Fingerprint {
				t.Fatalf("successful retry has inconsistent cache: %+v %+v %v", node, cached, err)
			}
			if operation == "implement" {
				events, err := filepath.Glob(filepath.Join(dir, "events/t1.*.json"))
				if err != nil || len(events) != 1 {
					t.Fatalf("implementation event missing: %v %v", events, err)
				}
				if err := os.Remove(events[0]); err != nil {
					t.Fatal(err)
				}
				if _, err := st.DB().Exec("UPDATE tree_nodes SET status='frozen' WHERE id='t1'"); err != nil {
					t.Fatal(err)
				}
				if err := invoke(); err != nil {
					t.Fatal(err)
				}
				cached, err = st.Tree().GetTreeNode("t1")
				if err != nil || cached.Status != "implemented" {
					t.Fatalf("already did not repair cache: %+v %v", cached, err)
				}
				if _, err := os.Stat(events[0]); err != nil {
					t.Fatalf("already did not restore event: %v", err)
				}
			}
		})
	}
}

func TestFreezeSnapshotFailureAndMissingBodyRetry(t *testing.T) {
	project := treeTestEnv(t)
	seedFreezeTree(t, project, false)
	for _, nodeID := range []string{"t1", "t2", "f", "m"} {
		if _, err := freeze(t, project, nodeID); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(project, "docs/beacon/v0.1.0/tree")
	before := hashTreeFiles(t, dir)
	snapshotPath := filepath.Join(dir, "snapshots")
	if err := os.WriteFile(snapshotPath, []byte("snapshot path blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := freeze(t, project, "root"); err == nil {
		t.Fatal("snapshot failure accepted")
	}
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if hashTreeFiles(t, dir) != before {
		t.Fatal("rejected freeze changed canonical")
	}
	if _, err := freeze(t, project, "root"); err != nil {
		t.Fatal(err)
	}
	ids, err := intenttree.ListSnapshots(dir)
	if err != nil || len(ids) != 1 {
		t.Fatalf("root snapshot missing: %v %v", ids, err)
	}
	if err := os.RemoveAll(filepath.Join(snapshotPath, ids[0])); err != nil {
		t.Fatal(err)
	}
	if _, err := freeze(t, project, "root"); err != nil {
		t.Fatal(err)
	}
	ids, err = intenttree.ListSnapshots(dir)
	if err != nil || len(ids) != 1 {
		t.Fatalf("already did not restore root snapshot: %v %v", ids, err)
	}
	if err := os.Remove(intenttree.NodePath(dir, "t1")); err != nil {
		t.Fatal(err)
	}
	if _, err := freeze(t, project, "t1"); err == nil {
		t.Fatal("already accepted missing canonical body")
	}
}

func TestImplementReportConcurrentContractsAndNodes(t *testing.T) {
	if _, err := exec.LookPath("mkfifo"); err != nil {
		t.Skip("FIFO synchronization requires mkfifo")
	}
	for _, scenario := range []string{"two_nodes", "contract_changed"} {
		t.Run(scenario, func(t *testing.T) {
			project := treeTestEnv(t)
			seedFreezeTree(t, project, false)
			for _, nodeID := range []string{"t1", "t2"} {
				if _, err := freeze(t, project, nodeID); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(project, "docs/beacon/v0.1.0/tree")
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			startReport := func(nodeID string) (*os.File, <-chan error) {
				path := filepath.Join(project, nodeID+".fifo")
				if output, err := exec.Command("mkfifo", path).CombinedOutput(); err != nil {
					t.Fatalf("mkfifo: %v %s", err, output)
				}
				result := make(chan error, 1)
				go func() {
					_, err := intenttree.ReportImplementation(st, dir, nodeID, "maker", "done", path, "")
					result <- err
				}()
				writers := make(chan *os.File, 1)
				go func() {
					writer, _ := os.OpenFile(path, os.O_WRONLY, 0600)
					writers <- writer
				}()
				select {
				case writer := <-writers:
					if writer == nil {
						t.Fatal("FIFO open failed")
					}
					return writer, result
				case <-time.After(10 * time.Second):
					t.Fatal("report did not reach artifact-read barrier")
				}
				return nil, nil
			}
			finish := func(writer *os.File, result <-chan error) error {
				if _, err := writer.Write([]byte("observed nonempty evidence\n")); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-result:
					return err
				case <-time.After(10 * time.Second):
					t.Fatal("report did not finish")
				}
				return nil
			}
			writer, result := startReport("t1")
			if scenario == "two_nodes" {
				otherWriter, otherResult := startReport("t2")
				if err := finish(writer, result); err != nil {
					t.Fatal(err)
				}
				if err := finish(otherWriter, otherResult); err != nil {
					t.Fatal(err)
				}
				idx, err := intenttree.ReadIndex(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range idx.Nodes {
					if entry.ID == "t1" || entry.ID == "t2" {
						node, err := intenttree.ReadNode(dir, entry.ID)
						if err != nil || node.Status != intenttree.StatusImplemented || node.Fingerprint != entry.Fingerprint || node.Status != entry.Status {
							t.Fatalf("accepted report lost: %+v %+v %v", entry, node, err)
						}
					}
				}
			} else {
				node, err := intenttree.ReadNode(dir, "t1")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := intenttree.RequestChange(st, dir, "test-version", "t1", "new requirement", "changer", node.Fingerprint); err != nil {
					t.Fatal(err)
				}
				if _, err := intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{Title: "new accepted contract"}); err != nil {
					t.Fatal(err)
				}
				if _, err := intenttree.FreezeNode(st, dir, "test-version", "t1"); err != nil {
					t.Fatal(err)
				}
				if err := finish(writer, result); err == nil || !strings.Contains(err.Error(), intenttree.ReasonChangeBaseMismatch) {
					t.Fatalf("old report accepted new contract: %v", err)
				}
				node, err = intenttree.ReadNode(dir, "t1")
				if err != nil || node.Title != "new accepted contract" || node.Status != intenttree.StatusFrozen {
					t.Fatalf("old report overwrote contract: %+v %v", node, err)
				}
			}
		})
	}
}
