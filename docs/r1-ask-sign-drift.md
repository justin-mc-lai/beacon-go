# R1：先问清楚 → 再签字 → 再施工 → 最后点头交付（v2.0.0-go.8）

> 日期：2026-10-23 ｜ 版本：v2.0.0-go.8 ｜ 范围：把 Python v1.6 的
> plan / truth review / 意图门能力落进 Go 主链，并补 R0 地基与 R1 真相自检。
> 写法：大白话；必要术语第一次出现时一句话解释。

---

## 1. 这次到底补了什么

| 环节 | Python v1.6 有什么 | Go 之前 | Go 现在 |
|---|---|---|---|
| 先问清楚 | `beacon-plan` 的 intent_snapshot / scope_mode / deferral_ledger；brainstorm 的苏格拉底提问 | 没有 plan 面 | `truth ask / answer / questions` 一问一答账本 |
| 再签字 | `truth review` 的 A/B/C/D（意图覆盖、FSM、AC→TC、填充深度） | 只查四件套非空 | `truth intent / confirm` + 冻结时的意图门 |
| 再施工 | implement admission / worktree / lanes | 有 | 未动，继续复用 |
| 最后点头 | release human gate | 有 | 未动，继续复用 |
| 真相自检 | 包级 fingerprint | 只有包级 | 需求级 fingerprint + `truth drift` + 并行变更基线守卫 |

一句话：**以前 Go 版是“你写我冻”，现在冻之前必须先问、必须签字；冻之后能指出是哪条 AC 动了。**

---

## 2. CLI 流程（真实命令）

```sh
beacon truth create --project . --slug my-feature

# 先问：一次只加一个问题；answer 关掉它
beacon truth ask    --feature my-feature --question "谁用？什么算完成？" --recommend "..."
beacon truth answer --feature my-feature --id Q1 --answer "..."

# 签字：记录原话 / 目标 / 非目标 / 约束 / 范围模式
beacon truth intent --feature my-feature \
  --utterance "用户原话" --goal "要的结果" \
  --non-goals "明确不做" --constraints "硬约束" --scope-mode focused

# 签字会检查：开放问题必须清零、每条 AC 必须有意图来源
beacon truth confirm --feature my-feature

# 冻结：四件套 + 一致性 + 意图门全过才放行
beacon truth freeze --feature my-feature
```

变更后：

```sh
beacon truth change   --feature my-feature --summary "..."
# 编辑需求包
beacon truth refreeze --feature my-feature
beacon truth drift    --feature my-feature   # 输出 added / changed / removed 的 AC
```

并行变更：

```sh
# 如果另一个 change 先落地，refreeze 会被 truth_base_diverged 拦住
beacon truth rebase --feature my-feature --change <id> --accept-current --note "已合并 A"
beacon truth refreeze --feature my-feature
```

---

## 3. 意图门规则（确定性，不靠提示词）

冻结前逐条检查，任一不过即 blocked：

1. `intent_confirmed: true` —— 必须显式签字。
2. `## User Intent Snapshot` 的 `utterance / goal / non_goals / constraints / scope_mode` 均非空、非占位。
3. `## Open Questions` 没有 `open` 行；`answered / dropped / closed / resolved` 才算关掉。
4. `## Intent Coverage` 覆盖 truth 定义的每一条 AC，且 source/evidence 不是占位。
5. `domain_required: true` 时必须有 `## Domain FSM`，五列齐全，并至少写一个挂起态与一个非法转移。
6. `package_maturity` 不能是 `scaffold`；`## 人话` 不能还是骨架占位。
7. `scope_mode: full_parity` 时必须有 `## Parity Matrix`。

另补 Python C_exec：TC 的 Command 不能只是 `rg/grep/cat/ls/find/echo` 这类
“查文件”命令，业务 AC 必须有执行层 TC（`truth_integrity_docs_only_tc`）。

代码落点：

- 纯判定：`internal/gate/intent.go`
- 冻结接线：`internal/orchestrator/truth.go` 的 `freezePrecheck`
- 文档改写：`internal/orchestrator/intent.go`（只改目标章节，其余字节保留）

