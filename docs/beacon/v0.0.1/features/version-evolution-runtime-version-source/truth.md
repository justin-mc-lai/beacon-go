---
slug: version-evolution-runtime-version-source
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_ff59308937cf
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
版本号容易在 README、plugin.json、AGENTS.md、git tag、release notes、CHANGELOG 等多处出现第二份字面量，一旦与代码脱节就产生版本不一致。需要确立 version.go 为唯一权威源，保证所有引用点无第二份字面量，并覆盖 tag/release/CHANGELOG 的 V-08/V-09 完整性。

## 目标用户
打 tag 与编写 release notes 的发布管理员，以及维护 CI 版本门禁的开发者。

## 场景
打 tag 或更新 CHANGELOG 后运行一致性门禁：脚本检查 version.go 中恰有一个 const Version 字面量，且最新 git tag、release notes 与 CHANGELOG 记录的版本都等于 Version；出现第二份字面量或任一引用点版本不等则失败。CI 的 version consistency 步骤在提交时触发同一校验。

## 范围
本包交付版本唯一源门禁的需求定义：version.go 为唯一源、禁止第二份字面量、覆盖 tag/release notes/CHANGELOG 完整性（V-08/V-09）。现有落点包括 internal/version/version.go、scripts/check_version_consistency.sh（当前只校验 README/plugin/AGENTS.md 的当前版本声明）与 .github/workflows/ci.yml 的 version consistency 步骤；但 tag/release notes/CHANGELOG 的完整覆盖当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不改写或生成版本号，只做一致性校验；不做发布时 manifest.Runtime 与二进制版本的对齐写入，那是 dual-axis-alignment 的范围。不做 AGENTS.md SDD 规范与意图树的漂移检测（agents-md-sync-guard）。

## 验收标准
一致性脚本通过，且 git describe 最新 tag、release notes、CHANGELOG 记录的版本均等于 version.go 的 Version。验收命令为 `bash scripts/check_version_consistency.sh && test "$(git describe --tags --abbrev=0)" = "v$(awk '/^const Version = /{gsub(/"/,"",$4);print $4;exit}' internal/version/version.go)"`；脚本与 CI 步骤已有落点，但 tag/release notes/CHANGELOG 的完整覆盖属待实现能力，当前尚不能完整验收。
