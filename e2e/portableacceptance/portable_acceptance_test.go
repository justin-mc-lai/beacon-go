package portableacceptance

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	portableBinary string
	portableRoot   string
)

func TestMain(tests *testing.M) {
	portableRoot, _ = filepath.Abs(filepath.Join("..", ".."))
	directory, err := os.MkdirTemp("", "beacon-portable-acceptance-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	portableBinary = filepath.Join(directory, "beacon")
	command := exec.Command("go", "build", "-o", portableBinary, "./cmd/beacon")
	command.Dir = portableRoot
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "CLI build precondition failed: %v\n%s", err, output)
		os.RemoveAll(directory)
		os.Exit(2)
	}
	result := tests.Run()
	os.RemoveAll(directory)
	os.Exit(result)
}

func run(t *testing.T, project, ledger string, arguments ...string) ([]byte, error) {
	t.Helper()
	command := exec.Command(portableBinary, arguments...)
	command.Dir = project
	command.Env = append(os.Environ(), "BEACON_STORE_PATH="+ledger)
	return command.CombinedOutput()
}

func good(t *testing.T, project, ledger string, arguments ...string) map[string]any {
	t.Helper()
	output, err := run(t, project, ledger, arguments...)
	if err != nil {
		t.Fatalf("public CLI failed: %v: %v\n%s", arguments, err, output)
	}
	var document map[string]any
	if err := json.Unmarshal(output, &document); err != nil {
		t.Fatalf("public CLI JSON missing: %v\n%s", err, output)
	}
	return document
}

func array(t *testing.T, project, ledger string, arguments ...string) []any {
	t.Helper()
	output, err := run(t, project, ledger, arguments...)
	if err != nil {
		t.Fatalf("public CLI failed: %v: %v\n%s", arguments, err, output)
	}
	var document []any
	if err := json.Unmarshal(output, &document); err != nil {
		t.Fatalf("public CLI JSON array missing: %v\n%s", err, output)
	}
	return document
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func treeFixture(t *testing.T) (string, string) {
	t.Helper()
	project := t.TempDir()
	ledger := filepath.Join(t.TempDir(), "fixture.db")
	source := filepath.Join(portableRoot, "docs", "beacon", "v0.0.1", "tree")
	target := filepath.Join(project, "docs", "beacon", "v0.0.1", "tree")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, content, 0600)
	}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(project, "go.mod"), "module fixture\n\ngo 1.23\n")
	good(t, project, ledger, "projects", "register", "--path", project, "--version", "v0.0.1", "--json")
	good(t, project, ledger, "doctor", "rebuild-tree", "--project", project, "--version", "v0.0.1", "--json")
	return project, ledger
}

