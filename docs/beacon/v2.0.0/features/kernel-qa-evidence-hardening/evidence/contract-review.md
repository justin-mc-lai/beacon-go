# Material Contract Review: kernel-qa-evidence-hardening

- scope: **contract_only**（只读材料与关键调用点；未实现、未做产品 QA、未发布）
- reviewer: qa-regression-checker（独立材料 checker）
- run: run_932e20c89ae9c375a7a5cab5 · stage=plan · HEAD=bd0195a
- reviewed_at: 2026-11-21T10:40Z (Asia/Shanghai 18:40)
- feature: docs/beacon/v2.0.0/features/kernel-qa-evidence-hardening

## 1. 被读文件 sha256(16) + 行数

| 文件 | sha256[0:16] | 行数 | 与派单收据前缀一致? |
|---|---|---|---|
| truth.md | 9c9551eb7a513eb3 | 144 | ✗ 派单写 f1e477de（收据过期/不符） |
| tests.md | 88e00fa82853f668 | 27 | ✓ |
| tasks.md | 5b1980dbbd9bd3b6 | 19 | ✓ |
| evidence.md | db7a5a7cbc3bf7a2 | 15 | ✗ 派单写 4bdf1a25（收据过期/不符） |
| interfaces.md | c64b29a80ea6474a | 69 | ✓ |
| feature-map.md | 29ce1c51b9a30d35 | 17 | ✓ |
| restatement.md | dc24c1c09fc88c32 | 21 | ✓ |

注：truth.md / evidence.md 两个实际前缀与 MausBot 派单不符，本报告以磁盘实测为准。

## 2. 门核验（原始结果）

- `beacon-go goal status --run-id run_932e20c89ae9c375a7a5cab5`
  → running / stage=plan / blocked=false / gate plan evaluated=false / executor=external（exit 0）
- `beacon-go gate integrity --feature kernel-qa-evidence-hardening --version v2.0.0 --project . --evaluator qa-regression-checker --json`
  → **verdict=pass, mode=block**, gate_result_id=gate_ab6272a1586ea743baa4a13c, feature_id=feat_cc859403f775a0a5864480c0（exit 0）
- `beacon-go fsm transition` 真机按 To|From 逐条（见 §4）

## 3. 四 AC ↔ 用户确认四约束 覆盖

| 约束（restatement §5 / utterance） | 覆盖 AC | 判定 |
|---|---|---|
| 原始证据 / 实际执行 | AC-001（TC-001） | 覆盖 |
| 同 run 同候选绑定 | AC-002（TC-002） | 覆盖（但 B1：source_commit 语义未闭合，见 §5） |
| 唯一写者 + 独立检查者（maker≠checker） | AC-003（TC-003） | 覆盖 |
| 精确路径本地提交 / release 人工确认 | AC-002 source_commit + AC-004 + AC-003 origin 声明 | 覆盖（B1 影响“实现提交”精度） |

schema 可实施性 / 自引用：
- candidate JSON（interfaces §1）含 truth_sha256/tests_sha256/source_commit/artifacts；candidate 身份=登记文件 hash，文件自身不含自身 hash → **无自引用**，可实施。
- report JSON（interfaces §3）含 candidate_hash + evidence_refs，无自引用 → 可实施。

分项核对（用户点名五处）：
- **候选 manifest hash / source_commit=实现提交**：见 B1，未闭合。
- 同 run latest FAIL：interfaces §3「同candidate最新qa_report登记顺序决定结论。最新fail即阻断，不能择旧pass」＋§2「最新invocation失败不能被旧report或旧stage成功遮住」→ **无问题**。
- 新 invocation 路径：interfaces §2「每次invocation新目录产生result/raw日志，不复用旧report/JUnit」→ **无问题**。
- report evidence hash：interfaces §3「evidence_refs至少一个存在文件且登记其hash/检查freshness」「被篡改文件均不满足」→ **无问题**。
- 系统信任边界：interfaces §1「注册器是受信任本机CLI，by是声明留痕不是认证服务」＋§3「身份是本机声明边界，不引证书体系」＋AC-003「不冒称平台认证」→ **无问题**，边界已显式声明。

