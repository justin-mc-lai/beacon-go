# Stable workflow canonical 需求修订回执

## 结论与授权范围

2026-11-28，用户明确确认最新 plan 调研 / truth 冻结 / change 节点变更 / 一棵树版本演进并授权实现；本次任务仅办理 canonical 需求变更及冻结。执行范围为主仓当前版本 `v0.0.1` 的 feature `n_0386efdd0ffb`，不进行实现、Portfolio 迁移、verified 标记、提交或发布。

本文是实际 CLI 回执的摘要，权威仍为 canonical JSON；旧 `docs/stable-workflow-feature.json`、`docs/stable-workflow-leaves.json` 保留为历史提案，本轮没有修改。

## 候选与变更绑定

| 项目 | 实际值 |
| --- | --- |
| mode_id | `ch-refreeze` |
| next | `stop` |
| source commit | `313ed8751b00a839ea2b1b92300d4c5dc53b8825`，工作区已有未提交实现改动；不是干净 commit 候选 |
| 临时 binary | `/var/folders/2j/1wjsxq4n62n3xx85xmkxy5hm0000gn/T/opencode/beacon-requirement-revision` |
| binary SHA-256 | `e9f2533851e0f86f326564906e9ee8fed5d2c0be59490c325dbf50f3edbaa767` |
| change ID | `chg_50880b0aff24` |
| 变更前 feature fingerprint | `33a45022ea735b27b0764022715fc7f1f6113e521934e6953a03b5e8bbc41375` |
| 变更后 feature fingerprint | `7786668ab9cf092165f527235ed3202f8f28f4c1843802267fc574f3a538d4f6` |
| self-drive fingerprint | `0ba2f06f4d30df6432deaeb5e56188e9a132f66a88920b38ccaf2f639a4d4236` |
| root fingerprint | `4666231259ed86cd2c670b7288b31c3a4600a5786b2f3eab652d8db827784184` |
| 变更前 snapshot | `v0.0.1-20261128T062301.732295000Z` |
| 重新冻结 snapshot | `v0.0.1-20261128T062308.658474000Z` |
| 冻结 snapshot source_hash | `1ff8a3a2d19f77eb2703147b16c34ad98a733d91014db8f735f2c8a6153de822` |

变更记录：`docs/beacon/v0.0.1/tree/changes/chg_50880b0aff24.json`。记录中的 `source_hash` 是请求时来源，不等于冻结后 snapshot source_hash。新 snapshot 位于同树 `snapshots/v0.0.1-20261128T062308.658474000Z/`；历史快照没有手工改写。

## 合同修订及同类扫描

| canonical 节点 / 入口 | 修订或检查 | 成功与拒绝合同 / 后续验证 |
| --- | --- | --- |
| `n_b5eb77526785` stage-contract | plan 仅调查、分析、目标范围与决定，不要求树或 HTML；truth 才要求本轮树、覆盖、明确确认 HTML、自底向上冻结 | 缺树/覆盖/HTML/确认/冻结分别拒绝；保留后续 admission、证据、独立审查与人工发布批准 |
| `n_34b30e01b511` auto-workspace | 本轮 scope、依赖闭包及来源的 scoped 合同 hash；prepare 与 confirm 分离 | 确认绑定身份和 expected prepared hash、合同/source/HTML/manifest；陈旧、篡改、未确认拒绝；并发 A/B 需双执行者屏障证明 |
| `n_8452f7536218` ownership-preview | 同文多来源 AC 可合并到一个 target，各源 AC 恰好覆盖一次，不强制复制 | 重复覆盖/调度、漏项、未知节点、环拒绝；反向不等价描述交宿主独立审查 |
| `n_6f4b7f484e23` continuous-tree-evolution（新增第六叶） | change grow/request/deprecate 唯一变更；注册版本显式 snapshot 与持久 baseline+delta lineage 解析同一 canonical | 历史不可变；连续两轮范围交付，future 隔离，闭包内依赖不可绕过，deprecated 墓碑保留且剔出活跃执行验证；空白/空集合/重复 key/别名/缺失篡改快照/lineage 环/零评估拒绝 |
| `n_0386efdd0ffb` feature 正文 | 五叶改为六叶；纠正阶段及连续演进范围，canonical AC 为权威 | 保持六段正文完整 |
| `n_366977ee5808` selector-snapshot | 读取检查，未改合同；因 feature request 一并转 draft，随后恢复冻结 | fingerprint 仍为 `8920d4aec024ebf75bd05de93a05b79cc52dc2f348f22729f77f95586e79350f` |
| `n_f6ba4267d5d0` shared-decision | 读取检查，未改合同；同上重新冻结 | fingerprint 仍为 `9625256a1f9620441b320bdee742e78cd2c4428f8cf3cb53a7f3fa9247773e5e` |
| self-drive、root | 仅重绑子指纹并冻结，正文未改 | 其余旧子节点不做 update/apply |

新增叶冻结指纹：`2a70430ff8ee0f5b4fd9fea2fb970f0bb8937af6b229ecd21c92762926a69995`。feature 实际 `children_bound: 6`，未超过 7。

