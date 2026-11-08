# P1 源锚点：kernel-stride-control

- 上游仓库：https://github.com/loopx-project/loopx
- 基线 commit：c9c0644be428bbb0ab10726637f5b36602972912（2.3.0 发布窗口 HEAD）
- 锚点清单来源：BeaconGo 研究 2026-11-09 落盘的 loopx-p1-p2-p3-p5-source-anchors.md，本文件只摘 P1 部分
- 主锚点文件：docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md
- 辅助实现参考：loopx/control_plane/work_items/stride_policy.ts

## AC-001（上游 AC-S1）

- 小节：## 1. Decision Summary；## 5. The Three-Layer Model
- 行号：L24-L35，L192-L245
- 支撑点：RFC 定义三层嵌套步长——effect 层是一次决策内可观察的因果动作，delivery 层是 Todo 拥有、带可验证证据的交付与 settlement 边界，authority 层是权限与人工门禁覆盖的范围。记录三层边界与停止原因，才能判断一次 turn 为什么停。

## AC-002（上游 AC-S2）

- 小节：### 6.1 Semantic boundaries outrank numeric budgets
- 行号：L330-L337
- 支撑点：RFC 原文明确 "ceiling is not a completion rule. The controller should end early on semantic change and must not claim success merely because a maximum was reached." 数值上限只作安全上限，触顶必须标记未完成，完成需要语义 closeout。

## AC-003（上游 AC-S3）

- 小节：### 6.2 Inner continuation cannot borrow outer authority；### 6.3 Outer continuity cannot excuse missing settlement
- 行号：L339-L352
- 支撑点：内层续跑不能借用外层授权，越过权限或接受边界必须停下；外层连续续跑不能以连续为由跳过 settlement。

## AC-004（上游 AC-S4）

- 小节：### 6.4 Scheduler cadence is not semantic cadence
- 行号：L354-L368
- 支撑点：RFC 原文 "A wake signal says that the host may inspect current state. It does not prove that a new Todo is needed, that a coherence slice ended, or that human attention is warranted." 调度唤醒只允许检查状态，不算语义进展。
