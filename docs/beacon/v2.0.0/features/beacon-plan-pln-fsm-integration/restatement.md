---
slug: beacon-plan-pln-fsm-integration
confirmed_by: apple
confirmed_at: 2026-11-17T07:29:00+08:00
hash: 128eb29447f08484077fa7fa534eec2c953b567ab338f296c87b49e790b93ad6
---

## 1 用户意图与核心目标
在现有 `beacon-plan` 技能中深度集成领域状态机模式（`pln-fsm`）。凡是涉及工作流、订单履约、派工调度、状态卡片或生命周期管理的需求，强制进入 FSM 拆解模式，杜绝模糊文字，生成包含实体前置序、聚合根状态矩阵与非法跃迁槽位的标准契约。

## 2 界面与交互 (CLI & Skill Surface)
1. `skills/plan/SKILL.md`：
   - 模式列表新增 `pln-fsm`。
   - HARD GATE 明确加入状态机强卡条件。
2. `skills/plan/references/core.md`：
   - 补充 `pln-fsm` 运行机制与 Go 表面映射。
3. `skills/plan/examples/domain-fsm-decomposition.md`：
   - 提供标准业务状态机拆解示例。

## 3 范围与非目标 (Scope & Non-Goals)
- **范围 (In Scope)**：
  - 更新 `beacon-go` 内置的 `skills/plan` 技能定义。
  - 通过 `beacon-go skills install --force` 同步分发至各个主机环境（Claude、Codex、Opencode、Cursor）。
- **非目标 (Non-Goals)**：
  - 不篡改既有 `pln-brainstorm` / `pln-review` 等通用规划模式。
  - 不在内核源码中引入特定业务硬编码。

## 4 边界与异常分支 (Boundaries & Exceptions)
- 非状态机类纯查询或文档型需求不强制进入 `pln-fsm`。
- 状态机拆解产物必须无悬空实体、无孤儿状态，且每条 Illegal Transition 均具备业务防御意图。

## 5 验收标准与测试 (Acceptance & Tests)
- 单测：`go test ./skills/...` 全部通过。
- 多端验证：`beacon-go skills verify` 显示各 host 技能环境完全匹配。
- 远程同步：提交并推送至 `beacon-go` 远程主分支。
