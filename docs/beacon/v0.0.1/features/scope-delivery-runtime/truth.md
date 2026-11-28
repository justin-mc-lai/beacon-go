---
slug: scope-delivery-runtime
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_d8566e18c1a1
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
现有run与单feature耦合，包装层可能代替范围事实。
## 目标用户
多模块需求交付的用户与宿主agent。
## 场景
用户确认范围后，宿主利用内核确定性绑定、拆分、核销和恢复；内核不调用LLM或启动agent。
## 范围
范围规范化、幂等拆分、全范围准入、证据汇总、恢复与统一入口；具体AC以子叶为准，调用点见docs/plan/scope-delivery-chain-audit.md。
## 非目标
不自动发布、不新增能力组、不物理删除历史、不将旧Portfolio来源变成第二需求权威。
## 验收标准
单feature与多模块同一规范化路径；真实成功/拒绝/恢复/并发探针通过，必需检查和独立复核齐全。需求冻结不等于实现或发布。
