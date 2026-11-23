---
slug: kernel-autotest-shell-tc
version: v2.0.0
status: frozen
revision_id: R3-draft
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence.md
---

# Requirement Truth: Shell TC execution for qa autotest (R3-draft)

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "让 qa autotest 支持 shell 型 TC，别只能跑 go test"
goal: "qa autotest 能执行 tests.md 里的 shell 脚本型 TC 并记录每 TC 退出码与输出；只有 TC 数大于 0 且全部通过才补写真实 stage_executed；失败、blocked、空集继续拒绝，marker 规则不放宽"
non_goals: "不放宽 placeholder/marker 规则；不新增 reason code；不改 gate 语义；不 push 不发布"
constraints: "Go 内核；BEACON_BIN 驱动；失败与空集必须拒绝；测试与实现产物分离"
scope_mode: focused
```

## 人话

现在 `beacon qa autotest run` 把 tests.md 每一行的命令拆成 argv 直接执行。TC 若写成项目内相对脚本路径（例如 `scripts/check.sh`），`exec.LookPath` 会按 CLI 当前目录查找，找不到就报 `executor_command_missing`，脚本根本没跑，结果行 exit=-1、stdout/stderr 为空；显式写 `sh scripts/check.sh` 才能跑通。本 feature 让项目内 `.sh`、`.zsh` 或首行带受支持 shebang 的脚本 TC 经对应解释器在项目根执行，逐 TC 落真实 exit 与输出；只有 TC 数大于 0 且全部通过才写真实 qa `stage_executed`，失败、blocked 与空集继续拒绝，现有门禁语义保持不变。

## 起点现场

- 单一真源为 beacon-go 仓库，HEAD `36c40b6`，工作树干净。
- 本 feature 包位于该仓库 `docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/`；truth、tests、tasks、evidence、seeds 与后续实现同属一个仓库。
- `qa autotest run` 只存在于 Go 内核；本包所有复现都在 beacon-go 根用 `go build -o /tmp/beacon-seed ./cmd/beacon` 的产物完成。
- 本阶段未建 goal run、未 tick、未 freeze、未改生产代码。

## 当前行为（HEAD 36c40b6，beacon-go 根实测）

| ID | 实测事实 |
|----|----------|
| F1 | `qa autotest run` 对 `scripts/ok_shell.sh`（644，带 shebang）返回 `total=2 passed=0 failed=2`；两条结果均 `status=failed`、`exit_code=-1`、`reason_code=executor_command_missing`，stdout 为空；qa `stage_executed=0`，`autotest_executed=1`。 |
| F2 | 失败脚本 TC 返回 `exit_code=-1` 而不是脚本自己的 7，stdout/stderr 都为空。 |
| F3 | 显式 `sh scripts/ok_noexec.sh` 可以执行，exit=0，并写入 qa `stage_executed=1`；解释器链路本身是通的。 |
| F4 | 空 tests.md（只有表头）与只有 `BLOCKED:` 行的矩阵都返回非 0，错误 `autotest_no_executable_tests`；qa/autotest 事件均为 0，无 report。 |
| F5 | 含管道符、重定向、命令替换或链式操作符的命令被 `SplitCommandWords` 拒绝为 `autotest_shell_syntax_unsupported`，不会执行。 |
| F6 | 聚合规则为 `report.Total > 0 && report.Failed == 0 && report.Blocked == 0` 才写 qa `stage_executed`，事件带 kernel 执行器身份与 `outcome:passed`。 |
| F7 | 门禁只承认真实 qa 事件：`gates_facts.go` 要求执行器身份字段存在且为非人工执行值、`recording` 缺失或为 false；`evaluate.go` 据此判定 `qa_stage_executed`。 |

## Scope

### In scope

- tests.md 的 TC 第一段是项目内相对路径且文件存在时，按扩展名或 shebang 选择解释器。
- `.sh` 用 `/bin/sh`，`.zsh` 用 `/bin/zsh`，无扩展名读首行 shebang。
- 显式 `sh ...`、`zsh ...` 与其他 PATH 命令保持原语义。
- 路径按项目根解析，不受 CLI 当前目录影响；含空格路径作为一个 argv 元素。
- 每条 TC 落 `stdout.log`、`stderr.log`、`result.json`，报告含真实 exit code。
- 聚合：TC 数大于 0 且全部 passed 才写真实 qa `stage_executed`；失败、blocked、空集保持拒绝。
- 真正执行了至少一条 TC 时继续写 `autotest_executed` 审计事件。

### Out of scope

- 引入 shell（用 `sh -c` 执行整串命令）、管道、重定向、命令替换或任何 shell 语法。
- 放宽 `BLOCKED:` 语义、空命令 fallback 与真源完整性扫描。
- 改动 truth 包结构、门禁判定、release 与 human gate。
- 自动 `chmod +x`、自动生成 shebang、自动安装解释器。

## Execution Contract

### 识别顺序

1. 按 `SplitCommandWords` 得到 argv；任一 shell 元字符继续拒绝。
2. argv[0] 若含 `/` 且是项目根下的已存在普通文件，视为脚本候选；否则维持 PATH 查找。
3. 脚本候选按扩展名选择解释器：`.sh` 用 `/bin/sh`，`.zsh` 用 `/bin/zsh`；无扩展名读首行 shebang。
4. 解释器不存在或脚本不可读时，结果为 failed 或 blocked 并记录明确 reason，不静默回退到 PATH。
5. 其余命令仍按 `exec.LookPath` 与 `exec.CommandContext` 执行。

### 证据契约

| Surface | Contract |
|---|---|
| per-TC exit | 报告 `exit_code` 等于脚本真实退出码（seed2 为 7），不是 -1 |
| stdout | `.beacon/autotest/<run>/<TC>/stdout.log` 保存脚本 stdout |
| stderr | `.beacon/autotest/<run>/<TC>/stderr.log` 保存脚本 stderr |
| result | `.beacon/autotest/<run>/<TC>/result.json` 保存状态、exit、reason、时长 |
| report | `.beacon/autotest/<run>/report.json` 含 total/passed/failed/blocked 与逐条结果 |
| qa event | 仅全绿且 total 大于 0 时写 `stage_executed`，字段见下节 |

### qa stage_executed 事件字段

```json
{"stage": "qa", "executor": "kernel_autotest", "source": "qa_autotest", "placeholder": false, "recording": false, "outcome": "passed"}
```

事件语义：该事件由内核自动测试真实执行后写出，执行器身份为 `kernel_autotest`，来源为 `qa_autotest`，不是人工登记或录像回放。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | tests.md 中以 `.sh` 与 `.zsh` 结尾的文件、以及首行带受支持 shebang 的项目内脚本，用对应解释器执行；显式解释器与 PATH 命令行为不变 |
| AC-002 | 相对脚本路径按项目根解析，不受 CLI 当前目录影响；含空格路径保持为单一 argv 元素 |
| AC-003 | 每条 TC 记录真实 exit code、stdout、stderr；失败脚本保留自身退出码与输出 |
| AC-004 | 只有 TC 数大于 0 且全部通过时才写真实 qa `stage_executed`，事件带 kernel 执行器身份与 `outcome:passed`；失败、blocked、空集不写 |
| AC-005 | 既有约束全部保持：`BLOCKED:` 不执行、空命令 fallback 不放宽、无 job 硬拒绝、shell 语法拒绝、门禁只认真实事件 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 派单 restatement：shell 型 TC 用对应解释器执行 | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed1.canon |
| AC-002 | 派单写包硬规则：路径按项目根解析 | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed1.canon |
| AC-003 | 派单 dod：跑通并落每 TC 证据 | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed2.canon |
| AC-004 | 派单 restatement：TC 数大于 0 且全部通过才补写 stage_executed | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed3.canon |
| AC-005 | 派单 restatement：失败与空集保持拒绝，marker 规则不放宽 | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed3.canon |

## Non-negotiable constraints（不得放宽）

| # | Constraint | 现有兜底校验点 |
|---|---|---|
| N1 | `BLOCKED:` 开头的 TC 仍是人工逃逸口，计为 blocked，永不执行，并使 `allPassed=false` | `internal/orchestrator/autotest.go` |
| N2 | 空命令只有在项目配置了 `test_command`、`autotest_command` 或 qa strategy fallback 时才可执行；否则 blocked | `internal/orchestrator/autotest.go` |
| N3 | 整个矩阵没有可执行 job 时必须硬拒绝，不写任何事件与 report | `internal/orchestrator/autotest.go` |
| N4 | `Total > 0 && Failed == 0 && Blocked == 0` 才写 qa `stage_executed`，事件字段见上文 | `internal/orchestrator/autotest.go` |
| N5 | 门禁只认真实 qa 事件：执行器身份字段必须存在且为非人工执行值，`recording` 必须存在且为 false | `internal/orchestrator/gates_facts.go`、`internal/gate/evaluate.go` |
| N6 | 不引入 shell：管道、重定向、命令替换等仍以 `autotest_shell_syntax_unsupported` 拒绝 | `internal/orchestrator/cmdsplit.go` |
| N7 | 真源完整性扫描保持启用，相关 reason code 不变 | `internal/gate/integrity.go`、`internal/gate/clarity.go` |

## Stop Line

本包只到 plan 打包：四件套与三条 seed 齐备，S1 与 S2 当前红，S3 为守护绿。不 freeze、不 tick、不写业务代码、不写任何 QA verdict。
