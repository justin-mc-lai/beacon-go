# 返工复盘与根因防线（2026-10-22 → 2026-11-23）

> 范围：首个提交 `8223da5`（2026-10-22）到 `5a1fa87`（2026-11-23），共 205 个提交。
> 方法：git 历史、冻结包 `evidence/`、独立 QA verdict、变更记录交叉核对；只采信仓内可复跑的证据。
> 性质：本文件是分析与规则建议，不是需求真相，不构成 gate verdict。

## 0. 结论先行

1. 返工不是均匀分布。可归因的返工集中在 9 类根因，其中 6 类能用同一套尺子在一次修复内堵死。
2. 最贵的不是“写错一行”，而是**同一事实有多种表示**：同一实体在不同层用不同 key（run/feature/version/manifest、相对/绝对路径、`change_id`/`change_ids`），每修一处只堵一个口。
3. 第二贵的是**代理指标**：空产物集合、`release_confirmed`、文件存在、子串匹配、零评估 run——让“条件不满足”看起来像“满足”。
4. 第三是**散文约束**：冻结 truth 写了“不 push”，但没有可执行检查，代码全绿仍被判 FAIL。
5. 已有防线不少（gate 严格化、E2E、单壳测试、issue #10 指纹分离）；缺的是**同类扫描**、**退化输入探针**、**版本/环境单一源**的强制。

## 1. 事件簇与代价

| # | 事件簇 | 日期 | 现象 | 返工/修复轮次 | 证据 |
|---|---|---|---|---|---|
| 1 | `acceptance-seed-red-test` | 11-11 | 零产物回报绕过收割校验；随后同一路径多 kind / 空白 kind 再次绕过 | implement 3 轮（2 次返工） | `docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence.md` §返工证据 |
| 2 | `intent-contract-bridge` | 11-11 | 3 份独立 QA 判 FAIL，唯一原因是冻结约束“不 push”被外部推送打破；代码本身全绿 | 返工 2（seed 补断言）+ 三方 QA | `.../intent-contract-bridge/evidence/qa-verdict-验收员.md` |
| 3 | `kernel-recover-guard-scope` | 11-09 | recover 清空阻断快照；第二次人工确认未落库；终态事实未参与重建，done 被重报 blocked | 3 个 fix + 2 轮 status 修 | `.../kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md`、`ac-r4-status-truth.md` |
| 4 | `kernel-truth-change-cancel` | 11-11 | cancel 成功路径改写冻结变更文档，需恢复正文；`--accept-current` 单独出现未拒绝；另一条线已提前实现同一控制流 | 2 轮返工 + 1 次冻结正文恢复 | `.../kernel-truth-change-cancel/evidence/implement-cancel-reverify-20261111.md`；`83978d9` |
| 5 | `delivery-paradigm` | 11-08 | 独立验收 BLOCKED 后集中修 B1–B5：零评估 conformance pass、manual 绕过 admission、feature-latest 串味、静默建 run、`state_exported` 只认文件 | 1 轮集中返修 | `.../delivery-paradigm/evidence.md` §R1 |
| 6 | `kernel-lake-freeze-closure`（issue #10） | 11-19 | evidence 追加导致合同指纹漂移，实现期反复 refreeze；artifact 只能挂 run，形成 freeze↔run↔artifact 环 | 架构级修复 | `.../kernel-lake-freeze-closure/truth.md` §人话 |
| 7 | SDD workflow repair | 11-23 | 同日 4 个 fix：过期签名被放行、合同未 grounded、draft 分解被覆盖、coverage 别名字节影响指纹 | 4 轮 | `git log e110c4a..59fdc9c`；`docs/sdd-repair-verification.md` |
| 8 | version manifest | 11-23 | 同日 6 个 fix：host slug/旧 feature 行、inherited lake 无行、manifest lake 集、`change_ids` 复数、draft vs baseline、released line 路由 | 6 轮 | `e3dfdc8`、`901e470`、`5efb4ba`、`13d3e6d`、`0ce2c89`、`f379df5` |
| 9 | program integrated delivery | 11-20 | `program complete` 标了湖和 program 字段，未写 host feature lifecycle；节目报 delivered，feature 仍 Freeze | 1 个 fix | `33ea99a` |
| 10 | 2026-10-29 全仓审查 | 10-29 | 假验资、账本分裂、冻结真相与代码脱节、AC 账本未闭环（10+ 项） | 1 轮集中收口 | `docs/changes/change-audit-2026-10-29.md` |
| 11 | 双壳漂移 | 10-28 | Web/Desktop 两份实现各自漂移，出现假数据与语法错误 | 删除一条渲染路径 | `docs/changes/change-desktop-only.md` |
| 12 | skill 命名空间 | 10-29 | `beacon-<id>` 与 Python v1 软链冲突；软链目录不计安装数；装机二进制停在 go.9 | 1 轮修复 | `docs/changes/change-skill-namespace-2026-10-29.md` |

