# 标准流 S1 实施报告：分支与基线纪律（AC-136/137/139/140）

版本保持 v2.0.0-go.6（三片全完再统一升版）。冻结原文已逐字核对：
`~/workspace/repos/beacon/docs/beacon/archive/v2.0.0/features/beacon-v2-rust-kernel-redesign/truth.md`
的 R10 Truth Canonical Branch Contract（约 245–250 行）与 AC-136..140（671–677 行）、
tests.md 的 TC-136/137/139/140。源仓只读，未动冻结 schema，运行状态只进 store。

## 结论先行

四条 AC 全部落地，行为级真 git 测试覆盖。**五个 reason_code 全部是冻结原文逐字码，
本片没有新登记任何码**（见第 6 节码表出处）——任务里预备的「现场可能需要新登记
truth_baseline_uncommitted 类码」没有发生：TC-139 本来就指定了
`canonical_refreeze_not_committed`。全量 221 个测试全过（210 既有 + 11 新增），
既有测试只改了 1 处（声明见第 7 节），E2E 加了 1 步后原样全链 PASS（声明见第 7 节）。

## 1. 逐 AC 落地表

| AC | 冻结口径 | 实现落点 | 码 | 测试 |
|---|---|---|---|---|
| AC-136 分支纪律 | truth 写入/冻结只能在 canonical 分支（truth_canonical_branch，默认 main、缺失时 master） | `orchestrator/branch.go`：`requireCanonicalBranch` 守卫 truth create/freeze/change/refreeze/baseline 五个写动作入口；detached HEAD 也算 mismatch（fail-closed）；读动作不受限 | `truth_canonical_branch_mismatch`（R10 原文） | TestS1BranchGuardRejectsNonCanonical、TestS1BranchDefaultsAndOverride、TestS1BranchConfigOverride、TestS1TruthBaselineCLI |
| AC-139 已提交基线 | refreeze 完成后必须提交到 canonical 才成为可消费基线；未提交给 canonical_refreeze_not_committed | FrozenFact 增基线维度（committed/uncommitted/not_a_git_repo/no_document）；freeze 事件落 truth_sha256_after + baseline_status（已提交时同事件落 baseline_commit）；新子命令 `truth baseline` 显式补记（幂等）；消费侧收紧：admission 第 1 项 + truth 出口门 | `canonical_refreeze_not_committed`（TC-139 原文） | TestS1BaselineLifecycle、TestS1TruthExitGateBaselineLeg、TestS1TruthBaselineCLI |
| AC-137 refreeze 交易 | reason-only / machine-only 的 refreeze 拒绝；narrative 与 machine projection 漂移拒绝 | `RefreezeTruthPackage` 在 pending 检查后、freeze 预检前跑 `refreezeTxGuard` 三路检查（见第 4 节） | `empty_refreeze_reason_only`、`revision_authority_drift`（R10 原文） | TestS1RefreezeEmptyRefused、TestS1RefreezeRevisionDrift、TestS1RefreezeChainDrift |
| AC-140 工位 stale 隔离 | 工位内 stale 的本地 refreeze 必须隔离，不许作为需求依据上行 | `orchestrator/worktree_quarantine.go`：worktree status/create 扫描 + 记录（事件 + run metadata），merge 前新鲜重扫、命中即拒绝合并（标记式隔离，取舍见第 5 节） | `stale_feature_local_refreeze`（TC-140 原文） | TestS1WorktreeQuarantineBlocksMerge |

## 2. canonical 配置口径

canonical 分支解析序（`resolveCanonicalBranch`）：

1. **项目配置覆盖**：项目根 `.beacon/config` 的 `truth_canonical_branch` 键
   （极简行格式 `truth_canonical_branch = dev`，`#` 注释）。
   落点论证：Go 内核没有 v1 的治理合同 store，store 九表 schema 已冻结、Project
   注册信息没有配置列；`.beacon/` 是内核既有的项目本地运行时目录
   （`.beacon/loop/pause-all`、`.beacon/artifacts/` 有先例），放这里不新增表、
   不污染需求文档 frontmatter（frontmatter 是需求真相，不是运行配置）。
2. 仓库存在 `main` → main；否则存在 `master` → master；
3. 都不存在 → `main`（冻结默认）。

非 git 项目口径（本片拍板）：不受分支纪律约束，但**不许静默跳过**——
truth show/freeze 输出与 FrozenFact 都明示 `branch_check=not_a_git_repo`。

## 3. 基线消费口径图（git / 非 git 两路）

```
包 frozen（frontmatter 判定，与 P4b 一致，两路相同）
  │
  ├─ 非 git 项目：frontmatter 即权威（P4b 口径不变）
  │    baseline_status = not_a_git_repo → admission / truth 出口门照常放行
  │
  └─ git 项目：多一维基线事实（AC-139）
       baseline 来源（读侧零 git 成本优先）：
         ① trail 里有基线记录（freeze 自动落 / truth baseline 补记）：
            当前正文 sha256 == 记录的 truth_sha256 → committed（带 commit 号）
            对不上 → uncommitted（正文在基线之后又动过）
         ② 无记录才 live probe：truth 文件 porcelain 干净 +
            canonical 分支上的版本 status=frozen + 与工作区字节一致
            → committed（commit = git log -1 该文件）；任一不满足 → uncommitted
       只有「磁盘上真有 truth 文件」的包才探针；无文件包 = no_document，
       不拦（保护既有种子型测试场景，如 p3c 的 gitEnv 推进流）。
       消费：baseline=uncommitted → admission 第 1 项与 truth 出口门
       以 canonical_refreeze_not_committed 拦截；committed → 放行。
```

