# P3b 实现报告：v1 老数据迁移 + doctor 投影重建

> 日期：2026-10-22 ｜ 版本：v2.0.0-go.1（不变，P3 整体出口再发 go.2）
> 范围：冻结真相 AC-077..079（v1 三路迁移）、AC-011（doctor projection rebuild）、AC-016（v1 JSON 只许迁移、不双写）。

## 1. 做了什么（大白话）

旧版项目的数据现在能搬进新账本了：一条命令把老项目的需求包、版本、每包的进度、评审结论、证据文件指针搬进全局库，搬第二遍不会重复（按自然键判重）；搬之前能先 dry-run 只看不动。项目里已经有新版自己跑出来的数据时，默认拒绝搬，必须加 `--force` 才补缺，而且只补不改不删。

另外加了 doctor 的对账命令：把「硬盘上的文件事实」和「账本记录」逐项比对，不一致列成清单（文件有账本无 / 账本有文件无 / 状态对不上）。默认只报告；加 `--apply` 才按文件事实补登记、改包状态，每一处修正都往事件账本里留一条痕。已有的进度快照和门禁结论永远不覆盖、不删除——这是冻结 AC-011 的硬要求：重建前后，看状态的命令在进度和门禁字段上必须一致。

## 2. v1 格式探明（以源仓自身为样本实测）

| 问题 | 实测结论 |
|---|---|
| 项目注册信息存哪 | 没有独立注册表文件。项目 = 目录本身；当前版本由 `beacon/version-truth.json` 的 `docs_default_line` 声明（样本值 v1.6.19），导入时据此标 current，无声明时取最高版本并在报告里标「推导」 |
| 需求包目录结构 | `docs/beacon/<version>/features/<slug>/{truth,tests,tasks,evidence}.md`；旧版本归档在 `docs/beacon/archive/<version>/`，同构。同一版本可能两处都有（样本 v1.6.3、v2.0.0），按包数多的一处为准，另一处记入不确定项 |
| 包状态存哪 | `truth.md` frontmatter 的 `status:`（样本实测 128 个包：frozen 125 / draft 3，另有个别缺失） |
| 生命周期状态存哪 | **v1 没有生命周期账本**。最接近的是每包一份 `.machine/execution/<slug>.revision.json`（样本 233 份），含 `doc_status` / `delivery_status` / `lifecycle_status` / `created_at`。导入的生命周期是按第 3 节映射表**派生**的，每条都带 `migration_v1_derived` 标记，不冒充 v1 原始数据 |
| gate 结论有没有落盘 | 有两类：①每包评审结论 `docs/beacon/<v>/.machine/execution/<slug>.truth-review-gate.json`（样本 40 份，`pass`/`blocked`/`reason_code`，无时间戳——导入时以文件 mtime 作评估时间，属派生并注明）；②项目级报告 `.beacon/gate-report.json`、`.beacon/release-gate-report.json`（只有版本/阶段/结论，**没有包归属**），无法诚实落到某个包，记 skipped 不导 |
| runs / 轨迹有没有 | `.beacon/runs/`、`.beacon/state/` 等是 v1 运行时杂项状态，没有与冻结九表对应的结构化 run 记录，**不导**。导入的产物指针挂在一个合成的迁移 run 上（`run_kind=v1_migration`，每包一个，按 sha1 确定性生成 id） |
| AC-077/078 的账本库 | `~/.beacon/runtime-ledger/runtime-ledger.sqlite3`、`loop-runs.db` 在样本和本机上**都不存在**。导入器只做存在性探测：不存在时计数为 0 并在报告列明；存在但未探明结构时列入不确定项、明确不导，不编造格式 |
| AC-079 qa projection | `.machine/qa/*.qa9-matrix.json` 是测试计划矩阵，不是结论，不导（evidence.md 引用到它时只作为证据文件指针登记） |
| 证据引用 | 每包 `evidence.md` 是索引表，「Canonical Artifact」列给出相对项目根的路径。文件在盘上才登记指针并算 sha256；是目录或文件缺失的记 skipped |

