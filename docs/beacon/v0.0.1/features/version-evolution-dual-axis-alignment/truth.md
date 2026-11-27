---
slug: version-evolution-dual-axis-alignment
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_26809ef014ec
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
存在两条版本轴：runtime 二进制版本取自 version.go，docs 文档版本线由 manifest.Version 表示。发布时需要把 runtime 版本对齐写入 manifest.Runtime，但两轴平时应各自独立演进、互不相改。manifest.Runtime 字段虽已定义，却没有发布时的对齐写入逻辑。

## 目标用户
执行版本发布的管理员，以及维护 docs manifest 与发布脚本的版本负责人。

## 场景
发布 v0.0.1 时，将当前 runtime 版本写入 docs/beacon/v0.0.1/manifest.json 的 runtime 字段，使其与 beacon-go --version 去掉前缀 v 后一致；非发布阶段两轴独立变化，不互相覆盖。若发布前 manifest.runtime 与二进制版本不等，视为未对齐。

## 范围
本包交付"发布时对齐"的需求定义：runtime Version 写入 manifest.Runtime，docs 版本线仍由 manifest.Version 表达，两轴平时独立。现有落点 internal/versionmanifest/manifest.go:45 已定义 Runtime 字段，Inherit 会从父 manifest 继承该字段；但发布对齐写入逻辑当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不改变 version.go 作为版本唯一源的职责，那是 runtime-version-source 的范围；不要求 docs 版本线与 runtime 版本长期相等，两轴平时独立。不做需求漂移报告（requirement-level-drift）或 AGENTS.md 规范漂移检测（agents-md-sync-guard）。

## 验收标准
发布产物 docs/beacon/v0.0.1/manifest.json 的 runtime 字段等于 beacon-go --version 去掉前缀 v 后的值。验收命令为 `test "$(jq -r .runtime docs/beacon/v0.0.1/manifest.json)" = "$(beacon-go --version | sed 's/^v//')"`；该命令依赖的 manifest 文件与对齐写入逻辑当前尚不存在，属待实现能力。
