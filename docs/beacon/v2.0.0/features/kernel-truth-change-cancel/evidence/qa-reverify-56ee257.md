# QA 独立复验 · kernel-truth-change-cancel @ 56ee257

- Verdict: **PASS**
- Repo: /Users/apple/Developer/Personal/products/beacon-go
- Target: commit `56ee257` (parent `cb7805f`)
- Method: 干净 detached worktree `/tmp/qa-cancel-56ee257`，`go build -o /tmp/qa-cancel-bin ./cmd/...` 重建后实跑；只读，未改代码/truth/tests/tasks/seeds，未 commit，未 push。
- Date: 2026-11-11
- Reviewer: QA 验收 · 回归（grok 独立）

## 逐条实测

### 1. 三条 seed 实跑 → PASS
命令（cwd=/tmp/qa-cancel-56ee257，`BEACON_BIN=/tmp/qa-cancel-bin`，zsh，各自 mktemp 隔离 store）：

```
seed1_cancel_identical_returns_frozen.sh   EXIT=0  RESULT: PASS - identical body cancels to frozen; legacy change needs and accepts --accept-current
seed2_cancel_modified_refuses.sh           EXIT=0  RESULT: PASS - modified body is refused, change_pending kept, refreeze route named, body untouched
seed3_cancel_no_pending_idempotent.sh      EXIT=0  RESULT: PASS (guard) - no-pending cancel is a successful no-op, twice
```

### 2. AC-005 字面合规：成功取消后 changes/<id>.md 字节不变、仍 status: pending；truth.md sha 不变 → PASS
自建 fixture（不走 seed 断言，独立算 sha）：

```
change_doc=docs/beacon/v2.0.0/features/demo/changes/chg_03e28da441d5.md
cancel_rc=0
doc_sha_before=84ea8937aaa2a11c9d4a31caf818e196ddfd152bce7ce567770a3395433774ec
doc_sha_after =84ea8937aaa2a11c9d4a31caf818e196ddfd152bce7ce567770a3395433774ec
doc_bytes_identical=YES
doc_status_before='status: pending'
doc_status_after ='status: pending'
truth_sha_before=6e3a649f0ff154916ccff552fd8b086b92e31644b6cdd92f284385c62ea2d80e
truth_sha_after =6e3a649f0ff154916ccff552fd8b086b92e31644b6cdd92f284385c62ea2d80e
truth_bytes_identical=YES
status=frozen  change_pending=False  cancelled_events=1
```

代码侧：`truth_change.go` 取消成功路径只 append 事件，无 `changes/<id>.md` 写入（`grep` 仅剩注释中的 AC-005 说明）。

### 3. `--accept-current` 不带 `--cancel` → rc=1 → PASS
```
$ beacon truth change --feature kernel-truth-change-cancel --accept-current
RC=1
stderr: Error: --accept-current only applies with --cancel
```
落点：`internal/cli/truth.go` RunE 首行、开 store 之前，普通 `fmt.Errorf`，非 `core.NewError` 新码。

### 4. 无挂起连调两次 cancel → rc=0、stderr 空、不追加事件 → PASS
```
rc_first=0 rc_second=0
stderr_first_len=0 stderr_second_len=0
events_before=3 events_after=3 delta=0
cancelled_events=0
status=frozen pending=False
```

### 5. 事件载荷 → PASS
```
{"accepted_current":false,"base_truth_sha256":"6e3a649f...","baseline_missing":false,"by":"qa",
 "change_doc":"docs/beacon/v2.0.0/features/demo/changes/chg_03e28da441d5.md",
 "change_id":"chg_03e28da441d5","change_ids":["chg_03e28da441d5"],"decision":"cancel",
 "truth_sha256":"6e3a649f..."}
```
含 `decision=cancel`；单变更时平铺 `change_id`/`change_doc`/`base_truth_sha256`。

### 6. 未新增 reason code → PASS
`git diff --stat 56ee257^ 56ee257 -- internal/core/` 为空。

### 7. 冻结文件未动 → PASS
`git diff --stat 56ee257^ 56ee257 -- .../truth.md .../tests.md .../tasks.md .../seeds/` 为空。
提交 7 文件：evidence 新增 4 个（`implement-cancel-reverify-20261111.md` + `seed{1,2,3}.canon.20261111-<sha8>.txt`）+ `internal/cli/truth.go` + `internal/orchestrator/truth_cancel_test.go` + `internal/orchestrator/truth_change.go`。无删改既有 `seed*.canon` / `*.out`。

### 8. `go test ./...` 干净 worktree 全绿 → PASS（exit 0）
尾部：
```
ok  beacon-go/internal/cli            29.838s
ok  beacon-go/internal/evalgate       84.618s
ok  beacon-go/internal/harness        17.003s
ok  beacon-go/internal/orchestrator  151.738s
ok  beacon-go/internal/workspace       6.559s
ok  beacon-go/internal/core / store / session / skills / stride / migration / llm / gate / ...
```
聚焦单测：
```
--- PASS: TestTruthCancelIdenticalBodyReturnsFrozen (0.40s)
--- PASS: TestTruthCancelModifiedBodyRefuses (0.58s)
--- PASS: TestTruthCancelMissingBaselineNeedsAcceptCurrent (0.74s)
--- PASS: TestTruthCancelWithoutPendingIsIdempotent (0.52s)
```

### 9. evidence 自洽 → PASS（口径说明）
`seed{1,2,3}.canon.20261111-<sha8>.txt` 的 `<sha8>` = 该文件去掉前 4 行头部注释后正文的 sha256 前 8 位：
```
seed1.canon.20261111-be8b20fb.txt name=be8b20fb actual=be8b20fb MATCH
seed2.canon.20261111-bb2ac420.txt name=bb2ac420 actual=bb2ac420 MATCH
seed3.canon.20261111-18fb6b60.txt name=18fb6b60 actual=18fb6b60 MATCH
```
旧红基线 `seed{1,2,3}.canon` / `seed*.out` 仍在、未被覆盖（不在提交 diff 内）。
注：canon 含随机 `chg_` id，跨轮只能比字段值、不比整文件原始 sha；这一点已在 canon 头与 `implement-cancel-reverify-20261111.md` 写明。

## 结论
9/9 PASS。AC-001..005 行为、AC-005 字节不变、事件载荷、无新 reason code、冻结文件零改动、干净 worktree `go test` 全绿，均实测成立。release gate verdict 不在本复验范围，未写。