freeze 的基线时序：freeze 是文档动作，冻结本身永远成功；翻牌瞬间正文通常
还没提交，freeze 事件记 `baseline_status=uncommitted`；作者提交后跑
`truth baseline` 落账（或下一次幂等 freeze 路过时自动补记）。refreeze 同理：
refreeze 事件若无已提交基线，会清掉旧基线记录（旧 commit 描述的已不是当前正文）。

## 4. AC-137 三路检查的落法

`refreezeTxGuard` 在 `RefreezeTruthPackage` 内、freeze 预检之前执行，全 fail-closed：

1. **空 refreeze**：freeze/refreeze 事件新增记录四件套指纹
   `surface_fingerprint`（core 新增 `PackageSurfaceFingerprint`，算法标签
   `pkgsurface-v1`；truth 成员沿用 P4c 的 status 归一化——冻结翻牌本身不算
   内容变化）。当前四件套指纹与最近 freeze 记录相等 → 没有任何需求面变动，
   这次 refreeze 只剩 reason / machine 面 → `empty_refreeze_reason_only` 拒绝。
   （pre-S1 的旧 freeze 记录只有 truth+tests 指纹时退回该对比较。）
2. **revision id 一致性**：truth frontmatter 的 `revision_id`（或 `revision`）
   与最近 freeze 事件记录的 revision 不一致 → `revision_authority_drift`。
   两边任一没写则不判（骨架包默认无此字段，不误伤）。
3. **narrative/machine 漂移（指纹链落法）**：v1 比对的是三方 revision 表示；
   Go 内核没有那套表示，等价证据链是 P4c 指纹：freeze 时的
   `package_fingerprint` → change 打开时记录的 `package_fingerprint`
   （本片给 change 事件新增）。两者不一致 = 正文在「冻结基线」与「变更登记」
   之间已经动过，即编辑绕开了变更流先行发生 → `revision_authority_drift`。
   全程只用 P4c/S1 指纹一族作证据，没有第二套哈希。

## 5. AC-140 取舍论证：标记式隔离（不自动 stash）

冻结原文（truth.md R10 第 4 条）：「worktree 中的 stale refreeze 必须被隔离
（quarantine/stash **或等价隔离**），不允许直接作为上层需求依据」——原文自带
「或等价隔离」出口。本实现取标记式隔离：

- **不自动 stash 的理由**：内核自动动用户工位文件（stash/移动）超出安全边界——
  stash 可能搁浅作者在途编辑的唯一副本、与作者编辑器竞态；且 Go 内核至今
  所有工位写动作都是用户显式命令触发的（create/merge/cleanup），没有后台
  改文件的先例。
- **标记式的隔离效力**：扫描在 worktree status / create 时跑，新发现写
  `worktree_truth_quarantined` 事件 + run metadata 的 quarantines 记录
  （含文件 sha256、frontmatter 状态、隔离原因），status 输出直接列出；
  **merge 前对来源工位做新鲜重扫，命中即以 stale_feature_local_refreeze
  拒绝合并**——stale 正文经内核永远上不了主工位，这就是「不许作为需求依据」
  的执行点。旧记录不单独拦合并（以新鲜扫描为准：作者把工位副本修回与基线
  一致后合并可正常进行，测试已覆盖此修复路径）。
- 与原文的差异如实标注：原文举例含 stash 语义，本实现是其「等价隔离」
  分支——隔离对象是「该工位产物的可信性」而非「把文件搬走」。

附带一处行为变化：`goal worktree status` 自本片起在**有新发现时才写库**
（无新发现零写入），CLI 该命令因此从只读引擎改走写引擎，Short 文案已同步。

## 6. 码表出处（逐码标注）

| 码 | 出处 |
|---|---|
| `truth_canonical_branch_mismatch` | 冻结原文 R10：「current branch != truth_canonical_branch → truth_canonical_branch_mismatch」 |
| `canonical_refreeze_not_committed` | 冻结原文 TC-139：「baseline refreeze 未提交时 admission 返回 canonical_refreeze_not_committed」 |
| `empty_refreeze_reason_only` | 冻结原文 R10：「empty refreeze (reason-only / machine-only) → empty_refreeze_reason_only」 |
| `revision_authority_drift` | 冻结原文 R10：「narrative truth 与 machine projection / revision record 不一致 → revision_authority_drift」 |
| `stale_feature_local_refreeze` | 冻结原文 TC-140：「merge quarantine 残留时返回 stale_feature_local_refreeze」 |

