# AGENTS.md · beacon-go

给在这个仓里工作的 Bot 和 agent。冲突时以本文件为准。

## 唯一真源（最高口径）

以下九项是 beacon-go 的全部产品真源。任何文档、代码、skill、测试与之冲突时，以本节为准，冲突方视为待清理的历史漂移。历史冻结工件已全部移除，一切从新按本节开始。

1. **需求树（intent tree）是唯一需求真相**。canonical JSON 为权威源，SQLite 只是可重建索引；叶子带可执行验证，冻结指纹只覆盖合同面。
2. **七能力/命令组**：`plan | truth | change | implement | verify | release | goal`。唯一一等能力集合，由 `internal/harness/harness.go` 的 `Abilities` 单点定义，CLI 从该表装配，回归测试锁死集合与顺序。旧六能力（loop/design/qa 为一等）及改名均为历史。
3. **worktree 是核心能力**，用于并行 worktree 交付；在 `implement` 组可见，admission 硬门禁，不得按"移出核心"处理。
4. **change 只有一个入口**：顶层 `change`（`request`/`list` 操作需求树；`legacy` 承载旧 feature-package 变更）。`truth change` 不再是独立命令。
5. **design 已从系统彻底移除**——不是能力、不是 goal 阶段、不是 gate 类型、不是 CLI flag、不是 admission 检查。产品基因：**复杂应该留给我们系统，不要留给用户，用户体验至高无上。** 任何让调用者手动声明"这是 UX 工作"、手动绑定设计原语、或暴露 design 阶段/命令/gate 的实现，都是违背这条基因的漂移。
6. **版本号是纯 `x.y.z`**，演进迭代；唯一源 `internal/version/version.go`，规则见 `docs/version-scheme.md`。当前版本以 `beacon --version` 为准，本文不写死。版本轴从 `0.0.1` 重新开始（首个正式版），历史版本线随旧工件一起清除。
7. **需求交付验证**分层：intent leaf/rollup → verify 证据登记 → harness 只读 hash 校验 → gate/release 评估；不混同 "hash verified / gate pass / tree verified / version ready"。
8. **内核不调 LLM、不跑 agent**；执行归宿主 agent，用 `goal tick --by` 记录。
9. **边界权威顺序**：本节 > spec #15 > `docs/adr/ADR-0002-intent-tree-truth-model.md` > 其余文档（历史范围文档冲突段已作废）；术语见 `GLOSSARY.md`。

## 目录与入口

- 引擎：`cmd/beacon`，核心包在 `internal/`。
- 需求真相：意图树，由 `plan`/`truth` 命令在 `docs/beacon/<version>/tree/` 生成（canonical JSON 为权威源）。历史冻结工件不随仓库维护。
- 账本：Go 内核用独立账本，避免与 Python v1 schema 冲突：

```bash
export BEACON_STORE_PATH="$HOME/.beacon/beacon-go.db"
```

- 需求正文永远在 Markdown，不进数据库。
- 外部协作约定（issue/triage/领域术语）：`docs/agents/`、`GLOSSARY.md`、`docs/adr/`。

## 授权边界

工作目录已授权，可直接创建/修改仓库内文件（`docs/`、`internal/`、`cmd/`、`e2e/`），无需逐文件申请。以下动作必须先说明并等用户确认：

- `git commit`、`git push`、`git tag`
- 改已冻结的 truth 正文（走唯一入口 `beacon change request`（树）或 `beacon change legacy`（旧包）→ refreeze）
- 改 release 语义、gate 语义、受保护路径（`.env`、密钥、支付、迁移）
- 发布

## 验证命令

改完必须跑，贴真实输出：

```bash
go build ./...
go test ./... -count=1
go vet ./...
bash e2e/run_e2e.sh        # 涉及端到端链路时
bash scripts/check_version_consistency.sh
bash scripts/check_kernel_deps.sh
```

测试没过不许说完成；不许用"应该可以"代替运行结果。未验证写"未验证"，不确定写"未确认"。

## 工程纪律

- 最小改动，遵守现有结构与风格；改了哪些文件、为什么、测试输出、未覆盖边界，四项一起交。
- 同一实体只允许一个规范化入口，validate/persist/query 共用同一结果；状态查询必须显式作用域键，禁止隐式 latest；每个状态迁移只允许一个写入口，多入口只转发参数。
- 每个事实只有一个权威源，派生物声明来源；禁止双账本、双版本字面量、双渲染路径。冻结工件在 API 层写保护，签名绑定当前内容哈希，evidence 追加不进合同指纹。
- 守卫必带退化探针：空集合、空白值、相对/绝对别名、重复 key、反向描述、零评估。修缺陷附同类扫描表（调用点/入口/测试/拒绝路径），未扫完不算完成。
- 测试门或 CLI 合同变化时，同批更新真实入口 fixture；必须有新链路成功与拒绝的实跑，基线既有红灯不得当作新功能已验收。真并发用两个执行者同步屏障证明，不以两个 active 状态或时间戳代替。
- 验收固定到 source commit + binary SHA + 需求指纹；证据、批准、安装同物，窄口径 PASS 不扩大为全量发布结论。
- 本机安装只经仓内 `scripts/local_release.py` 的批准清单入口；观察/维护脚本只报告，禁止直接构建覆盖 `~/.local/opt/beacon-go/beacon`。
- 约束分两类：可执行检查，或显式 `advisory`；被违反过一次的约束下次必须升级为检查。
- 同根因两次返工、两次断线或两次上下文错位，停止叠加任务，保存 checkpoint 后换方式；15 分钟无实证进展主动检查，30 分钟主动说明阻塞。到发布门后给用户确切清单，提交/推送/发布仍遵守上面的授权边界。