func readNode(t *testing.T, project, id string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(project, "docs", "beacon", "v0.0.1", "tree", "nodes", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func patchNode(t *testing.T, project, ledger, id, summary string, patch map[string]any) {
	t.Helper()
	node := readNode(t, project, id)
	good(t, project, ledger, "change", "request", "--project", project, "--node", id,
		"--base-fingerprint", fmt.Sprint(node["fingerprint"]), "--summary", summary, "--by", "portable-fixture", "--json")
	path := filepath.Join(t.TempDir(), "patch.json")
	write(t, path, string(mustJSON(t, patch)))
	good(t, project, ledger, "truth", "node", "update", "--project", project, "--node", id, "--file", path, "--json")
}

func changeSpec(packageName, run string, tests []string) map[string]any {
	sort.Strings(tests)
	return map[string]any{
		"type": "test",
		"spec": string(mustJSONMap(map[string]any{
			"schema": "beacon-capture-command.v1", "argv": []string{"go", "test", packageName, "-run", run},
			"test_files": tests, "required_tests": []string{run},
		})),
	}
}

func mustJSONMap(value map[string]any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func testFiles(t *testing.T, root, packageName string) []string {
	t.Helper()
	command := exec.Command("go", "list", "-json", packageName)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", packageName, err, output)
	}
	var info struct {
		Dir           string
		TestGoFiles   []string
		XTestGoFiles  []string
	}
	if err := json.Unmarshal(output, &info); err != nil {
		t.Fatal(err)
	}
	files := append([]string{}, info.TestGoFiles...)
	files = append(files, info.XTestGoFiles...)
	sort.Strings(files)
	for index, name := range files {
		absolute, err := filepath.Rel(root, filepath.Join(info.Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[index] = filepath.ToSlash(absolute)
	}
	return files
}

func TestPortableTreeDrift(t *testing.T) {
	project, ledger := treeFixture(t)
	before := good(t, project, ledger, "plan", "drift", "--project", project, "--node", "n_ca6fe1322405", "--json")
	patchNode(t, project, ledger, "n_ca6fe1322405", "portable drift fixture", map[string]any{"title": "AGENTS.md sync guard changed"})
	after := good(t, project, ledger, "plan", "drift", "--project", project, "--node", "n_ca6fe1322405", "--json")
	if len(before["changed"].([]any)) != 0 || len(after["changed"].([]any)) != 1 {
		t.Fatalf("drift did not locate edited body: before=%v after=%v", before["changed"], after["changed"])
	}
}

func TestPortableModuleRun(t *testing.T) {
	project, ledger := treeFixture(t)
	first := array(t, project, ledger, "goal", "module", "run", "--project", project, "--node", "n_5f7fcbdf0840", "--json")
	second := array(t, project, ledger, "goal", "module", "run", "--project", project, "--node", "n_5f7fcbdf0840", "--json")
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("module run count: %d %d", len(first), len(second))
	}
	secondRun := second[1].(map[string]any)
	statusRaw, err := run(t, project, ledger, "goal", "status", "--run-id", fmt.Sprint(secondRun["run_id"]), "--json")
	if err != nil {
		t.Fatalf("goal status: %v\n%s", err, statusRaw)
	}
	var status map[string]any
	if err := json.Unmarshal(statusRaw, &status); err != nil {
		t.Fatal(err)
	}
	if status["blocked"] != false || status["current_stage"] != "plan" {
		t.Fatalf("second run not released at plan: %v", status)
	}
}

func TestPortableModuleAdmission(t *testing.T) {
	project, ledger := treeFixture(t)
	patchNode(t, project, ledger, "n_5d6dde8f4e1a", "portable admission fixture", map[string]any{
		"acs": []map[string]any{{"id": "AC-001", "text": "draft gate fixture"}},
		"verification": changeSpec("./e2e/portableacceptance", "^TestPortableModuleAdmission$", testFiles(t, portableRoot, "./e2e/portableacceptance")),
	})
	output, err := run(t, project, ledger, "goal", "module", "run", "--project", project, "--node", "n_5f7fcbdf0840", "--json")
	if err == nil || !strings.Contains(string(output), "freeze_ack_required") {
		t.Fatalf("draft child was admitted: %v\n%s", err, output)
	}
}

func TestPortableParallelPlan(t *testing.T) {
	project, ledger := treeFixture(t)
	runs := array(t, project, ledger, "goal", "module", "run", "--project", project, "--node", "n_5f7fcbdf0840", "--json")
	if len(runs) != 2 {
		t.Fatalf("module runs: %v", runs)
	}
	for _, item := range runs {
		run := item.(map[string]any)
		if run["position"] == float64(0) && run["state"] != "running" {
			t.Fatalf("first run state: %v", run)
		}
		if run["position"] == float64(1) && (run["state"] != "pending" || run["predecessor_run_id"] == nil) {
			t.Fatalf("second run is not pool-gated: %v", run)
		}
	}
}

func TestPortableTreeHTMLPanels(t *testing.T) {
	project, ledger := treeFixture(t)
	node := readNode(t, project, "n_bd880ecae011")
	verification := changeSpec("./e2e/portableacceptance", "^TestPortableBindingWriteBack$", []string{"portable_acceptance_test.go"})
	patchNode(t, project, ledger, "n_bd880ecae011", "portable binding panel fixture", map[string]any{
		"acs": node["acs"],
		"test_cases": []map[string]any{{
			"id": "TC-PORTABLE-BIND", "ac_id": "AC-001", "kind": "e2e",
			"ref": "e2e/portableacceptance/portable_acceptance_test.go", "status": "existing",
		}},
		"verification": verification,
	})
	output, err := run(t, project, ledger, "plan", "tree", "--project", project, "--format", "html")
	if err != nil {
		t.Fatalf("tree html: %v\n%s", err, output)
	}
	path := filepath.Join(project, ".beacon", "artifacts", "tree", "v0.0.1", "index.html")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(bytes)
	for _, want := range []string{"gate-panel", "Gate 判定", "data-bind-ac=", "data-bind-testcase="} {
		if !strings.Contains(raw, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	command := exec.CommandContext(ctx, portableBinary, "plan", "tree", "serve", "--project", project, "--version", "v0.0.1")
	command.Dir = project
	command.Env = append(os.Environ(), "BEACON_STORE_PATH="+ledger)
	command.Stdout = writer
	command.Stderr = writer
	done := make(chan error, 1)
	go func() { done <- command.Run(); writer.Close() }()
	address, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatalf("tree serve: %v", err)
	}
	addressPattern := regexp.MustCompile(`https?://[^\s]+`)
	matched := addressPattern.FindString(address)
	if matched == "" {
		t.Fatalf("tree serve did not print a loopback address: %q", address)
	}
	address = strings.TrimSuffix(matched, "/")
	response, err := http.Get(address + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !strings.Contains(string(body), `name="csrf_token"`) {
		t.Fatal("binding page missing CSRF token")
	}
	if !strings.Contains(string(body), `data-bind-ac=`) {
		t.Fatal("binding page missing canonical binding rows")
	}
	tokenPattern := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	match := tokenPattern.FindSubmatch(body)
	if len(match) != 2 {
		t.Fatal("binding page missing csrf token value")
	}
	form := url.Values{}
	form.Set("node", "n_bd880ecae011")
	form.Set("base_fingerprint", fmt.Sprint(readNode(t, project, "n_bd880ecae011")["fingerprint"]))
	form.Set("test_case_id", "TC-PORTABLE-BIND")
	form.Set("ac_id", "AC-001")
	form.Set("csrf_token", string(match[1]))
	request, err := http.NewRequest(http.MethodPost, address+"/binding", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", address)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response2, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	written, _ := io.ReadAll(response2.Body)
	response2.Body.Close()
	if response2.StatusCode != http.StatusOK || !strings.Contains(string(written), "TC-PORTABLE-BIND") {
		t.Fatalf("binding write-back failed: %d %s", response2.StatusCode, written)
	}
	if readNode(t, project, "n_bd880ecae011")["fingerprint"] == fmt.Sprint(node["fingerprint"]) {
		t.Fatal("binding write-back did not change canonical fingerprint")
	}
	cancel()
	<-done
}

func TestPortableBindingWriteBack(t *testing.T) {
	project, ledger := treeFixture(t)
	node := readNode(t, project, "n_bd880ecae011")
	patch := filepath.Join(t.TempDir(), "binding.json")
	write(t, patch, string(mustJSON(t, map[string]any{
		"acs": node["acs"],
		"test_cases": []map[string]any{{
			"id": "TC-PORTABLE-BIND", "ac_id": "AC-001", "kind": "e2e",
			"ref": "e2e/portableacceptance/portable_acceptance_test.go", "status": "existing",
		}},
		"verification": changeSpec("./e2e/portableacceptance", "^TestPortableBindingWriteBack$", testFiles(t, portableRoot, "./e2e/portableacceptance")),
	})))
	good(t, project, ledger, "change", "request", "--project", project, "--node", "n_bd880ecae011",
		"--base-fingerprint", fmt.Sprint(node["fingerprint"]), "--summary", "portable binding write-back", "--by", "portable-fixture", "--json")
	good(t, project, ledger, "truth", "node", "update", "--project", project, "--node", "n_bd880ecae011", "--file", patch, "--json")
	node = readNode(t, project, "n_bd880ecae011")
	binding := node["test_cases"].([]any)
	found := false
	for _, item := range binding {
		caseRow := item.(map[string]any)
		if fmt.Sprint(caseRow["id"]) == "TC-PORTABLE-BIND" && fmt.Sprint(caseRow["ac_id"]) == "AC-001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("binding did not persist: %v", binding)
	}
	verification := node["verification"].(map[string]any)
	if !strings.Contains(fmt.Sprint(verification["spec"]), `TestPortableBindingWriteBack`) {
		t.Fatalf("binding spec did not persist: %v", verification)
	}
}

func TestPortableVersionManifestAlignment(t *testing.T) {
	project, ledger := treeFixture(t)
	bumped := good(t, project, ledger, "projects", "bump", "--path", project, "--level", "patch", "--json")
	manifest, ok := bumped["manifest"].(string)
	if !ok || manifest == "" {
		t.Fatalf("projects bump did not emit a manifest path: %v", bumped)
	}
	versionOutput, err := run(t, project, ledger, "--version")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(versionOutput))
	versionPattern := regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`)
	version = versionPattern.FindString(version)
	if version == "" {
		t.Fatalf("version: %q", versionOutput)
	}
	manifestPath := manifest
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifestDoc map[string]any
	if err := json.Unmarshal(raw, &manifestDoc); err != nil {
		t.Fatal(err)
	}
	found := false
	for key, value := range manifestDoc {
		if strings.EqualFold(key, "runtime") && value == version {
			found = true
		}
	}
	if !found {
		t.Fatalf("manifest runtime missing or mismatched: %s binary=%q", raw, version)
	}
}

func TestPortableStrictDeliveryContract(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(portableRoot, "skills")
	target := filepath.Join(project, "skills")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, content, 0600)
	}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(project, "go.mod"), "module beacon-go\n\ngo 1.23\n")
	command := exec.Command("go", "test", "./skills", "-run", "^TestStrictDelivery$", "-count=1")
	command.Dir = project
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("strict delivery contract: %v\n%s", err, output)
	}
}

func TestPortableCaptureLifecycle(t *testing.T) {
	project, ledger := treeFixture(t)
	behavior := filepath.Join(project, "portablebehavior")
	write(t, filepath.Join(behavior, "behavior.go"), "package portablebehavior\nfunc Ready() bool{return false}\n")
	write(t, filepath.Join(behavior, "behavior_test.go"), "package portablebehavior\nimport \"testing\"\nfunc TestBehavior(t *testing.T){if !Ready(){t.Fatal(\"behavior missing\")}}\n")
	node := filepath.Join(project, "docs", "beacon", "v0.0.1", "tree", "nodes", "n_bd880ecae011.json")
	raw, err := os.ReadFile(node)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	patch, _ := json.Marshal(map[string]any{
		"verification": map[string]any{
			"type": "test",
			"spec": string(mustJSON(t, map[string]any{
				"schema": "beacon-capture-command.v1", "argv": []string{"go", "test", "./portablebehavior", "-run", "^TestBehavior$"},
				"test_files": []string{"portablebehavior/behavior_test.go"}, "required_tests": []string{"TestBehavior"},
			})),
		},
	})
	patchPath := filepath.Join(project, "patch.json")
	write(t, patchPath, string(patch))
	good(t, project, ledger, "change", "request", "--project", project, "--node", "n_bd880ecae011", "--base-fingerprint", fmt.Sprint(document["fingerprint"]), "--summary", "portable capture fixture", "--by", "portable", "--json")
	good(t, project, ledger, "truth", "node", "update", "--project", project, "--node", "n_bd880ecae011", "--file", patchPath, "--json")
	good(t, project, ledger, "truth", "node", "freeze", "--project", project, "--node", "n_bd880ecae011", "--json")
	red, err := run(t, project, ledger, "implement", "capture", "--project", project, "--version", "v0.0.1", "--node", "n_bd880ecae011", "--ac", "AC-001", "--phase", "red", "--by", "portable")
	if err != nil {
		t.Fatalf("red capture: %v\n%s", err, red)
	}
	var redDoc map[string]any
	if err := json.Unmarshal(red, &redDoc); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(behavior, "behavior.go"), "package portablebehavior\nfunc Ready() bool{return true}\n")
	greenRaw, err := run(t, project, ledger, "implement", "capture", "--project", project, "--version", "v0.0.1", "--node", "n_bd880ecae011", "--ac", "AC-001", "--phase", "green", "--red", fmt.Sprint(redDoc["artifact_id"]), "--by", "portable")
	if err != nil {
		t.Fatalf("green capture: %v\n%s", err, greenRaw)
	}
	var green map[string]any
	if err := json.Unmarshal(greenRaw, &green); err != nil {
		t.Fatal(err)
	}
	if green["phase"] != "green" || green["status"] != "accepted" {
		t.Fatalf("green capture: %v", green)
	}
}

func TestPortableStrictDeliveryGuard(t *testing.T) {
	project, ledger := treeFixture(t)
	node := readNode(t, project, "n_bd880ecae011")
	patch := filepath.Join(t.TempDir(), "patch.json")
	write(t, patch, string(mustJSON(t, map[string]any{
		"verification": map[string]any{"type": "test", "spec": `{"schema":"unknown"}`},
	})))
	good(t, project, ledger, "change", "request", "--project", project, "--node", "n_bd880ecae011", "--base-fingerprint", fmt.Sprint(node["fingerprint"]), "--summary", "portable guard", "--by", "portable", "--json")
	good(t, project, ledger, "truth", "node", "update", "--project", project, "--node", "n_bd880ecae011", "--file", patch, "--json")
	good(t, project, ledger, "truth", "node", "freeze", "--project", project, "--node", "n_bd880ecae011", "--json")
	before := sha256Hex(mustJSON(t, readNode(t, project, "n_bd880ecae011")))
	output, err := run(t, project, ledger, "implement", "capture", "--project", project, "--version", "v0.0.1", "--node", "n_bd880ecae011", "--ac", "AC-001", "--phase", "red", "--by", "portable")
	if err == nil || !strings.Contains(string(output), "capture_command_invalid") {
		t.Fatalf("invalid command accepted: %v\n%s", err, output)
	}
	if after := sha256Hex(mustJSON(t, readNode(t, project, "n_bd880ecae011"))); after != before {
		t.Fatal("rejected capture mutated canonical node")
	}
}

func TestPortableTimestampOrdering(t *testing.T) {
	first := time.Now().UTC()
	time.Sleep(time.Millisecond)
	second := time.Now().UTC()
	if !second.After(first) {
		t.Fatal("timestamps did not advance")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