## 3. 生命周期映射表（v1 → 冻结阶段词表）

来源优先级：`revision.json` 的 `lifecycle_status` → `doc_status` / frontmatter `status` 兜底。映射不了的一律 skipped，不硬塞。

| v1 值 | 冻结 stage | 冻结 status | 样本量 |
|---|---|---|---|
| lifecycle_status = accepted | Release | Completed | 150 |
| lifecycle_status = verified | Qa | Completed | 5 |
| lifecycle_status = implemented | Implement | Completed | 7 |
| lifecycle_status = frozen | Freeze | Completed | 1 |
| lifecycle_status = blocked（doc 已 frozen） | Implement | Blocked | 3 之一部分 |
| lifecycle_status = blocked（doc 未 frozen） | Freeze | Blocked | 同上 |
| lifecycle_status = idle | Think | NotStarted | 65 |
| 无 lifecycle_status，doc/frontmatter = frozen | Freeze | Completed | 兜底 |
| 无 lifecycle_status，doc/frontmatter = draft | Freeze | InProgress | 兜底 |
| 其他任何值 / 无任何来源 | — | — | skipped + 原因 |

## 4. 判重键与安全口径

| 类别 | 判重键 |
|---|---|
| projects | `root_path`（解析后绝对路径，复用 P1 语义） |
| project_versions | (project, version 字符串) |
| feature_packages | (version, slug)，slug 以 frontmatter 为准、与目录名不一致时记不确定项 |
| feature_lifecycle | 快照已存在即不再写（永不覆盖） |
| gate_results | (feature, gate_type=`truth_review`, evaluated_at=文件 mtime, verdict) |
| artifact_refs | (迁移 run, abs_path)；迁移 run id = sha1(project\|version\|slug) 确定性生成 |

安全：导入器对 v1 目录全程只读（测试用整目录哈希证明一字节未改）；写库只走现有 DAO；项目已有 v2 数据（存在非迁移 run，或存在非迁移派生的生命周期快照）时默认拒绝并返回 `v1_migration_project_has_v2_data`（报告照常输出）；`--force` 只补缺失项，不覆盖、不删除。本片没有给冻结九表加任何结构。

## 5. 实跑样例（真实输出）

构造样本（1 项目 / 2 版本 / 3 包：frozen+accepted、draft、状态不可映射各一；证据引用含目录与缺失各一；项目级 gate 报告一份）：

```
projects          imported=1 already=0 skipped=0
project_versions  imported=2 already=0 skipped=0
feature_packages  imported=3 already=0 skipped=0
feature_lifecycle imported=2 already=0 skipped=1   （in-review 不可映射）
gate_results      imported=1 already=0 skipped=1   （项目级报告无包归属）
artifact_refs     imported=2 already=0 skipped=2   （目录引用 + 文件缺失）
```

源仓自身（独立临时库 `BEACON_STORE_PATH=/tmp/p3b-smoke/beacon.db`，未污染默认库）：

- dry-run：versions 24、packages 128、lifecycle imported=127 / skipped=1、gates imported=40 / skipped=2、artifacts imported=511 / skipped=396、两个账本库 present=false
- 唯一 lifecycle skipped：`v1.6.9/goal-main-bypass-admit-gate`（无 revision 记录、无包状态——如实跳过）
- 真导入数字与 dry-run 逐项一致；current 版本正确标为 v1.6.19（version-truth 声明）
- 第二次真导入：六类 imported 全 0，already_present 与第一次的 imported 逐项相等（128 包 / 40 gate / 511 指针），行数不翻倍
- 导入后 `doctor rebuild`（只报告）：全仓仅 1 条 drift——上面那个无源可映射的包 `missing_in_store`（lifecycle 缺失且不可派生），其余 128 包文件与账本一致
- 导入后 `spine list` 可读回 128 个快照

测试：新增 7 个（migration 包 6：计数+幂等、dry-run 不写库、v2 数据拒绝+force 补导、映射表、doctor 只报告、doctor apply；CLI 1：migrate/doctor/spine 端到端）。全量 135 个测试运行全过，既有 128 个零改动、零回归。