## 2. 成本量化

- 提交峰值：`11-09` 44 个、`11-11` 31 个、`11-23` 27 个；`11-23` 单日 13 个 `fix`。
- 单功能修复波次：version 6、SDD 4、recover 3、truth-change-cancel 2。
- 单校验返工：acceptance-seed 的同一收割校验 2 次返工，都是“修好一个入口，另两个入口照旧”。
- `docs(truth)` 23 次。其中一部分是内核要求的证据登记，**不是浪费**；可避免的是 issue #10 描述的“evidence 追加 → 指纹漂移 → 再 refreeze”，以及路径修正后重写证据索引。
- 统计口径：docs 提交多不等于返工多。本复盘只把“同一目标在短窗口内被反复修正”计为返工；一次成型的证据登记不计。
- 回滚口径：仓内没有 `git revert` 提交。回滚型事件以三种形式出现：恢复被误写的冻结正文（`83978d9`）、删掉第二条实现/第二个壳（`change-desktop-only.md`）、以及用 refreeze 把包拉回冻结态。它们和返工同源，均按根因归类。

## 3. 根因分类

### R1｜代理指标代替真实条件

**机制**：守卫检查一个“容易满足的替代信号”，而不是本次要保护的真实不变量。

**证据**：

- `len(in.Artifacts) > 0` 代替“本次回报携带的产物”，空回报整段跳过校验（acceptance-seed 第 2 轮）。
- `effectiveBlockView` 只认 `release_confirmed`，不认 `goal_completed` / `external_result_received` 等终止事实（recover-guard-scope）。
- `state_exported` 只认快照文件存在，不认 `state_snapshot_exported` 记录（delivery-paradigm B5）。
- FSM 非法边判定用方向无关的 `strings.Contains(desc, from) && strings.Contains(desc, to)`，把合法反向边误杀（kernel-qa-evidence-hardening 材料评审 B2）。
- 零评估 run 的 conformance 返回 pass（delivery-paradigm B1）。
- `qa verify-harness` 曾用常量哈希无条件返回 passed（10-29 审查）。

**代价**：同一类绕过在两个不同层面各出现一次，QA 与实现各返工一轮。

**防线**：每个守卫必须带退化探针：空集合、空白、同路径别名、重复 key、反向描述、零评估。探针名固定，缺一不算完成。

### R2｜身份键与规范化分裂

**机制**：同一实体在“校验、登记、查询、同步”四层用了不同的 key 或不同的规范化函数。

**证据**：

- 校验按 `strings.TrimSpace(kind)` 建集合，登记用未 trim 的 kind；`artifact_refs` 又按 `(run_id, abs_path)` 去重并保留首个 kind（acceptance-seed 第 3 轮）。
- 相对路径与绝对路径指向同一文件，被当成两个 kind（同上）。
- coverage 别名字节必须先归一化再算冻结指纹（`59fdc9c`）。
- version 同步依次踩中：host slug vs 旧 feature 行、store version vs manifest lake 集、project id 过滤、`change_id` vs `change_ids`、draft vs baseline、released line vs run metadata（`e3dfdc8`→`f379df5`）。
- status/出口判定读 feature 级 latest，跨 run 串味（delivery-paradigm B3）。

