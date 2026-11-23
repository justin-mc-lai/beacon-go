---
slug: agent-session-streaming
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
canonical_refs:
  truth: docs/beacon/v2.0.0/features/agent-session-streaming/truth.md
  tests: docs/beacon/v2.0.0/features/agent-session-streaming/tests.md
  tasks: docs/beacon/v2.0.0/features/agent-session-streaming/tasks.md
  evidence: docs/beacon/v2.0.0/features/agent-session-streaming/evidence.md
---

# Requirement Truth: agent-session-streaming

## 人话

给本机开发者：桌面工作台的会话能真正落盘、能列出、能回放；参谋长的回答以增量 chunk 推给界面做打字机效果，刷新后历史还在。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "桌面端会话别写死，要有真实历史和逐字输出"
goal: "会话与消息持久化，流式增量推送，界面动态加载并支持回放"
non_goals: "不把会话历史塞进内核九表账本；不做第二套渲染路径"
constraints: "原子追加；流式只走 Wails 事件；增量不少于 3 个 chunk"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 给本机开发者：会话可持久化与回放、回答可流式显示；完成=重启后历史仍在、增量到达界面、切换会话能回放 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「会话持久化」 | `TestSessionStore_CRUD` |
| AC-002 | 目标「流式增量」 | `TestStreamCompleteChunks` |
| AC-003 | 目标「界面动态加载」 | `TestShellSessionAndStreamWiring` |
| AC-004 | 目标「历史回放」 | `TestAppSessionAPIs` |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 会话与消息持久化到 SQLite：创建、列出、读取、追加，写入与时间戳同事务 |
| AC-002 | 流式增量：SSE 每个 delta 作为独立 chunk 回调，多段回答产生 3 个以上增量 |
| AC-003 | 界面动态加载：侧栏会话来自后端，活动会话渲染打字机效果，事件名为 `chat:stream:chunk` |
| AC-004 | 历史回放：切换会话时清空画布并恢复全部消息（含 Plan/Goal 卡片） |

## FSM 五元组

Not required: domain_required is false.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|

## Parity Matrix

Not required: scope_mode is focused.

| Capability | Covered | Evidence |
|------------|---------|----------|
