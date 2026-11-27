---
slug: self-drive-defect-feedback
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_5e74132fe68f
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
缺陷目前没有被自动回流为树节点或 change 记录并进入实现闭环；虽然存在冻结 truth 的 change 原语，但缺少「缺陷 → change → refreeze → 实现」的自驱回流定义，缺陷与意图树之间没有可追溯的关联。结果是同一个缺陷可能被反复发现，却在树上留不下 change_id 与闭环轨迹。

## 目标用户
在 dogfood 场景中记录 beacon-go 自身缺陷并希望其进入实现闭环的规格作者与维护者，以及通过 change 记录追踪缺陷修复进度的 QA / 发布验证者。

## 场景
在一次自驱迭代中发现一个 gap（例如 self-drive-gap）。系统应把该缺陷回流为对某个树节点的 change 记录，生成可查询的 change_id，随后该 change 进入 refreeze 与实现阶段，形成闭环。

## 范围
本 feature 冻结「缺陷自动回流为树节点或 change 记录并进入实现闭环」这一需求边界。现有可复用落点是冻结 truth 的 change 记录原语（internal/cli/change.go 的 change request，internal/intenttree/change.go 的 ChangeRecord / RequestChange），它按 --node 与 --summary 生成 change 记录。缺陷到 change 的自动回流触发与闭环串联当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不负责用 beacon goal 在 beacon-go 自身仓库跑完整交付闭环（由兄弟 feature self-goal-loop 冻结），也不覆盖 CI 对本仓意图树指纹、索引与投影的校验（由兄弟 feature self-tree-ci 冻结）。不定义缺陷的自动发现或采集来源，只冻结「已知缺陷回流为 change 并闭环」这一段。

## 验收标准
验收命令为 `beacon truth change --project . --node n_5e13a94c202d --summary self-drive-gap --json | jq -e '.change_id'`，该命令当前尚不存在，属待实现能力。就位后，对指定节点提交缺陷摘要应生成一条 change 记录并输出非空 change_id，且该 change 可进入 refreeze 与后续实现阶段。