**代价**：version 一个特性同日 6 个 fix；acceptance-seed 同一校验 2 次返工。

**防线**：一个实体只允许一个 `canonicalize()`；validate、persist、query 共用同一结果。状态查询必须显式 scope key（run / feature / project-version / manifest-line），禁止隐式 latest。

### R3｜多写入口 / 多实现

**机制**：同一状态迁移或同一判定在多个入口各写一遍，修一个漏两个。

**证据**：

- 收割校验有 `HarvestLaneResult` / `Harvest` / `ReceiveRalphResult` / CLI lane 多个入口；首轮只修共同点，CLI lane 仍残留被拒产物（acceptance-seed 证据表）。
- `program complete` 写 program event 与湖状态，但漏写 host feature lifecycle（`33ea99a`）。
- Web/Desktop 两套壳、beacon/loom 两套品牌、状态投影三套实现（10-29 审查、`change-desktop-only.md`）。

**代价**：每多一个入口，同一 bug 的修复成本线性增加；双壳还产生假数据。

**防线**：每个状态迁移只允许一个写入口（单函数/单事务）；多入口只做参数转发，并用矩阵测试逐入口跑同一组拒绝用例。第二套实现优先删除，而不是同步。

### R4｜事实源分裂

**机制**：同一事实同时存在快照、事件流、文件、文档、安装产物等多份表示，读路径各取一份。

**证据**：

- recover 清空快照阻断字段，status 又从事件流重建，二者不一致（recover-guard-scope）。
- 合同指纹把 evidence 算进去，evidence 追加即漂移（issue #10）。
- 默认账本 `beacon.db` 与文档 `beacon-go.db` 分裂，桌面与 CLI 打开不同库（10-29 审查）。
- AGENTS.md 写 `go.15`，代码/README 是 `go.17`；装机二进制停在 go.9（本轮核对）。
- `~/.local/opt/beacon-go/beacon` 不存在，PATH 无 `beacon`（本机现状）。

**代价**：同一状态两种读法，导致“done 被重报 blocked”“绿灯读空库”“文档版本与二进制不一致”。

**防线**：每个事实只有一个权威源，其余全部派生物并声明来源；合同指纹只锁 truth/tests/tasks/restatement；版本与安装状态由命令输出/CI 校验，文档不写死。

### R5｜约束只写在散文里

**机制**：冻结 truth 的 constraints 是人类可读的句子，没有对应的可执行检查。

**证据**：

- `intent-contract-bridge` 冻结约束“不 push”，实现提交被推到远端；三方 QA 全判 FAIL，代码无需返工（`qa-verdict-验收员.md`）。
- AGENTS.md 的“commit/push/tag 先问”同样只是流程约定，无 hook 拦截。

**代价**：一整轮交付被判 FAIL，成本不在代码而在流程。

**防线**：约束分两类——可执行约束必须落成 gate/CI/hook；不可执行约束必须标注 `advisory`，不得作为 FAIL 依据。被违反过一次的约束，下一次必须升级为检查。

### R6｜冻结边界未在执行层强制

**机制**：冻结正文/变更文档的“只读”靠自觉，写路径没有拒绝。

**证据**：

- cancel 成功路径把 `changes/<id>.md` 的 `status: pending` 改写为 `cancelled`，随后需“restore frozen body after cancel unwedge”（`83978d9`）。
- 过期签名在冻结准入路径被放行（`e110c4a`）。
- evidence 写入曾改变冻结包指纹（issue #10）。

**代价**：一次正文恢复 + 一次准入修复。

**防线**：冻结工件在 API 层写保护（不是文档约定）；签名必须绑定当前内容哈希；evidence 追加不得进入合同指纹。

### R7｜单点修复，不做同类扫描

**机制**：发现一个入口/一个用例的缺陷后，只修当前点，不枚举同根因的其他调用点。

