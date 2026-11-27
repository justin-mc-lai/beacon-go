# SDD truth contract (two files)

## Strict delivery host contract

六点交付协议的规范执行说明位于
`skills/beacon-go-verify/references/strict-delivery.md`，由七个 skill 强制引用：

| 控制 | SDD 落点 | 执行与证据 |
|---|---|---|
| Plan-First / 原子任务 | 计划、AC、tasks.md DAG | 确认后实施，逐任务推进 |
| Red-Green / 断言保护 | TC 与任务 Verify | 预期行为 Red、Green、测试修订记录或不适用说明 |
| 独立盲审 | 原始需求与候选 diff | 宿主新上下文只读 reviewer 回执 |
| 铁证门禁 | 验证命令与 evidence 引用 | 真实退出码、日志、候选与需求指纹 |
| 外部账本 | tasks.md 定义与执行回执 | 逐项核销，展示清单从执行事实派生 |
| 确定性工具 | 任务 Command/Verify | 真实 CLI/脚本接口与输入输出 |

`bash scripts/check_delivery_contract.sh` 检查协议及安装后引用的完整性。
合同分发检查不是宿主写工具权限钩子。树叶验证现在还需要运行时回执，格式如下。

### 树叶运行时回执（beacon-delivery.v1）

在已有 source_commit / binary_sha256 / requirement_fingerprint 头部之后，
加入单行 `delivery_receipt: <JSON>`。该 JSON 的字段由
`internal/intenttree/delivery_receipt.go` 的 DeliveryReceipt 定义：

- schema=`beacon-delivery.v1`、node_id、requirement_fingerprint、source_commit、binary_sha256、maker。
- tasks：当前叶子的每个 AC 必须恰好一个条目（ac_id、status=`done`、red、green）；空集合、遗漏、未知或重复 AC 被拒绝。
- red/green：command（argv）、显式 exit_code、started_at/finished_at（RFC3339）、test_sha256、log_path/log_sha256。
  同一命令和测试 hash，Red 非零、Green 零，Red 结束后才能开始 Green。
  另需 capture_id 指向当前节点保留种类 capture_result；上述自报字段必须与实际登记的采集记录逐项一致。
- review：reviewer、verdict=`pass`、open_findings=0、receipt_path/receipt_sha256。
  独立 JSON 文档还必须携带 node_id、requirement_fingerprint、source_commit、binary_sha256、reviewer、
  verdict、findings（空数组）、reviewed_at；时间必须在 Green 之后，且 maker 与 reviewer 不同。
  另需 source_sha256 绑定最终 Green 输入，以及 captures 映射每个 AC 到 Green artifact ID。

日志和回执路径相对证据文件目录解析，符号链接解析后仍须留在该目录；文件不能为空，hash 必须匹配。
`verify leaf` 和共享 MarkVerified 服务拒绝缺失/无效回执；rollup、deliver、goal 的 qa/release
树门重新校验已登记证据。失败返回 reason code 与非零 CLI 退出，不推进节点状态。
旧式只有文本 transcript 或元数据的证据不再满足树叶验证。

本回执是叶子 AC 核销，不替代 feature 级 tasks.md DAG 的 TaskStatus 执行事实。
存在 intent tree 且采用 truth-tasks 合同的 goal，在 qa/release 阶段检查还要求
TaskStatus 的当前调用和每个任务均 passed；未开始、失败或候选过期均返回 delivery_ledger_open。
当前树迁移还复核进程采集来源、当前源码/测试 hash；手填完整 v1 字段不能替代保留采集记录。
这些检查不能认证测试断言语义或审查者真实身份。
无树的 legacy gate 尚未接入此回执；不适用 TDD 的树叶暂不提供自动豁免，须保留阻塞并另行确定合同。

### 同类入口扫描

| 入口 | 共用校验 | 拒绝路径/测试 |
|---|---|---|
| verify leaf / MarkVerified | 登记作用域、工件 hash、DeliveryReceipt | TestDeliveryReceiptCLIRefusals；失败保持 implemented |
| verify audit | ValidateEvidenceFile | 元数据不足、日志/回执 stale 均上报 |
| verify rollup | ValidateVerifiedDelivery | TestDeliveryReceiptStaleBlocksRollupAndDelivery |
| truth node deliver | 递归 ValidateVerifiedDelivery | 同上；已 verified 的父节点也不能绕过 |
| goal tick / complete 的树阶段检查 | ValidateFeatureDelivery；SDD TaskStatus | qa/release 重验，legacy 无树行为保留 |
| verify evaluate / release evaluate | ValidateFeatureDelivery | 直接评估也不能只凭旧状态通过 |

测试 fixture 中的日志为合成输入，只用于验证拒绝逻辑，不是本功能通过真实 TDD/独立审查的交付证据。

`beacon truth create --sdd` lays down two files:

| File | Role |
|---|---|
| `truth.md` | the SDD Spec: intent snapshot, open questions, intent coverage, subagent plan, FSM (diagram + transitions), acceptance criteria, AC/TC matrix, out-of-scope |
| `tasks.md` | task DAG with dependencies, AC bindings and evidence paths |

`tests.md`, `evidence.md` and `restatement.md` are not required. Evidence lives in
`evidence/` and is referenced from tasks and the coverage matrix. Old four-piece
packages keep working unchanged.

## Freeze contract

Creation produces an unfilled scaffold, never a pre-approved requirement. Fill the
intent, grounding table (`Source | Reference | Supports`), assumptions, AC/TC
matrix and executable task graph. Declare `package_author`, `fsm_required` and
`negative_required` explicitly. SDD evidence traceability cannot be disabled.

Every AC needs a positive behavior case. When `negative_required: true`, each AC
also needs a negative case. TC IDs must be unique; empty, documentation-only and
success-only commands do not prove behavior. Counts are not a substitute for coverage.
Stateful requirements carry a normalized FSM plus `Transition Coverage` rows:
`From | To | Trigger | TC ID`. Stateless work declares `fsm_required: false`.
For identical transition triples in different aggregates, prepend an `Aggregate`
column and bind each aggregate separately. Optional prose-only FSM rationale is
not a model; a present malformed transition table still blocks.

Before confirmation, an independent reviewer supplies a JSON receipt:

```json
{"fingerprint":"<current fingerprint>","reviewer":"checker","verdict":"pass","findings":[],"summary":"What was reviewed and why the requirements agree with the grounded intent"}
```

Inspect the fingerprint with `beacon truth semantic-review --project . --feature
<slug> --json`; record the external receipt with the same command plus `--file
review.json`. Then use `truth confirm --by <owner>` and `truth freeze`. The kernel
records the receipt on its event trail. Editing truth or task definitions invalidates
the review and confirmation. Identities are caller-attested, not authenticated;
the kernel does not perform semantic judgment or call a model.

## Task execution

`tasks.md` uses the exact table:

```text
TASK | Action | Depends | AC | Command | Verify | Evidence | Auto
```

Dependencies and ACs are comma-separated; `-` means no dependency. Cycles,
unknown dependencies, duplicate task IDs and unknown ACs are refused. Use
`beacon implement tasks run --run-id <id>` after admission. The runner executes
Command then Verify in deterministic dependency order and stops on failure.
`beacon implement tasks status --run-id <id>` reads the latest bound report.
Before success, every verification runs again against the final candidate so a
later task cannot silently invalidate an earlier task. Evidence files are copied
to immutable invocation/task snapshots; shared authored evidence paths do not
overwrite history. Candidate hashes cover project inputs with explicit recorded
runtime/output exclusions. Direct `go test` verification forces fresh verbose
execution and rejects zero tests or skipped cases; other custom commands remain
trusted verification programs, not a proof of semantic assertion quality.

Repair is disabled by default. Opt in with `--repair-cmd <program>`, repeatable
`--repair-arg`, `--max-repairs 1..5` and `--timeout-sec`. The external repair
receives pointers to failed result/stdout/stderr through environment variables.
Each attempt is retained under `.beacon/task-runs/` and registered by hash.
Repairing the requirement contract, stale candidates, cancellation, timeout and
exhaustion fail rather than becoming completion. No automatic release or install
lane is provided. Executables are trusted project code; this is workdir isolation,
not a container or security sandbox.

## FSM runtime

Both `State | Event | Guard | To | Action` and
`From | To | Trigger | Guard | Side effect` normalize to the same graph.
`truth fsm check` validates the graph; `fsm transition --from A --to B` refuses
missing, invalid or ambiguous edges. Use `--event` to disambiguate and repeat
`--guard '<exact text>'` for caller-attested satisfied guards. This is not an
evaluation of live business state. Refusals emit structured output and exit nonzero.

## Compatibility

- Legacy packages (no `package_contract: truth-tasks`) still require the four-piece set.
- `truth migrate --dry-run` previews legacy-to-SDD adoption; `--tasks-file` supplies an authored executable DAG when a checkbox ledger cannot be translated without guessing. Original tests/evidence/restatement files remain intact and truth/tasks are backed up. Migration invalidates old confirmation and requires normal completion/review/signing again. See `docs/beacon/v2.1.5/sdd-migration-examples.md`.

## Deterministic draft intake

`truth decompose` performs non-LLM rule matching and reports `semantic_ready: false`.
For SDD it updates only its own suggestions section and a runtime decomposition
artifact. It does not replace authored ACs, coverage or tasks, and it never
turns a discovered test command into claimed semantic coverage.

## Content-bearing lake proposals

Opt in with proposal `package_contract: truth-tasks` and each lake's authored
`truth` / `tasks` strings. The proposal validates applicability, AC/TC/DAG references
and applicable state models, but reports `sdd_content_validated_draft`, not frozen.
Legacy proposals report `legacy_shape_only`. Selection creates two-file drafts;
every selected SDD package must pass its own reviewed freeze before program freeze.
