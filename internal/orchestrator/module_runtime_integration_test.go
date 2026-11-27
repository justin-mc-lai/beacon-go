package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
)

func seedModuleRuntimeTree(t *testing.T, env gitEnv, laterStatus intenttree.Status) {
	t.Helper()
	dir := intenttree.TreeDir(env.root, "docs")
	stamp := func(node intenttree.Node) intenttree.Node {
		node, err := intenttree.Stamp(node)
		if err != nil {
			t.Fatal(err)
		}
		return node
	}
	doc := "## 问题\n- x\n## 目标用户\n- x\n## 场景\n- x\n## 范围\n- x\n## 非目标\n- x\n## 验收标准\n- x\n"
	one := stamp(intenttree.Node{ID: "alpha-one", Kind: intenttree.KindFeature, ParentID: "alpha", Title: "one", Path: "root/alpha/one", Status: laterStatus, Doc: doc})
	two := stamp(intenttree.Node{ID: "alpha-two", Kind: intenttree.KindFeature, ParentID: "alpha", Title: "two", Path: "root/alpha/two", Status: intenttree.StatusFrozen, Doc: doc})
	beta := stamp(intenttree.Node{ID: "beta-one", Kind: intenttree.KindFeature, ParentID: "beta", Title: "one", Path: "root/beta/one", Status: intenttree.StatusFrozen, Doc: doc})
	nodes := []intenttree.Node{
		stamp(intenttree.Node{ID: "root", Kind: intenttree.KindIntent, Title: "root", Path: "root", Status: intenttree.StatusFrozen}),
		stamp(intenttree.Node{ID: "alpha", Kind: intenttree.KindModule, ParentID: "root", Title: "alpha", Path: "root/alpha", Status: intenttree.StatusFrozen, ChildFingerprints: []string{two.Fingerprint, one.Fingerprint}}),
		one, two,
		stamp(intenttree.Node{ID: "beta", Kind: intenttree.KindModule, ParentID: "root", Title: "beta", Path: "root/beta", Status: intenttree.StatusFrozen, ChildFingerprints: []string{beta.Fingerprint}}),
		beta,
	}
	idx := intenttree.Index{Schema: intenttree.Schema, Version: "v2.0.0", RootID: "root"}
	for _, node := range nodes {
		if err := intenttree.WriteNode(dir, node); err != nil {
			t.Fatal(err)
		}
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{
			ID: node.ID, ParentID: node.ParentID, Kind: node.Kind, Status: node.Status,
			Title: node.Title, Path: node.Path, Fingerprint: node.Fingerprint, FingerprintAlg: node.FingerprintAlg,
		})
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
}

