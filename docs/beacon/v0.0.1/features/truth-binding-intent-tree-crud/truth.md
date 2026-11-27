---
slug: truth-binding-intent-tree-crud
version: v0.0.1
status: frozen
source: tree
tree_node_id: n_1a712a5f8a04
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
# intent tree CRUD

## 意图

为意图树节点提供显式增删改入口；删除走 change request 作废链路而非物理删除，保持真相可追溯。

## 问题

- [待补充：这个需求要解决的用户问题]

## 目标用户

- [待补充：谁在什么场景下使用]

## 场景

- [待补充：关键用户场景]

## 范围

- [待补充：本次做什么]

## 非目标

- [待补充：本次明确不做什么]

## 用户流程

1. [待补充：用户主动作]
2. [待补充：系统响应]

## 异常与边界

- [待补充：失败、重试、取消或超时]

## 验收标准

- [待补充：用户可观察的结果]

## 状态迁移

| From | To | Event |
|---|---|---|
| draft | frozen | freeze |
| frozen | change_pending | change_request |
| change_pending | frozen | refreeze |
| frozen | deprecated | deprecate |

### 非法迁移

- deprecated -> frozen 必须被拒绝
- draft -> deprecated 必须被拒绝

> 生成骨架：语义正文与真实测试由宿主 agent 补全；不得把占位符当作已验证结果。
