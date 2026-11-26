---
slug: node-id-namespace
surface_strict: true
intent_confirmation_by: main
intent_confirmation_fingerprint: 357d53b14f26ddb6b189406473724e328817c6b32a4cbccde5d141fd5adf1756
sdd_review_fingerprint: 357d53b14f26ddb6b189406473724e328817c6b32a4cbccde5d141fd5adf1756
sdd_review_at: 2026-11-26T14:53:42.072279Z
sdd_review_verdict: pass
sdd_review_by: SemReviewNodeID3-r7-freeze-gates
version: v0.0.1
status: frozen
language: zh
domain_required: false
fsm_required: false
negative_required: true
parser_contract: beacon-feature-package-v2
package_contract: truth-tasks
package_maturity: filled
materials_status: current
traceability_required: true
intent_confirmed: true
package_author: main
confirmed_by: main
surfaces: [cli]
canonical_refs:
  truth: docs/beacon/v0.0.1/features/node-id-namespace/truth.md
  tasks: docs/beacon/v0.0.1/features/node-id-namespace/tasks.md
---

# Requirement Truth: node-id-namespace

## 人话

多个项目可以同时处在同一文档版本（例如 v0.0.1）。意图树的节点 id 以前只用版本号派生，导致任何项目建根都得到同一个 id，撞数据库全局主键。这个需求保证所有节点 id 都带上项目版本标识：不同项目的同版本号互不冲突，同一项目重复派生结果稳定。