## 4. FSM 结构校验（真机 fsm transition，To|From）

13 条合法边（Domain FSM 行 90–102）：
- 通过 12/13。
- **错误拒绝 1 条**：`TO=superseded FROM=eligible` → verdict=illegal_transition，reason「Matches explicit illegal transition: illegal: superseded -> eligible」。实为合法边（truth 行 101「superseded | eligible | 新候选替代」），被 Illegal Transitions 行 128 的反向子串命中。
- 附带：创建边 `TO=awaiting_tests FROM=-` 也被误拒（reason 命中「awaiting_tests -> eligible」，因 `-` 子串命中 ` -> `）。此边不在 13 条计数内，但同属结构误判。

3 条非法边（行 126–128）：**全部正确拒绝**
- eligible ← awaiting_tests：拒绝（命中 awaiting_tests -> eligible）✓
- eligible ← tested：拒绝（命中 tested -> eligible）✓
- eligible ← superseded：拒绝（命中 superseded -> eligible）✓

结论：3 非法 = 结构性拒绝（非语义 guard），合格；但 1/13 合法边被同一子串匹配机制误判为非法 → 结构上不满足。

根因（只读关键调用点，未改）：`internal/cli/fsm.go:105-133 evaluateFSMTransition` 的 domain illegal 判定用
`strings.Contains(desc, lFrom) && strings.Contains(desc, lTo)`（第 124 行附近），**方向无关**。凡非法描述同时含某合法边的 from/to 状态名（如 `superseded -> eligible` 与合法 `eligible -> superseded`），合法边即被误杀。当前 truth.md 的 Illegal Transitions 措辞正好触发该坑。

## 5. Open Questions 逐条

- Q1：answered（human），无未决行。**无未答 OQ**。
- 一致性缺口 G3（材料级，非 OQ 行）：truth.md frontmatter `status: draft`、`intent_confirmed: false`，与 restatement.md `confirmed_by: justin / confirmed_at` 及 Q1 answered 矛盾。freeze 前需对齐（否则“已确认”与“未确认”两处冲突）。

## 6. 缺口清单（一次合并，不扩大范围）

- **B1｜source_commit 语义未闭合（阻塞）**：interfaces.md §1 仅校「run/feature、所给摘要与磁盘、git commit 存在」，**未要求** source_commit 所指向的实现提交树必须包含候选 artifact 的字节（sha256），也**未禁止**以只改 evidence/docs 的提交充当 source_commit。→ 与用户明确约束「report 的 source commit 应指向实现提交，不能用验收文档提交改变产物」直接冲突；可实现但可绕过。修法建议（供实现阶段，不在本票范围）：登记时校验 `git show <source_commit>:<artifact_path>` 的 blob sha256 等于候选登记值，且该提交非纯文档/evidence 提交。
- **B2｜FSM 合法边被自身非法清单误杀（阻塞）**：合法 `eligible → superseded` 被 `illegal: superseded -> eligible` 经方向无关子串匹配误判非法（真机 1/13）。需方向化匹配（checker）或改写非法清单措辞（材料）——二选一，本票只报不改。
- **G3｜frontmatter 与确认事实冲突（非阻塞但须 freeze 前对齐）**：`intent_confirmed: false` / `status: draft` vs 已确认 restatement + Q1 answered。

## 7. 边界声明

- 未实现、未运行产品测试、未做产品 QA、未注册为 verifier、未 attach、未 freeze/commit/push、未推进门、未碰旧 monitor。
- feature-map 四条 `missing_entry` 为未实现种子，按正常处理：**既不当伪通过，也不当缺陷**。
- 产品测试名（TestKernelQA*）尚未实现，无真实匹配 PASS → 不计为已验。
- 生产基线反例（docs/decisions/2026-11-21-kernel-qa-hardening.md）按要求未重跑。

## 8. 结论

**MATERIAL FAIL** — 阻塞项：B1（source_commit=实现提交未闭合）、B2（FSM 合法边被自身非法清单误杀）。附带 G3（frontmatter 确认态冲突）。未验清单：产品测试真实执行、产品 QA、release 门——均因未实现/超出 contract_only 范围未验。