---

## 4. 真相自检规则

### 4.1 需求级 fingerprint

冻结事件新增：

- `requirement_fingerprints`: `AC-001 -> sha256(...)`，每条 AC 一个哈希
- `requirement_fingerprint_alg`: `reqfp-v1`

`beacon truth drift` 对比当前包与最近一次冻结记录，输出：

- `body_drift`：非 AC 正文也变了
- `added / removed / changed`：哪条 AC 增删改
- `clean`：全部干净才算 true
- `base_diverged`：pending change 的作者基线是否落后于最新冻结

### 4.2 并行变更不静默覆盖

- `truth change` 记录 `base_truth_sha256`（作者动手时看到的冻结正文）。
- `truth refreeze` 前比对：最新冻结正文 ≠ 该 change 的 base → `truth_base_diverged`，拒绝。
- 人工确认合并后，`truth rebase --change <id> --accept-current` 显式改基线，再 refreeze。

这对应 OpenSpec 的 base fingerprint + rebase 思路，但实现更小：不引入三方文本合并器，先把“静默覆盖”变成硬错误。

---

## 5. R0 地基修复

| 项 | 结果 |
|---|---|
| macOS 路径不一致 | `store.Register` 写库前统一 `ResolveRootPath`；`ResolveRootPath` 现在会解析“最长已存在祖先”，未创建的叶子目录也能得到稳定路径。**macOS 55 个失败测试清零。** |
| CI | 新增 `.github/workflows/ci.yml`：Ubuntu + macOS 跑 build/vet/test/gofmt；Windows 作为 advisory（不阻塞，等真机验证）。 |
| 技能包 | 六个公共 `SKILL.md` 已 `go:embed` 进二进制；新增 `beacon skills list/show`。 |
| 版本面 | 统一为 `v2.0.0-go.8`（root.go 单一常量；测试用常量断言）。 |
| LICENSE | **未加**：上游 Python 仓没有 LICENSE 文件，许可证是法律决定，不替用户拍板。 |

---

## 6. 验证证据（本机实测）

- `go build ./...` / `go vet ./...` / `gofmt -l .`：全净。
- `go test ./... -count=1`：7 个包全 ok，**252 个顶层测试**（macOS arm64）。
  - 新增：意图门 4 条、漂移 1 条、并行基线 1 条、需求级指纹 1 条、CLI 意图/skills 2 条、docs-only TC 1 条。
- `bash e2e/run_e2e.sh`：**exit 0**，全链通过；新增 STEP 3b（ask → intent → answer → confirm），冻结前必须签字。
- 真实命令面：`beacon skills list` 输出六条；`beacon truth drift --json` 能指出 `AC-001`。

---

## 7. 还没做的（如实列出）

1. **没有把人机“一问一答”做成自动访谈器**：现在由宿主 agent 调 `truth ask` 发起；内核保证“问了没答就不能冻”，但不替你想问题。
2. **没有三方文本合并**：并行变更先硬拦 + 显式 rebase；自动 merge 文本留到有真实并发样本再做。
3. **plan 的完整 reviewer catalog 未迁**：parity/deferral 只做了 full_parity 的结构门：没有把 Python plan 的完整 reviewer catalog 搬进来。
4. **Humanizer / 人话润色** 未迁：属于宿主侧叙事，不是冻结判据。
5. **Windows** 仅 advisory：worktree/path 套件未在真 Windows 跑过。
6. **LICENSE** 待用户拍板。

---

## 8. 升级注意（老项目）

1. **首次 refreeze 会被意图门拦住**：老包没有 `intent_confirmed` 与意图章节。
   先补 `truth intent`、答完开放问题、`truth confirm`，再 refreeze。
2. **只读命令不受影响**：`truth show/list`、`spine`、`gate status` 照常。
3. **新包自带 Q1**：`truth create` 会种下“谁、什么问题、什么算完成”这一问，
   不回答（或 `--drop`）就冻不住。
4. **AC 变更要重新追溯**：新增/修改 AC 后，`truth intent` 会重建覆盖矩阵并
   清掉旧签字，需要重新 `truth confirm`。
