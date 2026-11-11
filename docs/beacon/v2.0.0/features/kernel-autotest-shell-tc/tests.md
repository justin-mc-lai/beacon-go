---
slug: kernel-autotest-shell-tc
version: v2.0.0
status: draft
revision_id: R3-draft
materials_status: current
---

# Tests: kernel-autotest-shell-tc

所有命令都在 beacon-go 仓库根执行。先构建一次：

```bash
go build -o /tmp/beacon-seed ./cmd/beacon
```

Command 列是仓库根下的单条可执行命令。每条 seed 自己 `mktemp` 建隔离 store 与临时项目，不读写全局 `~/.beacon` 与仓库 `.beacon/state`。

## TC 矩阵

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-SH-001 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh` | `.sh` 与无扩展名 shebang 脚本都经解释器执行，`total=2 passed=2`，两条 stdout 分别含 `hello-from-shell` 与 `hello-shebang` |
| AC-002 | TC-SH-002 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh` | 脚本路径按项目根解析：seed 在仓库根启动、项目在 mktemp 目录，仍能找到脚本并执行 |
| AC-003 | TC-SH-003 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh` | 每条 TC `status=passed`、`exit_code=0`，`stdout.log` 落盘且有内容 |
| AC-003 | TC-SH-004 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed2_failed_shell_tc_no_passed.sh` | 失败脚本保留自身 `exit_code=7`，stdout 含 `partial-out`，stderr 含 `boom-err` |
| AC-004 | TC-SH-005 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh` | 全绿且 total=2 时恰好写 1 条 qa `stage_executed`，`autotest_executed=1` |
| AC-004 | TC-SH-006 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed2_failed_shell_tc_no_passed.sh` | 失败套件 qa `stage_executed=0`，`autotest_executed=1`，report 存在且记 failed |
| AC-004 | TC-SH-007 | guard | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed3_empty_tc_no_passed.sh` | 空矩阵与只有 `BLOCKED:` 行的矩阵都返回 `autotest_no_executable_tests`，qa=0、autotest=0、无 report |
| AC-005 | TC-SH-008 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed3_empty_tc_no_passed.sh` | `BLOCKED:` 不被执行；空集不因 shell 改动变成 pass |

## 当前实测（HEAD 36c40b6，2026-11-11，beacon-go 根）

| Seed | Exit | canon sha256 | 关键实测 | 归类 |
|------|------|--------------|----------|------|
| seed1_shell_tc_executes | 1 | `6ed34ca4e5c8ceb43570bec9edb43321299808fa115ef69d65d56a9eae136db3` | total=2 passed=0 failed=2；两条 `executor_command_missing`、exit=-1、stdout 空；qa=0、autotest=1 | 红，实现后须转绿 |
| seed2_failed_shell_tc_no_passed | 1 | `18edc88107aeb521a209045c6b79ab27b06bf695a7ee3765035821bb70578c50` | total=1 failed=1、exit=-1、`executor_command_missing`；stdout/stderr 空；qa=0、autotest=1 | 红，实现后须转绿（真实 exit 7 与输出） |
| seed3_empty_tc_no_passed | 0 | `3e0e989c1e408e9be36c246cd3b5825e74cb8b60d12f479d0154620e851d415e` | 空矩阵与 blocked-only 均 rc=1、`autotest_no_executable_tests`；qa=0、autotest=0、无 report | 绿，守护回归 |

S2 实测归类：当前为红。失败路径虽然已经拒绝写 qa 证据，但退出码与输出证据是错的（exit=-1，stdout/stderr 空），所以按“失败 shell TC 保留真实 exit=7 与输出”的契约判红，不按“未写 passed”单点判绿。

原始输出：`docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/plan-seed-red.log`，逐条全文分别在 `docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed1_shell_tc_executes.out`、`docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed2_failed_shell_tc_no_passed.out`、`docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed3_empty_tc_no_passed.out`。

## 修复后行为契约

| 场景 | total | passed | failed | blocked | qa stage_executed | autotest_executed | report |
|------|-------|--------|--------|---------|-------------------|-------------------|--------|
| shell TC 全部通过（total 大于 0） | N | N | 0 | 0 | 1 | 1 | present |
| 至少一条失败 | N | 小于 N | 大于 0 | 0 | 0 | 1 | present，记 failed 与真实 exit |
| 混合（有可执行 + 有 BLOCKED 行） | N | 见结果 | 见结果 | 大于 0 | 0 | 1 | present，blocked 行不执行 |
| 空矩阵 | 0 | 0 | 0 | 0 | 0 | 0 | absent |
| 只有 BLOCKED 行 | 0 | 0 | 0 | 见结果 | 0 | 0 | absent（无 job 硬拒绝） |

## 负向断言

- tests.md 中 `BLOCKED:` 开头的 TC 不得进入执行队列，且必须使套件不写 qa passed 证据。
- 空 tests.md、只有表头的矩阵、只有 `BLOCKED:` 的矩阵不得写 `stage_executed` 或 `autotest_executed`。
- 管道、`&`、`;`、重定向、反引号、`$(` 等 shell 语法不得被执行；必须继续返回 `autotest_shell_syntax_unsupported`。
- 实现不得引入 `sh -c` 或任何字符串拼接执行。
- 失败脚本不得被记成 passed；`exit_code` 不得被吞成 0 或 -1。
- 事件缺少执行器身份字段或 `recording:false` 时，门禁不得认作 qa 已执行（`internal/orchestrator/gates_facts.go`、`internal/gate/evaluate.go`）。

## Seed 文件 sha256

| 文件 | sha256 |
|------|--------|
| docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/lib.sh | `15f63c3448b7958130430cf8589522f08bb8eac7621a74c35c109b700e8f6dc0` |
| docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh | `e6c8350ef5999a13dd78e084d155daeb6e159c6381558f4a716a30a3b495a24f` |
| docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed2_failed_shell_tc_no_passed.sh | `d0b85e6ed6b9c0c7f49701f01c769a0408c6414872dc1c431fb2ef5c71e87b53` |
| docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed3_empty_tc_no_passed.sh | `6b0f4ca75c322642542d9df448b643ccb31f22cd1cd49dd664938c0a750a90be` |
