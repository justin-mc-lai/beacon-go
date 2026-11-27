---
slug: self-drive-self-goal-loop
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_41398c3173e0
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
在 beacon-go 自身仓库尚无以项目身份登记并跑通 goal 交付闭环的入口，自身开发无法 dogfood beacon goal 全流程。

## 目标用户
维护 beacon-go 的开发者与发布者，需要在本仓验证 goal 闭环。

## 场景
开发者在仓库根执行项目登记命令，期望登记后 current_version 为 v0.0.1，并以此版本为起点驱动 goal 完整交付。

## 范围
定义本仓作为 beacon 项目登记并以 v0.0.1 启动 goal 闭环的契约：登记幂等、current_version 固定为 v0.0.1。现有落点 internal/cli/goal*.go 提供 goal 基础设施，但尚无 self-goal-loop 专属 wiring，当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不实现 goal 调度、worktree 分配与多 module 并行（属 module-scheduler）；不处理缺陷回流（属 defect-feedback）；不校验树指纹与索引投影（属 self-tree-ci）。

## 验收标准
登记命令 `beacon projects register --path . --version v0.0.1 --json` 输出的 current_version 等于 "v0.0.1"（jq -e '.current_version == "v0.0.1"' 退出 0）。该命令当前尚不存在，属待实现能力。

## 状态迁移

| From | To | Event |
|---|---|---|
| idle | running | run |
| running | verifying | implement_done |
| verifying | delivered | gate_pass |
| verifying | blocked | gate_block |

### 非法迁移

- idle -> delivered 必须被拒绝
- blocked -> delivered 必须被拒绝