**证据**：

- acceptance-seed 第 2 轮修共同校验，CLI lane 仍可绕过；第 3 轮才引入唯一 `normalizeArtifacts`。
- version 6 个 fix 是同一根因（身份/作用域）的逐点发现。
- delivery-paradigm B1–B5 是被独立验收一次性列出后集中修的，说明内部实现阶段没有做同类扫描。

**代价**：同一根因按入口数量重复付费。

**防线**：修复必须附“同类扫描表”：调用点、入口、测试、拒绝路径四列；未扫完不算完成。共享路由/执行器/准入判断的改动，必须列全部调用者。

### R8｜验证与产物/环境脱节

**机制**：验收证据没有绑定 source commit + binary SHA + 需求指纹；装机与仓库脱节。

**证据**：

- `sdd-repair-verification.md` 明确区分“候选二进制 SHA”与“未安装”；这是正确做法。
- 但 skill 部署曾停在 go.9（`change-skill-namespace`）。
- 本机 `~/.local/opt/beacon-go` 为空、PATH 无 `beacon`。
- AGENTS.md 版本字面量落后两个版本。

**代价**：验收结论可能对应错二进制；操作者按文档执行得到不同结果。

**防线**：验收固定到 source commit + binary SHA + 需求指纹；`beacon doctor preflight` 检查二进制路径/版本/账本；CI 校验文档版本与代码版本一致。

### R9｜并发与所有权错位

**机制**：多个执行者/身份同时推进同一 run，或派工时控制流已被另一条线实现。

**证据**：

- `intent-contract-bridge` 的 run 在验收期间被外部身份推进到 release，并写入非验收者签发的 qa pass。
- `kernel-truth-change-cancel` 派单时控制流已由 `4918f1a` 落地，本轮白跑一遍核对。
- `kernel-qa-evidence-hardening` 派单收据的 truth/evidence 前缀与磁盘不符（收据过期）。

**代价**：重复实现、错误身份签字、过期收据驱动返工。

**防线**：派工前先读当前 run/分支/收据哈希；写入必须校验执行者身份与 run 租约；收据哈希不匹配直接拒绝派工。

## 4. 已经生效的防线（不要重复造）

| 防线 | 位置 | 覆盖根因 |
|---|---|---|
| 防返工与交付收口规则 | `AGENTS.md` §2026-11-20 | R3/R7/R8 |
| 合同指纹与 evidence 分离 | `2f5efef`（issue #10） | R2/R4/R6 |
| 验收种子 + 红转绿要求 | `internal/orchestrator/acceptance_seed.go`、`internal/gate` | R1/R7 |
| 单壳断言 | `desktop/shell_test.go` `TestSingleSurface` | R3 |
| 真实 E2E 链路 | `e2e/run_e2e.sh` | R1/R8 |
| gate 严格化 / integrity block | `internal/gate`、`gate-strict-integrity` | R1 |
| effect journal / change quality | `internal/effectjournal`、`internal/changequality` | R6 |
| 本机安装批准清单 | `scripts/local_release.py` | R8 |
| 文件大小 ADR + CI 拦截 | `scripts/check_file_size.sh` | 可维护性 |

## 5. 新增防线

### 5.1 项目规则（写入 AGENTS.md）

| 编号 | 规则 | 对应根因 |
|---|---|---|
| R-01 | 新增/修改守卫必须带退化探针：空、空白、别名、重复、反向、零评估；探针名固定。 | R1 |
| R-02 | 同一实体只允许一个规范化函数；validate/persist/query 共用；禁止各自 trim/abs。 | R2 |
| R-03 | 状态查询必须显式作用域键（run/feature/project-version/manifest-line）；禁止隐式 latest。 | R2/R4 |
| R-04 | 每个状态迁移只允许一个写入口；多入口只转发参数，并用矩阵测试跑同一组拒绝用例。 | R3 |
| R-05 | 每个事实只有一个权威源；派生视图必须声明来源；禁止双账本/双版本字面量。 | R4 |
| R-06 | 冻结工件在 API 层写保护；签名绑定当前内容哈希；evidence 不进合同指纹。 | R6 |
| R-07 | 修复必须附同类扫描表（调用点/入口/测试/拒绝路径）；未扫完不算完成。 | R7 |
| R-08 | 验收证据必须绑定 source commit + binary SHA + 需求指纹 + 命令输出。 | R8 |
| R-09 | 约束要么可执行检查，要么标注 advisory；被违反过的约束必须升级为检查。 | R5 |
| R-10 | 文档不写死当前版本；版本唯一源是 `internal/version/version.go` 的 `Version`，由 CI 校验。 | R4/R8 |