五码全部冻结原文逐字，本片**零新登记**。词表登记动作仅为把五码加进
`core/reasons.go` 的 S1 家族常量与 `IsKnownReasonCode` 前缀集
（truth_canonical_ / canonical_ / empty_ / revision_ / stale_）。

## 7. 既有测试与 E2E 调整声明（各一处，均为纪律生效的必然结果）

1. **既有测试改 1 处**：`internal/cli/p4a_test.go` 的 TestTruthCLIChain——
   原剧本 change 后不改任何内容直接 refreeze，那正是 AC-137 要拒的空
   refreeze。已按变更摘要给 truth.md 的 AC-001 正文补一处真实修订再
   refreeze（正常路径语义不变）。除此之外 210 个既有测试零改动、零回归。
2. **E2E 改 1 处**：`e2e/run_e2e.sh` 新增 STEP 4b。原因实测在案：脚本的
   项目是 git 仓库（main），原剧本 freeze 后不提交，STEP 7 admit 被新基线
   纪律正确拦下（`goal admit: implement admission blocked …
   (canonical_refreeze_not_committed)`，exit 1）。4b 先断言 freeze 输出
   `baseline_status=uncommitted`，再 `git add docs` 提交（即标准流的
   canonical commit 步），跑 `truth baseline` 并断言 committed + recorded；
   其余步骤一字未动。脚本头部链路注释已同步加 4b 说明。
   另注意：脚本用预编译的 `bin/beacon`（`BEACON_BIN` 默认值），本片已按
   现源码重编该二进制（16,219,026 B）。

## 8. 实跑样例（真 git 仓库，bin/beacon 文本输出摘录）

```
--- 1) freeze on dev (AC-136 refuses)
Error: truth.Freeze: truth writes are canonical-only (AC-136): current branch dev
  != truth_canonical_branch main; run `git checkout main` first
  (truth_canonical_branch_mismatch)
--- 2) freeze on main
package:  login-2fa (v1.0.0)
status:   frozen
branch:   check=ok current=main canonical=main
baseline: uncommitted
--- 3) baseline before commit (AC-139 refuses)
Error: truth.Baseline: frozen truth is not a committed baseline yet: truth file
  has uncommitted changes; commit the package on main, then re-run
  `beacon truth baseline` (AC-139) (canonical_refreeze_not_committed)
--- 4) commit + baseline
baseline: committed commit=37a31b863af14e53c349fb4be68b0195ab0ba975 (recorded)
--- 5) change + empty refreeze (AC-137 refuses)
Error: truth.Refreeze: no requirement surface (truth/tests/tasks/evidence)
  changed since the last freeze; a refreeze that only restates the reason or
  the machine projection is refused (AC-137) (empty_refreeze_reason_only)
```

基线全周期（未提交拦 admission → 提交落账转绿 → 正文再改未提交转红）、
refreeze 三路拒绝、工位隔离标 + merge 拒绝 + 修复后放行，均有自动化测试
逐步断言（stdflow_s1_test.go），不在此重复贴日志。

## 9. 验证结果与交付物

- `go build ./...`、`go vet ./...`、`gofmt -l` 全清；
  `go test ./... -count=1` **221 个测试全过**（210 既有零回归 + 新增 11：
  orchestrator 10 + cli 1），含真 git 临时仓库行为测试。
- E2E：`bash e2e/run_e2e.sh` **exit 0**，全链 PASS（13 步，含新 STEP 4b）。
- 二进制：`bin/beacon` 16,219,026 B（v2.0.0-go.6 不变）。
- 新增/主要文件：`internal/orchestrator/branch.go`（371 行，分支纪律+基线
  机器+trail 记录扫描）、`internal/orchestrator/worktree_quarantine.go`
  （173 行）、`internal/orchestrator/stdflow_s1_test.go`（457 行）、
  `internal/cli/stdflow_s1_test.go`（105 行）；改动面：core/reasons.go、
  core/fingerprint.go、harness/git.go（+4 个只读 git 帮手）、frozen_fact.go、
  truth.go、truth_change.go、admission.go、exitgates.go、meta_p3c.go、
  worktree.go、cli/truth.go（+baseline 子命令与输出行）、cli/goal_p3c.go。
  全部源文件 < 500 行。

## 10. 已知边界（留给 S2/S3 的衔接点）

- AC-141 的 `truth_baseline_not_merged` / `truth_baseline_commit_missing`
  （develop 线基线合入 main 的检查）不在本片，按计划归 S2；本片的
  probeBaseline 已具备「canonical 分支上文件状态」的判定件，S2 可直接复用。
- 基线 commit 的语义是「canonical 分支上最后触碰 truth 文件的提交」
  （`git log -1 -- <path>`），不是「包含冻结翻牌的那一提交」的逐字考据——
  两者在正常流（冻结后提交）下重合；若作者把多次修改攒成一次提交，
  记录的仍是该文件在 canonical 上的最新提交，与正文 sha 绑定校验一致。
- quarantine 扫描只看 frontmatter 声明 frozen 的工位副本；工位里把正文
  改烂但保持 draft 的副本是普通在途编辑，不在本片范围（由 merge 冲突与
  内容评审兜底）。
