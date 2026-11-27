package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func bindingFixture(t *testing.T) (string, string, *httptest.Server, string) {
	t.Helper()
	project := treeTestEnv(t)
	seedFreezeTree(t, project, false)
	dir := filepath.Join(project, "docs/beacon/v0.1.0/tree")
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	_, err = intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{
		ACs: []intenttree.AC{{ID: "a1", Text: "first"}, {ID: "a2", Text: "second"}},
		TestCases: []intenttree.TestCase{
			{ID: "tc1", ACID: "a1", Kind: "unit", Ref: "go test ./...", Status: "existing"},
			{ID: "tc2", ACID: "a2", Kind: "manual", Ref: "check second", Status: "planned"},
			{ID: "tc3", ACID: "a1", Kind: "manual", Ref: "keep first covered", Status: "planned"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	origin := "http://" + server.Listener.Addr().String()
	server.Config.Handler = newTreeBindingHandler(st, dir, origin, "test-token")
	server.Start()
	t.Cleanup(server.Close)
	return project, dir, server, origin
}

func bindingValues(t *testing.T, dir string) url.Values {
	t.Helper()
	node, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	return url.Values{"node": {"t1"}, "base_fingerprint": {node.Fingerprint},
		"test_case_id": {"tc1"}, "ac_id": {"a2"}, "csrf_token": {"test-token"}}
}

func postBinding(t *testing.T, server *httptest.Server, origin string, values url.Values) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/binding", strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestTreeBindingHTTPRoundTrip(t *testing.T) {
	project, dir, server, origin := bindingFixture(t)
	resp, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(page), `name="csrf_token" value="test-token"`) {
		t.Fatalf("live page: %d %s", resp.StatusCode, page)
	}
	values := bindingValues(t, dir)
	status, body := postBinding(t, server, origin, values)
	if status != http.StatusOK {
		t.Fatalf("binding: %d %s", status, body)
	}
	var returned intenttree.Node
	if err := json.Unmarshal([]byte(body), &returned); err != nil {
		t.Fatal(err)
	}
	canonical, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	cliNode := parseDoc(t, mustRun(t, "truth", "tree", "--project", project, "--version", "v0.1.0", "--node", "t1", "--json"))
	if canonical.TestCases[0].ACID != "a2" || canonical.Fingerprint == values.Get("base_fingerprint") || returned.Fingerprint != canonical.Fingerprint || cliNode["fingerprint"] != canonical.Fingerprint {
		t.Fatalf("readback mismatch: canonical=%+v returned=%+v cli=%v", canonical, returned, cliNode)
	}
	if canonical.TestCases[1].Ref != "check second" || canonical.Status != intenttree.StatusDraft {
		t.Fatal("binding changed unrelated fields or lifecycle")
	}
	before := hashTreeFiles(t, dir)
	status, body = postBinding(t, server, origin, values)
	if status != http.StatusConflict || hashTreeFiles(t, dir) != before {
		t.Fatalf("stale replay: %d %s", status, body)
	}
	t.Logf("real HTTP POST 200 -> canonical + CLI readback fingerprint=%s; stale replay=409", canonical.Fingerprint)
}

func TestTreeBindingHTTPRefusals(t *testing.T) {
	_, dir, server, origin := bindingFixture(t)
	for _, test := range []struct {
		name   string
		edit   func(url.Values)
		origin string
		want   int
	}{
		{"foreign_origin", func(v url.Values) {}, "https://attacker.invalid", 403},
		{"missing_origin", func(v url.Values) {}, "", 403},
		{"null_origin", func(v url.Values) {}, "null", 403},
		{"bad_token", func(v url.Values) { v.Set("csrf_token", "wrong") }, origin, 403},
		{"blank_fingerprint", func(v url.Values) { v.Set("base_fingerprint", " ") }, origin, 400},
		{"stale_fingerprint", func(v url.Values) { v.Set("base_fingerprint", strings.Repeat("0", 64)) }, origin, 409},
		{"duplicate_key", func(v url.Values) { v.Add("ac_id", "a1") }, origin, 400},
		{"unknown_field", func(v url.Values) { v.Set("status", "draft") }, origin, 400},
		{"path_alias", func(v url.Values) { v.Set("node", "../t1") }, origin, 404},
		{"absolute_alias", func(v url.Values) { v.Set("node", filepath.Join(dir, "t1")) }, origin, 404},
		{"blank_ac", func(v url.Values) { v.Set("ac_id", " ") }, origin, 400},
		{"unknown_ac", func(v url.Values) { v.Set("ac_id", "missing") }, origin, 422},
		{"unknown_tc", func(v url.Values) { v.Set("test_case_id", "missing") }, origin, 422},
		{"reverse_ids", func(v url.Values) { v.Set("ac_id", "tc1"); v.Set("test_case_id", "a2") }, origin, 422},
		{"uncovered_ac", func(v url.Values) { v.Set("test_case_id", "tc2"); v.Set("ac_id", "a1") }, origin, 422},
		{"empty_cases", func(v url.Values) { v.Set("node", "t2") }, origin, 422},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := bindingValues(t, dir)
			if test.name == "empty_cases" {
				node, _ := intenttree.ReadNode(dir, "t2")
				values.Set("base_fingerprint", node.Fingerprint)
			}
			test.edit(values)
			before := hashTreeFiles(t, dir)
			status, body := postBinding(t, server, test.origin, values)
			if status != test.want || hashTreeFiles(t, dir) != before {
				t.Fatalf("got %d %s; want %d and unchanged canonical", status, body, test.want)
			}
		})
	}
}

