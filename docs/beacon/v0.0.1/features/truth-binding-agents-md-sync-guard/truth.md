---
slug: truth-binding-agents-md-sync-guard
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_ca6fe1322405
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
AGENTS.md 中用自然语言声明了一组 SDD 规范正文，意图树节点也承载需求，两处内容可能各自演进而产生漂移。治理规则（协作流程、写入口约定等过程性约定）的变化属于治理范畴，不应被当作需求漂移上报。当前缺少只针对规范正文、且能区分治理规则的漂移检测定义。

## 目标用户
维护 AGENTS.md 与意图树的规范负责人、在提交前核对规范与树是否一致的 beacon 使用者。

## 场景
当有人改动 AGENTS.md 的规范正文或治理规则后，运行漂移检查：规范正文的变化应作为需求级 changed 项列出，治理规则字段的变化不得出现在需求漂移报告中。检查过程只读，不回写 AGENTS.md 或意图树。

## 范围
本包交付一个只读、只报不改的漂移检测需求定义：比较 AGENTS.md 声明的 SDD 规范正文与意图树节点，只输出需求级 changed 项并排除治理规则。现有落点 scripts/check_version_consistency.sh 只校验 AGENTS.md 的版本号硬编码，与规范正文漂移无关。规范正文与意图树的漂移检测逻辑当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不自动修改 AGENTS.md 或意图树，检测只报不改；不把治理规则变更计入需求漂移。不做版本号唯一源校验，那属于 runtime-version-source；不做节点级 added/changed/removed 通用漂移报告，那属于 requirement-level-drift。

## 验收标准
治理规则字段变更后报告不含任何需求漂移项，只有规范正文变化被列为 changed 项。验收命令为 `beacon plan drift --agents-md --json 输出仅含需求级 changed 项，治理规则字段不在列表中`，该命令当前尚不存在（现有 plan 命令组只有 init/tree/expand/apply），属待实现能力。