func TestModuleRuntimeCrossModuleWorkersBarrier(t *testing.T) {
	env := newGitEnv(t)
	seedModuleRuntimeTree(t, env, intenttree.StatusFrozen)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runs := make(map[string][]ModuleRunView)
	for _, module := range []string{"alpha", "beta"} {
		views, err := env.eng.ModuleRun(ModuleRunOptions{ProjectRoot: env.root, Version: "v2.0.0", NodeID: module, By: "module-host"})
		if err != nil {
			t.Fatalf("ModuleRun %s: %v", module, err)
		}
		runs[module] = views
	}
	if len(runs["alpha"]) != 2 || len(runs["beta"]) != 1 {
		t.Fatalf("module expansion: %+v", runs)
	}
	first, pending := runs["alpha"][0], runs["alpha"][1]
	if first.FeatureNodeID != "alpha-two" || pending.FeatureNodeID != "alpha-one" || pending.PredecessorRunID != first.RunID {
		t.Fatalf("declaration order/predecessor lost: %+v", runs["alpha"])
	}
	before, err := env.st.Runs().GetByID(pending.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		view, err := env.eng.TickBy(ctx, pending.RunID, "module-host")
		if err != nil || view.ReasonCode != core.ReasonModuleRunPending || !view.UpToDate || view.Blocked || view.CurrentStage != StagePlan || view.ModuleRun == nil || view.ModuleRun.State != ModuleRunStatePending {
			t.Fatalf("pending tick %d must refuse advancement: %+v %v", attempt, view, err)
		}
	}
	after, err := env.st.Runs().GetByID(pending.RunID)
	if err != nil || string(before.Metadata) != string(after.Metadata) || env.rec.CallCount() != 0 {
		t.Fatalf("pending tick mutated metadata or called an executor: %v", err)
	}
	t.Logf("pending refusal: run=%s reason=%s, repeated ticks leave metadata unchanged and executor calls=0", pending.RunID, core.ReasonModuleRunPending)
	paths := make([]string, 0, 2)
	branches := make([]string, 0, 2)
	for _, module := range []string{"alpha", "beta"} {
		run := runs[module][0]
		view, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{Lake: module, AllowDirty: true})
		if err != nil {
			t.Fatalf("WorktreeCreate %s: %v", module, err)
		}
		if view.PlanMode != ModulePoolMode || view.ParallelPlan == nil || len(view.ParallelPlan.Satellites) != 2 || len(view.Records) != 1 {
			t.Fatalf("module pool must materialize one of two satellites: %+v", view)
		}
		record := view.Records[0]
		if record.Lake != module || record.Ref != ".beacon/worktrees/satellite/"+module || record.State != WorktreeActive {
			t.Fatalf("wrong module workstation: %+v", record)
		}
		if got := strings.TrimSpace(gitCmd(t, record.Path, "rev-parse", "--show-toplevel")); got != record.Path {
			t.Fatalf("satellite is not a real independent git worktree: %q != %q", got, record.Path)
		}
		paths = append(paths, record.Path)
		branches = append(branches, strings.TrimSpace(gitCmd(t, record.Path, "branch", "--show-current")))
	}
	if paths[0] == paths[1] || branches[0] == branches[1] {
		t.Fatalf("modules share a workstation: paths=%v branches=%v", paths, branches)
	}
	if err := moduleWorkersBarrier(ctx, paths); err != nil {
		t.Fatal(err)
	}
	pids := map[string]bool{}
	for _, path := range paths {
		started, err := os.ReadFile(filepath.Join(path, "worker.started"))
		if err != nil {
			t.Fatal(err)
		}
		crossed, err := os.ReadFile(filepath.Join(path, "worker.barrier"))
		if err != nil || string(started) != string(crossed) {
			t.Fatalf("worker did not cross host barrier: %s %q %q %v", path, started, crossed, err)
		}
		if pid := strings.TrimSpace(string(started)); pid == "" || pids[pid] {
			t.Fatalf("workers must have distinct nonempty process IDs: %q", pid)
		} else {
			pids[pid] = true
		}
		t.Logf("real subprocess barrier: path=%s branch=%s pid=%s", path, strings.TrimSpace(gitCmd(t, path, "branch", "--show-current")), strings.TrimSpace(string(started)))
	}
	view, err := env.eng.TickBy(ctx, first.RunID, "module-host")
	if err != nil || view.Blocked || view.CurrentStage != StageTruth || view.StagePlan[0].Status != "done" {
		t.Fatalf("predecessor plan completion: %+v %v", view, err)
	}
	released, err := env.eng.Status(pending.RunID)
	if err != nil || released.ModuleRun == nil || released.ModuleRun.State != ModuleRunStateRunning || released.CurrentStage != StagePlan || released.StagePlan[0].Status != "active" || released.Blocked {
		t.Fatalf("successor was not released: %+v %v", released, err)
	}
	if !eventSeen(t, env, pending.RunID, "module_run_released") {
		t.Fatal("successor release did not leave an event")
	}
	beta, err := env.eng.Status(runs["beta"][0].RunID)
	if err != nil || beta.CurrentStage != StagePlan || eventSeen(t, env, beta.RunID, "module_run_released") {
		t.Fatalf("alpha predecessor changed an unrelated module: %+v %v", beta, err)
	}
	view, err = env.eng.TickBy(ctx, pending.RunID, "module-host")
	if err != nil || view.Blocked || view.CurrentStage != StageTruth {
		t.Fatalf("released successor cannot advance: %+v %v", view, err)
	}
	if env.rec.CallCount() != 0 {
		t.Fatal("manual module runs invoked a kernel executor")
	}
	t.Logf("predecessor release: %s plan=done; %s pending -> running -> truth; kernel executor calls=0", first.RunID, pending.RunID)
}