### 5.2 自动化（已落地）

| 编号 | 内容 | 触发点 |
|---|---|---|
| A-01 | `scripts/check_version_consistency.sh`：代码版本 vs README/README.zh-CN 声明；AGENTS.md 禁止写死当前版本 | CI `ubuntu-latest` |
| A-02 | `.github/pull_request_template.md`：根因、同类扫描、退化探针、作用域键、单写入口、证据绑定六问 | PR 创建时 |
| A-03 | 现有 CI 继续跑 `go build/vet/test/gofmt`、file-size、bench-gate | 每次 push/PR |

### 5.3 自动化（建议下一步）

| 编号 | 内容 | 对应根因 | 成本 |
|---|---|---|---|
| A-04 | `beacon doctor preflight`：校验 `beacon` 二进制路径、版本、账本路径、PATH | R8 | 小 |
| A-05 | `beacon constraint check` + pre-push hook：读冻结 truth 的约束，阻断可执行约束的违反 | R5 | 中 |
| A-06 | 守卫探针辅助器：表驱动 `guardcase`，强制六类退化输入命名 | R1/R2 | 中 |
| A-07 | 作用域 lint：扫描 gate/status 路径中的 `latest` 读取，要求显式 run 键 | R2/R4 | 中 |

### 5.4 Skill（建议）

| 编号 | 内容 | 说明 |
|---|---|---|
| S-01 | 在 `skills/qa/references/rework-guard.md` 增加“守卫退化探针 + 同类扫描表 + 证据绑定”操作手册，并从 `SKILL.md` 路由 | 不新增第 7 个技能，避免改动技能契约测试与分发面 |

## 6. 落地顺序

1. **现在（本文件同批）**：复盘文档、AGENTS.md 规则、版本一致性脚本 + CI、PR 模板。
2. **下一批**：A-04 环境预检、A-06 守卫探针辅助器、A-07 作用域 lint。
3. **再下一批**：A-05 约束检查与 pre-push、S-01 技能参考文件。
4. **门槛**：A-05 涉及 git hook 与冻结约束语义，必须先冻结 truth 再实现；不得直接改 release/gate 语义。

## 7. 度量（怎么判断有效）

| 指标 | 定义 | 目标 |
|---|---|---|
| 单功能修复波次 | 同一 feature 首次交付后 30 天内的 `fix` 提交数 | ≤1 |
| 同类根因复发 | 同一 R 编号在 60 天内再次导致返工 | 0 次 |
| 守卫探针覆盖 | 新增守卫中带六类退化探针的比例 | 100% |
| 同类扫描完成率 | 修复类 PR 附扫描表的比例 | 100% |
| 证据绑定率 | 验收证据同时含 commit + binary SHA + 指纹的比例 | 100% |
| 版本漂移 | CI 版本一致性检查失败次数 | 0 |

## 8. 边界与未验

- 本复盘基于仓内 git 与文档证据，未访问 GitHub issue 全文与 CI 历史；issue #11 的完整 checklist 以仓库内 `docs/issue11-entry-closure.md` 为准。
- “返工”按“同一目标短窗口内反复修正”计数，未把内核要求的证据登记算作返工。
- 本文件不修改任何 gate/release 语义，不替代独立 QA verdict。
- 新增自动化只做只读校验，不写账本、不改冻结包。