func TestTreeBindingFrozenAndHostGuards(t *testing.T) {
	project, dir, server, origin := bindingFixture(t)
	if _, err := freeze(t, project, "t1"); err != nil {
		t.Fatal(err)
	}
	before := hashTreeFiles(t, dir)
	status, body := postBinding(t, server, origin, bindingValues(t, dir))
	if status != 409 || !strings.Contains(body, intenttree.ReasonNodeNotDraft) || hashTreeFiles(t, dir) != before {
		t.Fatalf("frozen: %d %s", status, body)
	}
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req.Host = "attacker.invalid"
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("foreign host = %d", resp.StatusCode)
	}
}

func TestTreeBindingConcurrentStaleWrite(t *testing.T) {
	_, dir, server, origin := bindingFixture(t)
	values := bindingValues(t, dir)
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan int, 2)
	for range 2 {
		go func() {
			req, _ := http.NewRequest("POST", server.URL+"/binding", strings.NewReader(values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Origin", origin)
			ready <- struct{}{}
			<-start
			resp, err := server.Client().Do(req)
			if err != nil {
				results <- 0
				return
			}
			resp.Body.Close()
			results <- resp.StatusCode
		}()
	}
	<-ready
	<-ready
	close(start)
	first, second := <-results, <-results
	if !((first == 200 && second == 409) || (first == 409 && second == 200)) {
		t.Fatalf("barrier-synchronized POSTs = %d, %d; want 200 and 409", first, second)
	}
	t.Logf("two executors released by shared barrier: HTTP %d, HTTP %d", first, second)
}

func TestTreeBindingCLIServe(t *testing.T) {
	for _, group := range []string{"plan", "truth"} {
		t.Run(group, func(t *testing.T) {
			project, dir, _, _ := bindingFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader, writer := io.Pipe()
			defer reader.Close()
			cmd := NewRootCmd()
			cmd.SetArgs([]string{group, "tree", "serve", "--project", project, "--version", "v0.1.0"})
			cmd.SetOut(writer)
			done := make(chan error, 1)
			go func() {
				done <- cmd.ExecuteContext(ctx)
				writer.Close()
			}()
			line, err := bufio.NewReader(reader).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			address := strings.Fields(line)[2]
			resp, err := http.Get(address)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			page := string(body)
			prefix := `name="csrf_token" value="`
			_, tail, found := strings.Cut(page, prefix)
			if !found {
				t.Fatalf("missing token in CLI-served page: %s", page)
			}
			token, _, _ := strings.Cut(tail, `"`)
			values := bindingValues(t, dir)
			values.Set("csrf_token", token)
			req, _ := http.NewRequest("POST", address+"binding", strings.NewReader(values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", strings.TrimSuffix(address, "/"))
			client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
			resp, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 303 || resp.Header.Get("Location") != "/#t1" {
				t.Fatalf("native form fallback = %d %s", resp.StatusCode, resp.Header.Get("Location"))
			}
			node := parseDoc(t, mustRun(t, "truth", "tree", "--project", project, "--node", "t1", "--json"))
			if node["test_cases"].([]any)[0].(map[string]any)["ac_id"] != "a2" {
				t.Fatal("CLI server failed canonical readback")
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("CLI server did not stop")
			}
			t.Logf("%s tree serve -> native POST 303 -> CLI canonical readback; cancellation stopped listener", group)
		})
	}
}