## 实际命令及输出

以下 `B`、`T` 仅为命令路径缩写：

```bash
T=/var/folders/2j/1wjsxq4n62n3xx85xmkxy5hm0000gn/T/opencode
B=$T/beacon-requirement-revision
/Users/hainagou-laikunqi/.local/opt/go1.27.4/bin/go build -o "$B" ./cmd/beacon
"$B" plan tree --project . --json
"$B" change request --project . --node n_0386efdd0ffb \
  --base-fingerprint 33a45022ea735b27b0764022715fc7f1f6113e521934e6953a03b5e8bbc41375 \
  --by user-confirmed-host \
  --summary '用户已确认：plan仅调查目标范围决定；truth本轮树覆盖确认HTML冻结；scoped合同hash与明确确认；同文多来源合并单target；新增连续树版本演进合同。仅需求变更与refreeze。' --json
"$B" truth node update --project . --node n_b5eb77526785 --file "$T/revision-stage.json" --json
"$B" truth node update --project . --node n_34b30e01b511 --file "$T/revision-workspace.json" --json
"$B" truth node update --project . --node n_8452f7536218 --file "$T/revision-ownership.json" --json
"$B" truth node update --project . --node n_0386efdd0ffb --file "$T/revision-feature.json" --json
"$B" plan expand --project . --node n_0386efdd0ffb --children "$T/revision-evolution.json" --json
"$B" plan apply --project . --node n_0386efdd0ffb --proposal "$T/revision-evolution.json" --json
```

以上成功命令退出码均为 0。expand 返回 `allowed: true`；apply 返回 `applied: 1`，生成 `n_6f4b7f484e23`。JSON 输入存放临时目录，树的全部写入经 CLI；当前 canonical 节点及 CLI 生成的 expansion 留存最终合同。

实跑 `truth node freeze --project . --node <id> --json`，严格依次为：

1. `n_34b30e01b511`
2. `n_8452f7536218`
3. `n_366977ee5808`
4. `n_f6ba4267d5d0`
5. `n_b5eb77526785`
6. `n_6f4b7f484e23`
7. `n_0386efdd0ffb`
8. `n_5e13a94c202d`
9. `nroot_6b8991789867`

九次均退出 0，返回 `status: frozen`、`already_frozen: false`；root 返回上述新 snapshot ID。未使用任何 mark verified 或 verify leaf 命令。

```json
{"command":"verify audit --project . --json","ok":true,"findings":null,"node_count":63,"reads_only":true}
{"command":"plan drift --project . --json","snapshot_id":"v0.0.1-20261128T062308.658474000Z","node_id":"nroot_6b8991789867","added":[],"changed":[],"removed":[]}
```

两项均退出 0。audit 同时返回 19 个 `tree_test_case_missing` advisory，包括 stable workflow 六叶；它们不是已完成的行为验收。

初次读树曾尝试 `--version stable`，实际拒绝 `docs_version_invalid`；随后使用 current 解析到 `v0.0.1`。此失败为只读参数错误，没有写树。

## 仓库检查真实结果

使用 `PATH=/Users/hainagou-laikunqi/.local/opt/go1.27.4/bin:$PATH` 执行 Go 检查：

| 命令 | 实际结果 |
| --- | --- |
| `go build ./...` | 退出 0，无输出 |
| `go test ./... -count=1` | 退出 0，所有包通过；`internal/cli 78.092s`、`internal/orchestrator 108.181s`、`internal/intenttree 2.688s` |
| `go vet ./...` | 退出 0，无输出 |
| `bash scripts/check_version_consistency.sh` | 退出 0，`version-consistency: ok (0.0.1)` |
| `bash scripts/check_kernel_deps.sh` | 首次未给脚本传 Go PATH，退出 127：`go: command not found`；补正确 PATH 重跑退出 0：`kernel-deps: ok (no removed surfaces in the binary)` |
| `bash e2e/run_e2e.sh` | 未验证；本轮仅需求 JSON 与回执变更，没有变更实现或 E2E 链路 |

本轮是需求合同冻结，不适用实现 Red/Green；等价检查为真实 change/update/expand/apply/freeze/audit/drift。全包回归通过不证明新增 AC 已被完整覆盖。独立实现验收未执行，连续两轮完整交付未执行，版本 lineage 产品行为未在本轮验收；新合同中的这些义务留给后续实现与独立验证。

## 文件范围与交接

实际需求变更文件位于 `docs/beacon/v0.0.1/tree/`：index、feature、三个修订叶、新增演进叶、self-drive/root 指纹、change、expansion 及两份 CLI 快照。两个未改叶恢复原冻结指纹。另新增本回执。仓内既有实现/skill/其他文档改动为任务开始前已存在的工作，本次未编辑。

plain_summary：错误冻结的 plan 树要求已通过唯一 change 入口修正，六叶需求树已自底向上冻结；这里只确认需求冻结，不确认实现验收或发布。