func moduleWorkersBarrier(ctx context.Context, paths []string) error {
	script := `
set -eu
printf '%s\n' "$$" > worker.started
printf 'ready\n'
IFS= read -r token
test "$token" = release
test -f "$1/worker.started"
cat worker.started > worker.barrier
`
	type worker struct {
		cmd   *exec.Cmd
		input io.WriteCloser
		ready *bufio.Reader
	}
	var workers []worker
	defer func() {
		for _, worker := range workers {
			worker.input.Close()
			if worker.cmd.ProcessState == nil {
				worker.cmd.Process.Kill()
				worker.cmd.Wait()
			}
		}
	}()
	for position, path := range paths {
		cmd := exec.CommandContext(ctx, "sh", "-c", script, "sh", paths[1-position])
		cmd.Dir = path
		input, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			input.Close()
			return err
		}
		if err := cmd.Start(); err != nil {
			input.Close()
			output.Close()
			return err
		}
		workers = append(workers, worker{cmd: cmd, input: input, ready: bufio.NewReader(output)})
	}
	for position, worker := range workers {
		ready, err := worker.ready.ReadString('\n')
		if err != nil || ready != "ready\n" {
			return fmt.Errorf("worker %s readiness: %q %v", paths[position], ready, err)
		}
		if _, err := os.Stat(filepath.Join(paths[position], "worker.barrier")); !os.IsNotExist(err) {
			return fmt.Errorf("worker %s crossed before host release: %v", paths[position], err)
		}
	}
	for _, worker := range workers {
		if _, err := io.WriteString(worker.input, "release\n"); err != nil {
			return err
		}
		worker.input.Close()
	}
	for position, worker := range workers {
		if err := worker.cmd.Wait(); err != nil {
			return fmt.Errorf("worker %s barrier: %w", paths[position], err)
		}
	}
	return nil
}

func TestModuleRuntimeAdmissionRefusesAllRuns(t *testing.T) {
	for _, status := range []intenttree.Status{intenttree.StatusDraft, intenttree.StatusChangePending} {
		t.Run(string(status), func(t *testing.T) {
			env := newGitEnv(t)
			seedModuleRuntimeTree(t, env, status)
			views, err := env.eng.ModuleRun(ModuleRunOptions{ProjectRoot: env.root, Version: "v2.0.0", NodeID: "alpha", By: "module-host"})
			if reason, ok := core.ReasonCodeOf(err); !ok || reason != core.ReasonFreezeAckRequired || len(views) != 0 {
				t.Fatalf("later %s child must reject entire module: views=%+v error=%v", status, views, err)
			}
			runs, err := env.st.Runs().ListAll()
			if err != nil || len(runs) != 0 {
				t.Fatalf("refused module created runs: %+v %v", runs, err)
			}
			features, err := env.st.ListFeatureContexts(env.root)
			if err != nil || len(features) != 1 || features[0].Feature.ID != env.feature.ID {
				t.Fatalf("refused module materialized feature packages: %+v %v", features, err)
			}
			if _, err := os.Stat(filepath.Join(env.root, "docs", "features")); !os.IsNotExist(err) {
				t.Fatalf("refused module wrote derived package files: %v", err)
			}
			t.Logf("later child=%s -> %s; runs=0, derived packages=0", status, core.ReasonFreezeAckRequired)
		})
	}
}
