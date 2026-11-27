---
slug: version-evolution-docs-version-line
version: v0.0.1
status: frozen
source: tree
tree_node_id: n_143f1617bc78
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
# docs version line

## 意图

项目级 docs 版本创建、bump、manifest 基线/继承，版本按 project_id 隔离。

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
| frozen | released | release |

### 非法迁移

- released -> draft 必须被拒绝
- draft -> released 必须被拒绝

> 生成骨架：语义正文与真实测试由宿主 agent 补全；不得把占位符当作已验证结果。
