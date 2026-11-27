package cli

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func TestTreeBindingCacheFailureRollsBack(t *testing.T) {
	_, dir, server, origin := bindingFixture(t)
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cached, err := st.Tree().GetTreeNode("t1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TRIGGER reject_tree_cache AFTER UPDATE ON tree_nodes BEGIN SELECT RAISE(FAIL, 'injected cache failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := hashTreeFiles(t, dir)
	values := bindingValues(t, dir)
	status, body := postBinding(t, server, origin, values)
	if status != 422 || hashTreeFiles(t, dir) != before {
		t.Fatalf("cache rejection changed canonical: status=%d body=%s", status, body)
	}
	after, err := st.Tree().GetTreeNode("t1")
	if err != nil || after != cached {
		t.Fatalf("cache rejection changed cached node: %+v -> %+v; %v", cached, after, err)
	}
	if _, err := st.DB().Exec("DROP TRIGGER reject_tree_cache"); err != nil {
		t.Fatal(err)
	}
	if status, body := postBinding(t, server, origin, values); status != 200 {
		t.Fatalf("same fingerprint retry after recovery: %d %s", status, body)
	}
}

func TestTreeBindingIndependentWorkspacesRejectStaleWrite(t *testing.T) {
	_, dir, first, firstOrigin := bindingFixture(t)
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	second := httptest.NewUnstartedServer(nil)
	secondOrigin := "http://" + second.Listener.Addr().String()
	second.Config.Handler = newTreeBindingHandler(st, dir, secondOrigin, "test-token")
	second.Start()
	defer second.Close()
	seed, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	for extra := 0; extra < 2000; extra++ {
		seed.TestCases = append(seed.TestCases, intenttree.TestCase{ID: fmt.Sprintf("extra-%d", extra), ACID: "a1", Kind: "unit", Ref: "fixture", Status: "planned"})
	}
	for attempt := 0; attempt < 10; attempt++ {
		if _, err := intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{TestCases: seed.TestCases}); err != nil {
			t.Fatal(err)
		}
		firstValues, secondValues := bindingValues(t, dir), bindingValues(t, dir)
		secondValues.Set("test_case_id", "tc3")
		ready, start, results := make(chan struct{}, 2), make(chan struct{}), make(chan int, 2)
		for _, call := range []struct {
			server *httptest.Server
			origin string
			second bool
		}{{first, firstOrigin, false}, {second, secondOrigin, true}} {
			go func() {
				values := firstValues
				if call.second {
					values = secondValues
				}
				ready <- struct{}{}
				<-start
				status, _ := postBinding(t, call.server, call.origin, values)
				results <- status
			}()
		}
		<-ready
		<-ready
		close(start)
		one, two := <-results, <-results
		if !((one == 200 && two == 409) || (one == 409 && two == 200)) {
			t.Fatalf("independent workspaces attempt=%d statuses=%d,%d; want one accepted and one stale", attempt, one, two)
		}
	}
}

func TestTreeBindingConcurrentFreezePreservesFrozenContract(t *testing.T) {
	project, dir, server, origin := bindingFixture(t)
	values := bindingValues(t, dir)
	ready, start := make(chan struct{}, 2), make(chan struct{})
	frozen := make(chan error, 1)
	saved := make(chan int, 1)
	go func() {
		ready <- struct{}{}
		<-start
		_, err := freeze(t, project, "t1")
		frozen <- err
	}()
	go func() {
		ready <- struct{}{}
		<-start
		status, _ := postBinding(t, server, origin, values)
		saved <- status
	}()
	<-ready
	<-ready
	close(start)
	if err := <-frozen; err != nil {
		t.Fatal(err)
	}
	status := <-saved
	node, err := intenttree.ReadNode(dir, "t1")
	if err != nil || node.Status != intenttree.StatusFrozen {
		t.Fatalf("concurrent save undid freeze: %+v %v", node, err)
	}
	if status == 200 && node.TestCases[0].ACID != "a2" {
		t.Fatal("freeze lost the accepted binding")
	}
	if status != 200 && status != 409 {
		t.Fatalf("save racing freeze returned %d", status)
	}
	if status == 409 && node.TestCases[0].ACID != "a1" {
		t.Fatal("refused binding changed the frozen contract")
	}
}
