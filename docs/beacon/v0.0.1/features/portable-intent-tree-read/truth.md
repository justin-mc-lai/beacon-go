---
slug: portable-intent-tree-read
intent_confirmation_by: justin-mc-lai
intent_confirmation_fingerprint: 9fe1638cfdd5e8ba1428a98adb8d7ef93a9cc5bc8893f6662209484e4903ae12
sdd_review_fingerprint: 9fe1638cfdd5e8ba1428a98adb8d7ef93a9cc5bc8893f6662209484e4903ae12
sdd_review_at: 2026-11-27T03:11:03.384413Z
sdd_review_verdict: pass
sdd_review_by: omp-independent-qa-spec
version: v0.0.1
status: frozen
language: zh
domain_required: false
fsm_required: false
negative_required: true
surface_strict: true
parser_contract: beacon-feature-package-v2
package_contract: truth-tasks
package_maturity: filled
materials_status: current
traceability_required: true
intent_confirmed: true
package_author: omp-spec-author
surfaces: [cli]
canonical_refs:
  truth: docs/beacon/v0.0.1/features/portable-intent-tree-read/truth.md
  tasks: docs/beacon/v0.0.1/features/portable-intent-tree-read/tasks.md
---

# Requirement Truth: portable-intent-tree-read

## 人话

接手项目的人可以从仓库直接查看意图树。读取不要求登记项目、创建 SQLite 或迁移账本。

## User Intent Snapshot

```yaml
utterance: "做一个后续所有项目需求规划的稳定底座内核；确认首项只做无本机 SQLite 读取"
goal: "换机器后沿用 plan tree 或 truth tree 读取仓库意图树"
non_goals: "不改写入命令、需求正文、冻结历史、并发确认、影响计算、证据复用或发布门禁"
constraints: "指定版本不回退；未指定时保留已登记当前版；否则单版直读、多版列候选；不创建或迁移账本；保留读视图与输出契约"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|---|---|---|---|---|
| Q1 | 无登记当前版时如何选版？ | 单版直读，多版列候选 | answered | 用户在聊天确认首项范围与选版规则 |
| Q2 | 用什么入口验收？ | 通过现有两个命令测试 | answered | 用户确认沿用 plan tree 和 truth tree |

## Grounding

| Source | Reference | Supports |
|---|---|---|
| 用户确认 | restatement.md；规格 #45 | 首项范围、选版规则与验收入口 |
| 架构决定 | ADR-0002 | 仓库 JSON 是唯一权威源，SQLite 是可重建索引 |
| CLI | internal/cli/tree.go 的 newTreeReadCmd 与 resolveTree | 两个读入口共用命令实现，当前要求本机账本 |

## Assumptions

显式请求渲染仍按现有契约生成派生产物。需求读取不改权威文件。

复述正文保留确认前原文；其头部签名记录用户对默认值的确认。`intent_confirmed` 由内核在独立评审后写入，表示签名绑定规格与任务的当前内容。

## Intent Coverage

| AC ID | Intent Source | Evidence |
|---|---|---|
| AC-001 | restates=1；restates=2；restates=3 | 两个入口在隔离的无账本项目中读取成功；核对文件前后状态 |
| AC-002 | restates=3；restates=5 | 多版本、已登记当前版和选版拒绝场景 |

## Acceptance Criteria

| AC ID | Criterion |
|---|---|
| AC-001 | 没有账本时，两个入口都可以读取显式指定的版本。未指定且只有一棵树时，两个入口都直接读取该树。保持现有 JSON 契约。读取成功与拒绝时均不得修改权威树与已有账本，不得创建账本，不得注册项目。restates=2 restates=3 restates=4 |
| AC-002 | 显式选版优先。未显式选版时使用登记的当前版。没有登记的当前版时，唯一候选直接读，多版列出候选并拒绝读取。指定版缺失时拒绝。指定版损坏时拒绝。两种拒绝均不得回退到其他版。restates=5 |

## Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion | Evidence | Required Tests |
|---|---|---|---|---|---|---|
| AC-001 | TC-001 | behavior | `go test -json ./internal/cli -run '^TestPortableIntentTreeRead$/^local_without_store$' -count=1` | 两入口分别显式选版和单版隐式选版；比较同一仓库既有读法的完整 JSON；无账本且权威树不变 | evidence/TC-001.txt | TestPortableIntentTreeRead/local_without_store |
| AC-001 | TC-002 | negative | `go test -json ./internal/cli -run '^TestPortableIntentTreeRead$/^local_no_tree$' -count=1` | 无意图树时两个入口均拒绝；缺失账本不创建；已有其他项目账本及权威文件保持不变 | evidence/TC-002.txt | TestPortableIntentTreeRead/local_no_tree |
| AC-001 | TC-005 | behavior | `go test -json ./internal/cli -run '^TestPortableIntentTreeRead$/^local_legacy_store$' -count=1` | 使用写入口会迁移的旧账本；两个读入口显式选版均成功，账本 schema、内容和权威树不变 | evidence/TC-005.txt | TestPortableIntentTreeRead/local_legacy_store |
| AC-002 | TC-003 | behavior | `go test -json ./internal/cli -run '^TestPortableIntentTreeRead$/^version_registered_current$' -count=1` | 两个可读版本中默认读取登记当前版；显式指定另一版优先；权威树与账本不变 | evidence/TC-003.txt | TestPortableIntentTreeRead/version_registered_current |
| AC-002 | TC-006 | behavior | `go test -json ./internal/cli -run '^TestPortableIntentTreeRead$/^version_unregistered$' -count=1` | 账本只登记其他项目；两个入口不指定版本均读到目标项目唯一树，未注册目标且权威树和账本不变 | evidence/TC-006.txt | TestPortableIntentTreeRead/version_unregistered |
| AC-002 | TC-004 | negative | `go test -json ./internal/cli -run '^TestPortableIntentTreeRead$/^version_reject_' -count=1` | 两个入口均列出歧义版本。分别使用登记当前版和唯一磁盘候选作为可读备选，验证指定版缺失与损坏均拒绝且不输出备选树；权威树与账本不变 | evidence/TC-004.txt | TestPortableIntentTreeRead/version_reject_ambiguous,TestPortableIntentTreeRead/version_reject_explicit |

## FSM

本需求只读取意图树，不引入业务状态或状态迁移。


## 未做

不改写入命令、需求正文、冻结历史或发布门禁。不做并发确认、影响计算或旧证据复用。
