package cli

import (
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"

	"modernc.org/sqlite"
)

func init() {
	if err := sqlite.RegisterScalarFunction("binding_test_make_readonly", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		return int64(1), os.Chmod(args[0].(string), 0555)
	}); err != nil {
		panic(err)
	}
}

func TestTreeBindingRollbackFailureRetainsRecoveryJournal(t *testing.T) {
	project, dir, server, origin := bindingFixture(t)
	nodesDir := filepath.Join(dir, "nodes")
	t.Cleanup(func() { _ = os.Chmod(nodesDir, 0755) })
	if err := os.Chmod(nodesDir, 0555); err != nil {
		t.Fatal(err)
	}
	probe, err := os.CreateTemp(nodesDir, "permission-probe-*")
	if err == nil {
		probe.Close()
		os.Remove(probe.Name())
		os.Chmod(nodesDir, 0755)
		t.Skip("filesystem permits writes despite directory permissions")
	}
	if !os.IsPermission(err) {
		t.Fatal(err)
	}
	if err := os.Chmod(nodesDir, 0755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	statement := fmt.Sprintf(`CREATE TRIGGER fail_binding_rollback AFTER UPDATE ON tree_nodes BEGIN SELECT binding_test_make_readonly('%s'); SELECT RAISE(FAIL, 'injected cache failure'); END`, strings.ReplaceAll(nodesDir, "'", "''"))
	if _, err := st.DB().Exec(statement); err != nil {
		t.Fatal(err)
	}
	before := hashTreeFiles(t, dir)
	values := bindingValues(t, dir)
	status, body := postBinding(t, server, origin, values)
	if status != 422 || !strings.Contains(body, "permission denied") {
		t.Fatalf("rollback failure: %d %s", status, body)
	}
	journal := filepath.Join(project, ".beacon/locks/tree-change-journal.json")
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("rollback failure lost recovery journal: %v", err)
	}
	if err := os.Chmod(nodesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec("DROP TRIGGER fail_binding_rollback"); err != nil {
		t.Fatal(err)
	}
	stale := values.Get("base_fingerprint")
	node, err := intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{ExpectedFingerprint: stale})
	if err != nil || node.Fingerprint != stale || hashTreeFiles(t, dir) != before {
		t.Fatalf("next shared update did not recover original state: %+v %v", node, err)
	}
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatalf("recovered update left journal: %v", err)
	}
	if status, body := postBinding(t, server, origin, values); status != 200 {
		t.Fatalf("same-base retry after recovery: %d %s", status, body)
	}
}

func TestTreeBindingCommittedCacheJournalCleanupFailureRecovers(t *testing.T) {
	project, dir, server, origin := bindingFixture(t)
	lockDir := filepath.Join(project, ".beacon/locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockDir, 0755) })
	if err := os.Chmod(lockDir, 0555); err != nil {
		t.Fatal(err)
	}
	probe, err := os.CreateTemp(lockDir, "permission-probe-*")
	if err == nil {
		probe.Close()
		os.Remove(probe.Name())
		os.Chmod(lockDir, 0755)
		t.Skip("filesystem permits writes despite directory permissions")
	}
	if !os.IsPermission(err) {
		t.Fatal(err)
	}
	if err := os.Chmod(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cachedBefore, err := st.Tree().GetTreeNode("t1")
	if err != nil {
		t.Fatal(err)
	}
	before := hashTreeFiles(t, dir)
	statement := fmt.Sprintf(`CREATE TRIGGER fail_journal_cleanup AFTER UPDATE ON tree_nodes BEGIN SELECT binding_test_make_readonly('%s'); END`, strings.ReplaceAll(lockDir, "'", "''"))
	if _, err := st.DB().Exec(statement); err != nil {
		t.Fatal(err)
	}
	values := bindingValues(t, dir)
	if status, body := postBinding(t, server, origin, values); status != 422 || !strings.Contains(body, "permission denied") {
		t.Fatalf("cleanup failure: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(lockDir, "tree-change-journal.json")); err != nil {
		t.Fatalf("cleanup failure lost journal: %v", err)
	}
	cached, err := st.Tree().GetTreeNode("t1")
	if err != nil || cached != cachedBefore || hashTreeFiles(t, dir) != before {
		t.Fatalf("failed cleanup did not restore committed cache: %+v %v", cached, err)
	}
	if err := os.Chmod(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec("DROP TRIGGER fail_journal_cleanup"); err != nil {
		t.Fatal(err)
	}
	if status, body := postBinding(t, server, origin, values); status != 200 {
		t.Fatalf("retry after cleanup failure: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(lockDir, "tree-change-journal.json")); !os.IsNotExist(err) {
		t.Fatalf("successful retry retained journal: %v", err)
	}
}