## 6. doctor rebuild 口径（AC-011）

- 比对键：(version, slug)。三类 drift：`missing_in_store`（文件有账本无，含包缺失与 lifecycle 快照缺失两种 detail）、`missing_on_disk`（账本有文件无，只报告）、`status_mismatch`（包 status 不一致；或 lifecycle 与文件派生值不一致——后者只报告）。
- `--apply` 动作：注册缺失包（+派生 lifecycle）、补缺失 lifecycle 快照、修正包 status；每项修正追加一条 `lifecycle_events`（`doctor_rebuild_registered` / `doctor_rebuild_corrected` / `doctor_rebuild_lifecycle_filled`）。
- 绝不做：覆盖已有 lifecycle 快照、改动 gate_results、删除任何账本行。测试专门验证：rebuild 前后同一包的 spine 快照在 lifecycle/gate 字段上完全一致（AC-011 一致性条款）。

## 7. v1.6.3 九个免疫修复 + v1.6.4–1.6.6 能力对账（只盘点，不硬塞）

九个免疫修复 = v1.6.3 的九个 `-v163` 修复湖（`docs/beacon/archive/v1.6.3/features/`，均为 frozen；同版本另有 `beacon-governance-truth-canonical-schema` 不带 v163 后缀，不计入九个）。

| # | v1.6.3 修复湖 | 修的是什么 | Go 版现状 |
|---|---|---|---|
| 1 | ecosystem-discovery-sync-drift | 管线发现不了新包、同步不对、freeze 后不重物化 | 部分覆盖：发现/同步由 migrate + doctor rebuild 承担；「物化」面未迁（P4 候选） |
| 2 | freeze-integrity-drift-repair | 真相完整性校验器自身的 4 处漂移 | 已覆盖：P3a 已按 AC-101 在 Go 侧重写 Truth Integrity Gate 并带断裂测试；v1 的 4 处是 Python 校验器内部 bug，不逐项对应 |
| 3 | gate-truth-model-alignment | release gate 对所有版本硬套包闭合要求 | 已覆盖：P3a release 门只查四门结论 + 人工确认，不硬套包闭合 |
| 4 | post-release-hook-agent-coverage | post-release hook 的 agent 覆盖硬编码 | 未覆盖：hooks 在口径稿中只留最小集，agent 覆盖循环未实现（P4 候选） |
| 5 | runtime-release-alignment | 发布后 runtime 与 docs 版本撕裂 | 部分覆盖：Go 单二进制消解了撕裂源头；版本面一致性只落了 self-heal/doctor 最小集 |
| 6 | spec-fingerprint-package-auth-paths | 规格指纹对新包路径残留 legacy 扫描 | 未覆盖：Go 侧尚无 spec fingerprint 面（P4 候选） |
| 7 | truth-format-package-auth-skip | legacy 格式校验器误套 package-authoritative 包 | 已覆盖：Go 的 integrity 按包权威口径实现，不存在 legacy 校验器误套路径 |
| 8 | version-resolve-fail-closed | 版本解析绕过集中解析、无漂移 fail-closed | 部分覆盖：解析已集中（store 路径/版本解析一处）；runtime↔docs 漂移守卫随 doctor 最小集部分落地，完整版列 P4 |
| 9 | workflow-memory-completion-gate | completion truth 缺失时软放行 | 已覆盖（结构性）：P2a Complete 四项齐全才放行且代码根本不读 memory；P2d 有专门测试——写满「已完成」的轨迹也完不了工 |

v1.6.4–1.6.6 能力变更对账：

