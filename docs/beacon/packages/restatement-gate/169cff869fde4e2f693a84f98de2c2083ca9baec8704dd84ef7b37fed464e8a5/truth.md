---
slug: restatement-gate
surface_strict: true
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
surfaces: [cli]
canonical_refs:
  truth: docs/beacon/v2.0.0/features/restatement-gate/truth.md
  tests: docs/beacon/v2.0.0/features/restatement-gate/tests.md
  tasks: docs/beacon/v2.0.0/features/restatement-gate/tasks.md
  evidence: docs/beacon/v2.0.0/features/restatement-gate/evidence.md
---

# Requirement Truth: restatement-gate

## 人话

`dispatch.sh --restatement` 现在只检查「非空」，AI 对需求的理解可以一路绿灯冻成 truth、派成实现，错到用户看截图才发现。本需求把「复述」变成 truth 冻结前的强制工件：`restatement.md` 必须五段齐全、有人确认、正文 hash 与确认时一致，且每条 AC 都能指回五段里的一节；首次冻结不满足即拒绝，已冻结包 refreeze 保留旧豁免。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "dispatch.sh 的 --restatement 只是个非空即过的自由文本，等于没有门；把复述变成 truth 冻结前的强制工件，未确认不许冻结"
goal: "首次冻结前强制校验 restatement.md（存在、已确认、hash 未漂移、五段齐全），AC 可追溯到节号，truth show 可观测，既有 refreeze 语义不变"
non_goals: "不改 dispatch.sh 本身；不动 truth/design/implement/qa/release 既有语义；不新增顶层命令；不 push 不安装不发布"
constraints: "只加不改；Go 内核；首次冻结强制、refreeze 走 legacy waiver；go test ./... 全绿；commit 仅本地"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 门放在 dispatch 侧还是 truth freeze 侧？ | 放 truth freeze 侧：dispatch 的自由文本改不动真相，冻结是唯一入口 | answered | truth freeze 侧；dispatch.sh 本次不改 | human(dispatch 36786bb4) |
| Q2 | 老包（已冻结、无 restatement）怎么办？ | 与 surfaces 同规则：refreeze 保留 legacy waiver，只 warn | answered | refreeze 豁免；AC 追溯对老包只 warn | human(dispatch 36786bb4) |

## Acceptance Criteria

`restates=` 指回 `restatement.md` 的节号；本包自身也按新规则标注，作为格式样例。

| AC ID | Criterion | Level |
|-------|-----------|-------|
| AC-001 | 新增 `internal/gate/restatement.go`：解析 frontmatter 的 `slug`/`confirmed_by`/`confirmed_at`/`hash`；正文 = frontmatter 结束 `---` 之后的全部字节，逐字节计算 sha256；按行前缀校验 `## 1` … `## 5` 五段齐全并允许 `## 3.2` 子标题；解析结果暴露「正文 hash 与 frontmatter hash 是否一致」。单测含五段齐全 + 子标题的正例。restates=1 | Must |
| AC-002 | 首次冻结在 `freezePrecheck` 中强制校验 restatement，插入位置在 surface 校验之后、`EvaluateIntegrity` 之前：文件不存在 → `restatement_required`；`confirmed_by` 为空 → `restatement_unconfirmed`；现算 sha256 ≠ frontmatter `hash` → `restatement_stale`；五段标题缺失 → `restatement_sections_missing`。四类失败均返回 blocked 视图、CLI 非 0 退出，且 surface 同时失败时先报 surface。restates=3.1 | Must |
| AC-003 | 五段齐全 + `confirmed_by` 非空 + hash 一致时首次冻结通过，包为 `frozen`；已冻结包的 refreeze 不做 restatement 强制（无 restatement 的老包 refreeze 仍通过，只记 warning），与 surfaces 的 legacy waiver 同规则。restates=3.2 | Must |
| AC-004 | `truth.md` 的 Acceptance Criteria 表每行支持 `restates=<节号>` 记号（可带子节号，如 3.2；同一行可重复）；节号必须能在 restatement 五段（含子标题）中找到；找不到或缺失 → `ac_not_traceable_to_restatement`。Intent Coverage 表不要求该记号。老包（无 restatement）不校验，只 warn。restates=3.1 | Must |
| AC-005 | `truth show --json` 增加 `restatement` 块：`path`、`confirmed_by`、`confirmed_at`、`hash`、`matches`；`matches` 仅表示正文现算 hash 与 frontmatter hash 一致；无 restatement 文件的老包省略该块，其余 JSON 不变。restates=2 | Must |
| AC-006 | `internal/core/reasons.go` 增加五个常量：`restatement_required`、`restatement_unconfirmed`、`restatement_stale`、`restatement_sections_missing`、`ac_not_traceable_to_restatement`；内核代码只引用常量，不自拼字符串。restates=2 | Must |
| AC-007 | 单测覆盖五个负例（缺文件 / 未确认 / hash 漂移 / 缺段 / AC 节号不存在）与两个正例（解析与首次冻结通过）；`go test ./...` 全绿，既有测试零回归。restates=1 | Must |
| AC-008 | 只加不改：既有 truth/design/implement/qa/release 语义不变，只新增文件与最小插入点；实现完成后 commit 到本地，不 push、不安装 `~/.local/opt/beacon-go/beacon`。restates=4 | Must |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | utterance「把复述变成强制工件」+ 工件格式 | tests.md TC-001 |
| AC-002 | 「未确认不许冻结」+ 五件事第 2 条 | tests.md TC-002..TC-005 |
| AC-003 | 「已冻结包 refreeze 走 legacy waiver」 | tests.md TC-007/TC-008 |
| AC-004 | 五件事第 3 条（AC 可追溯） | tests.md TC-006 |
| AC-005 | 五件事第 4 条（视图） | tests.md TC-009 |
| AC-006 | 五件事第 5 条（reason codes） | tests.md TC-010 |
| AC-007 | 硬约束第 3 条（五负例 + 两正例 + 全绿） | tests.md TC-011/TC-013 |
| AC-008 | 硬约束第 1、4 条（只加不改、不 push 不安装） | git diff 范围 + tests.md TC-012 |

## 设计约定

1. 解析器与 `surface.go` 同层：纯解析、无状态；orchestrator 只消费解析结果，不重复实现 hash/标题匹配。
2. 正文 hash 按 `---` 之后的原始字节计算，不做 trim/换行归一；确认后改一个字必须变 `restatement_stale`。
3. 首次冻结与否只有一个事实来源 `recs.freeze == nil`，restatement 与 surfaces 共用，不引入第二套判定。
4. 老包只 warn：不补文件、不改 truth、不把 warning 升级成 blocked。
5. 顺序固定为四件套 → surface → restatement → `EvaluateIntegrity` → intent gate；单测用「surface 与 restatement 同时失败」断言先后。
