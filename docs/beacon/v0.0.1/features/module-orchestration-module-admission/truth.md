---
slug: module-orchestration-module-admission
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_f6f625e5f61b
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
当前只有 feature 粒度 implement 准入（internal/orchestrator/admission.go 的 CheckImplementAdmission），没有 module 入口的统一准入；module run 若直接启动，可能在子 feature 的 truth 尚未 freeze 时就进入执行。结果是任一子 feature 处于 change_pending / draft 时 module 仍会被拉起，违背「任一子 feature blocked 则 module 不启动」的约束。

## 目标用户
通过 `beacon goal module run --node <module-id>` 批量启动一个 module 下多个 feature run 的工作流作者与操作者，以及需要确认启动前所有子 feature 已满足 freeze 条件的 gate 维护者。

## 场景
一个 module 节点下挂多个 feature，其中一个子 feature 的 truth 还是 draft 或存在未处理的 change。操作者调用 module run 时，期望命令在共享 admission 检查阶段被拒绝、退出非零并给出 reason=freeze_ack_required，module 内所有 feature 都不启动。

## 范围
本 feature 冻结「module 入口统一调用共享 admission 检查，任一子 feature blocked 则 module 不启动」这一需求边界。现有 feature 级检查在 internal/orchestrator/admission.go（CheckImplementAdmission / checkAdmission），可作为共享检查的落点。module 级 admission 聚合与 module run 入口当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不负责 module 展开成多个 run 的调度、顺序或 worktree 并行分配（由兄弟 feature module-scheduler 冻结），也不定义单个 feature 内部的 implement 准入细则（沿用现有 feature 级 admission）。

## 验收标准
验收命令为 `beacon goal module run --node <module-id>`，该命令当前尚不存在，属待实现能力。就位后，当任一子 feature 的 truth 未 freeze 时，该命令应退出非零（退出码 != 0）并给出 reason=freeze_ack_required；仅当所有子 feature 均通过 admission 时才允许 module 启动。