| 版本 | 能力项 | Go 版现状 |
|---|---|---|
| v1.6.4 | program-next-action-admission | 已覆盖（P2c admission + P2d program route/admission） |
| v1.6.4 | qa-business-state-machine | 部分覆盖（阶段状态机 + qa 门有，业务状态机细则未迁） |
| v1.6.4 | qa-chrome-extension-e2e | 未覆盖（执行器/宿主侧，P3c/P4） |
| v1.6.4 | qa-coverage-confidence-chain | 部分覆盖（P3a qa 门判证据齐全 + 哈希存活，不算覆盖置信） |
| v1.6.4 | qa-defect-repair-loop | 部分覆盖（P2a repair 计数上限 + error_recovery 有骨架，无缺陷单闭环） |
| v1.6.4 | qa-methodology-contract | 未覆盖（合同面，P4 候选） |
| v1.6.5 | loop-failure-mode-catalog | 部分覆盖（P2b 统一 reason code 词表有，失败模式目录面无） |
| v1.6.5 | loop-pattern-registry | 未覆盖（P4 候选） |
| v1.6.5 | loop-readiness-audit | 部分覆盖（P2b self-heal 开跑前检查） |
| v1.6.5 | loop-token-budget | 部分覆盖（G-21 预算护栏为轮次+时长；token 维度无真实来源，未做且已在 P2b 注明） |
| v1.6.5 | protected-path-denylist | 未覆盖（与 worktree 真集成相关，P3c 评估，P4 候选） |
| v1.6.5 | workspace-pool-lease | 部分覆盖（P2b driver 租约已落到 run 级；工位池留 P3c） |
| v1.6.6 | agent-structured-trace | 已覆盖（G-17 trace 以 lifecycle_events 为源，P2a） |
| v1.6.6 | harness-remediation-sandbox | 未覆盖（P4 候选） |
| v1.6.6 | loop-goal 本体 | 已覆盖（P2 全量 G-01..G-22） |
| v1.6.6 | loop-stagnation-detector | 已覆盖（P2b supervisor 心跳超时判 stall；更细的停滞信号未迁） |

本片没有顺手补任何「小改」：对账中未覆盖项要么碰执行器/worktree（P3c 范围），要么是新增面（受口径稿防长胖条款约束，须先声明替代谁），硬塞都会越界，全部列 P4 候选。

## 8. 语义差异 / 简化清单

1. 生命周期是**派生**不是搬运：v1 没有生命周期账本（见第 2 节），映射表是本片定义的口径，每条派生快照带 `migration_v1_derived` 标记，与真实 v2 运行数据可区分。
2. gate 只导每包评审门（`truth_review`，保留 v1 名，不是 P3a 五门之一）；评估时间用文件 mtime（源文件无时间戳）。项目级 gate 报告不导（无包归属）。
3. AC-077/078 账本库在样本中不存在，只做存在性探测与诚实报告，未写解析器；将来真出现这类库，先探明结构再补导入（列 P4 候选）。
4. runs 不导：合成迁移 run 只为挂产物指针（一包一个、确定性 id、状态 Succeeded），不冒充 v1 运行历史。
5. 同版本双根（顶层 + archive）按包数取舍，落选根记不确定项；evidence.md 里个别单元格是分号拼接的复合路径，按单路径处理后文件不存在、记 skipped（如实呈现在 skipped 清单里，不做猜测拆分）。
6. doctor 的 lifecycle 派生与 migration 共用同一张映射表，不存在第二套口径。

## 9. 交付物

| 文件 | 行数 / 大小 |
|---|---|
| internal/migration/scan.go（v1 磁盘扫描器） | 468 行 |
| internal/migration/migrate.go（导入主流程+报告） | 296 行 |
| internal/migration/migrate_items.go（映射表+逐类导入） | 297 行 |
| internal/migration/doctor.go（rebuild 对账+apply） | 236 行 |
| internal/migration/migration_test.go | 470 行 |
| internal/cli/migrate.go / doctor.go / migrate_test.go | 81 / 87 / 100 行 |
| internal/store/dao_p3b.go（ListFeatures/CountRunsByKind/UpdatePackageStatus） | 65 行 |
| root.go 注册 migrate/doctor 两组命令 | 改 2 行 |
| bin/beacon | 15,334,991 字节 |

验证：`go build ./...` / `go vet ./...` / `gofmt -l` 全净；`go test ./... -count=1` 全过（135 个测试运行，0 失败；既有 128 个零改动）。冻结 schema 九表结构零改动，Python 源仓未动（导入器对 v1 目录全程只读，测试以整目录哈希验证一字节未改）。