## User Intent Snapshot
```yaml
utterance: "任何项目不共享文档版本号；节点 id 必须按项目命名空间，跨项目同版本号不撞主键"
goal: "5 类身份 id 有跨项目隔离回归测试；7 类调用点全部经白名单审计无 version-only 派生"
non_goals: "不改用户可见的版本号语义；不改 docs/runtime 两轴规则；不迁移历史 v3.0.2 树的既有 id"
constraints: "id 必须确定性可复现；全局主键唯一；修复前 version-only 派生不得留在生产调用点"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|---|---|---|---|---|
| Q1 | 用户要获得什么可验证结果，哪些工作明确不做？ | 以跨项目同版本号不撞主键为可验证结果；不改版本号语义、不迁移历史树 | answered | 可验证结果=身份 id（nroot_/n_/nfeat_/nmod_/tmig_）跨 versionID 隔离且 nroot_ 同输入确定性、全部 shortID 调用点白名单审计无 version-only；不迁移既有树 |
| Q2 | 本需求是否有领域状态与安全拒绝路径？ | 无业务 FSM（纯 id 派生函数）；负向路径=version-only 派生不得复现 | answered | fsm_required=false：这是确定性派生与校验，无业务状态流转；negative_required=true：跨项目撞主键必须被测试拒绝 |

## Grounding

| Source | Reference | Supports |
|---|---|---|
| 源码 | internal/intenttree/migrate.go:45 shortID | id 派生单点 |
| 源码 | internal/intenttree/init.go:18,39 | 根/迁移 id 已改用 versionID |
| 源码 | internal/intenttree/migrate.go:88-103,141 | legacy/feature/迁移 id 已改用 versionID |
| 真源 | AGENTS.md「唯一真源」「防返工」 | 一个规范化入口、退化探针 |
| 测试 | internal/intenttree/init_test.go | 5 类身份 id 跨 versionID 隔离与 nroot_ 确定性回归 |
## Assumptions

- 假设 project_versions.id 全局唯一且稳定（store.AddVersion 保证），可作为命名空间种子。确认依据：internal/store/dao.go:120-153。
- 假设 n_/chg_/exp_ 的父 id 已含 versionID，继承唯一性；但它们本身仍需直接测试覆盖，不能只靠推断。

## Intent Coverage

| AC ID | Intent Source | Evidence |
|---|---|---|
| AC-001 | utterance「节点 id 必须按项目命名空间」 | TestShortIDNamespacedByVersionID 覆盖 5 类身份 id 前缀 |
| AC-002 | constraints「version-only 派生不得留在生产调用点」 | grep 生产调用点无短 id 只用 version 的 |
| AC-003 | utterance「跨项目同版本号不撞主键」 | 两 projectID 同 version 建根得到不同 id |
## Acceptance Criteria

| AC ID | Criterion |
|---|---|
| AC-001 | 身份 id（nroot_/n_/nfeat_/nmod_/tmig_）按 versionID 派生：不同 versionID 产出不同 id、相同输入确定性产出相同 id；事件/记录类 id（chg_/exp_）含时间戳，不要求跨调用同输入确定性，但参数必须含 versionID，若经父 id 间接派生则父 id 必须由 versionID 派生。 |
| AC-002 | 生产代码中不存在仅以 version 字符串派生身份 id 的调用点；显式白名单审计（排除函数定义、versionID/VersionID、parentID 派生）无残余。 |
| AC-003 | 两个不同项目版本（versionID）派生的 root id 不同——ID 派生层实现跨项目隔离，从根因上消除全局主键冲突；由 TASK-001 测试中两个 versionID 的 nroot_ 派生不相等断言证明。 |

## Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion | Evidence | Required Tests |
|---|---|---|---|---|---|---|
| AC-001 | TC-001 | unit | `go test ./internal/intenttree -run TestShortIDNamespacedByVersionID -count=1` | 通过且测试对 nroot_/n_/nfeat_/nmod_/tmig_ 逐一断言跨 versionID 隔离，并对 nroot_ 断言相同输入确定性 | evidence/TC-001.txt | TC-001 |
| AC-002 | TC-002 | unit | `python3 -c "import glob; bad=[f'{p}:{i+1}:{l.rstrip()}' for p in glob.glob('internal/intenttree/*.go') if not p.endswith('_test.go') for i,l in enumerate(open(p)) if 'shortID(' in l and not any(x in l for x in ['func shortID','versionID','VersionID','parentID'])]; print('\n'.join(bad)); raise SystemExit(bool(bad))"` | 退出码 0 且无输出 | evidence/TC-002.txt | TC-002 |
| AC-003 | TC-003 | unit | `go test ./internal/intenttree -run TestShortIDNamespacedByVersionID -count=1` | 测试内两个 versionID 的 nroot_ 派生结果不相等的断言通过 | evidence/TC-003.txt | TC-003 |
| AC-001 | TC-004 | negative | `go test ./internal/intenttree -run TestShortIDNamespacedByVersionID -count=1` | 测试内 version-only 派生探针与 namespaced id 不相等的断言通过，证明旧算法会碰撞 | evidence/TC-004.txt | TC-004 |
| AC-002 | TC-005 | negative | `python3 -c "bad='shortID(\\\"nroot_\\\", version)'; hit=('shortID(' in bad and not any(x in bad for x in ['func shortID','versionID','VersionID','parentID'])); raise SystemExit(0 if hit else 1)"` | 退出码 0，证明白名单审计能检出 version-only 派生 | evidence/TC-005.txt | TC-005 |
| AC-003 | TC-006 | negative | `go test ./internal/intenttree -run TestShortIDNamespacedByVersionID -count=1` | 测试内两个 versionID 派生 root id 不等的负向断言通过，证明同版本号不碰撞 | evidence/TC-006.txt | TC-006 |

## FSM

`fsm_required: false`。理由：本需求是确定性 id 派生函数与回归校验，无业务状态、无实体生命周期、无审批或流转，不构成领域状态机。按 AGENTS.md「不得用包生命周期冒充业务 FSM」，不编造 FSM。

## Transition Coverage

无（fsm_required: false）。


- 不迁移历史 v3.0.2 树既有 id（它们是 frozen 工件，改 id 会破坏指纹链）。
- 不改 runtime/docs 两轴的版本号规则（属 version-evolution 模块）。
- 不改节点 CRUD 的 kind 规则与 7 子节点上限（属 intent-tree-crud feature）。
