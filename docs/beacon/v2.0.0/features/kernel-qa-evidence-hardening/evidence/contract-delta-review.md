# 合同增量复核（delta）· kernel-qa-evidence-hardening

- scope: **contract_only**
- checker: qa-regression-checker（grok 独立，非 maker）
- repo: /Users/apple/Developer/Personal/products/beacon-go
- HEAD: `bd0195a`
- run: `run_932e20c89ae9c375a7a5cab5` · stage `plan` · blocked `false` · gate plan evaluated=false
- 复核范围：仅 interfaces §1 与 truth Illegal 段；实跑 13 合法边 + 2 非法边。未全仓读，未跑产品测试。
- 未实现、未做产品 QA、未 attach verifier、未 freeze/commit/push、未推进门、未碰旧 monitor。旧报告 `contract-review.md` 保留未动。

## 本轮实读文件（sha256 前16 / 行数）

| 文件 | sha256 | 行数 | 与上一版 |
|---|---|---|---|
| truth.md | `c56cbd4e841bfba6` | 143 | 由 `9c9551eb` 变更（Illegal 4→2 条） |
| interfaces.md | `d233a6d6f81cce15` | 73 | 由 `c64b29a8` 变更（§1 新增 source_files/build_command） |
| tests.md | `88e00fa82853f668` | 27 | 未变 |
| tasks.md | `5b1980dbbd9bd3b6` | 19 | 未变 |
| evidence.md | `db7a5a7cbc3bf7a2` | 15 | 未变 |
| feature-map.md | `0541f3f6e76fb730` | 17 | 由 `29ce1c51` 变更（auto 重生成投影，generated_at 10:44；仍全 `missing_entry` 种子） |
| restatement.md | `dc24c1c09fc88c32` | 21 | 未变 |

## 门核验（原始输出）

1. `goal status --run-id run_932e20c89ae9c375a7a5cab5` → running / stage=plan / blocked=false / executor=external / gate plan evaluated=false（exit 0）
2. `gate integrity --feature kernel-qa-evidence-hardening --evaluator qa-regression-checker --json` → verdict **pass**, mode=block, gate `gate_250223f55b529a2090a0e7de`（exit 0）
3. `fsm transition` 逐条实跑（reality trunk）：
   - 13 条合法：**13/13 allowed=true**
     awaiting_tests→tested, tested→verified, verified→eligible, awaiting_tests→rejected, tested→rejected, verified→rejected, eligible→rejected, rejected→awaiting_tests, awaiting_tests→superseded, tested→superseded, verified→superseded, eligible→superseded, rejected→superseded
     （含 `eligible→superseded`，上一版被反向子串误杀，本版已过）
   - 2 条非法：**2/2 allowed=false, verdict=illegal_transition**，reason=`Matches explicit illegal transition` → **结构性拒绝，不是语义 guard**
     awaiting_tests→eligible, tested→eligible
   - 创建边 `awaiting_tests|-` 未执行，按要求不作为普通 transition。

## AC ↔ 四约束

| 约束 | 落点 | 结论 |
|---|---|---|
| 候选 manifest hash | interfaces §1 | **无问题**（见 B1 关单） |
| 同 run 最新失败阻断 | interfaces §2/§3、AC-003 | 无问题 |
| 新 invocation 路径 | interfaces §2（每次 invocation 新目录，不复用旧 report/JUnit） | 无问题 |
| report evidence hash | interfaces §3（evidence_refs 存在文件且登记 hash/check freshness） | 无问题 |
| 系统信任边界 | interfaces §1/§3（本机声明留痕，不冒称认证/不声称 hash 证明编译器） | 无问题 |

四 AC 覆盖：AC-001→restatement §1、AC-002→§2、AC-003→§3、AC-004→§3+§5，与用户确认四约束一一对应，无缺项。

### B1 关单核验（source_commit）

interfaces §1 现具备全部要素：
- `source_commit` 必须指向实际实现提交；`source_files` 非空且为仓内非 docs/evidence 源码；
- 每文件 `git show <commit>:<path>` 字节摘要 与 当前工作树摘要 均与 manifest 一致；
- 至少一项在该实现提交中发生变化；**纯 QA 文档提交不得代替实现提交**；
- 非 git 项目用源码快照、source_commit 留空，不伪造提交；
- `build_command` 非空记录构建来源；binary 不入 git、只按固定文件 hash 绑定，**不声称源码摘要可证明编译器产出**；
- Go 产物有 VCS 元数据时核对 revision/modified；`-buildvcs=false` 须显式记录限制；
- 独立 QA 核对同一源码与构建记录，不把 manifest 登记当编译可复现证明。

→ **B1 已闭合，无自引用**（候选身份=登记文件 hash，合同不引用自身摘要作为证明）。

### B2 关单核验（FSM）

Illegal 仅剩 2 条且均无出边方向冲突；`superseded` 无出边，`superseded→eligible` 天然被结构拒绝，无需显式禁；13 合法全过，反向子串误杀消失。→ **B2 已闭合**。

### G3（原非阻塞，已澄清）

truth frontmatter `status: draft` / `intent_confirmed: false` 属官方 confirm 之前状态，非代签；管理方材料通过后正式 confirm/freeze，不得手填 true。**不作为缺陷**。

### O1（非阻塞，本轮顺带观测，未扩范围）

`beacon-go gate integrity` 的 `--feature` 标为 required，但缺省调用未报错，改在 `acceptance-seed-red-test` 上落了一条 pass 记录（`gate_7c672068a734f3777930ae80`）。属 CLI 必填参数未强制，建议后续单独票处理；不影响本包合同结论。

## 未验清单

- 产品测试真实执行（`TestKernelQA*` 四个入口尚未实现，`feature-map` 全 `missing_entry`；无可匹配 PASS，不伪通过）
- 产品 QA、release 门（超出 contract_only）

## 最终结论

**MATERIAL PASS**（scope=contract_only；无阻塞项；B1、B2 均已闭合；O1 为非阻塞观测）
