# 内核QA证据合同 v1

## 范围

修复现有CLI和账本，不新开调度库。所有本次及后续主动QA评估/推进采用强校验，不因旧包未声明surface_strict而豁免。已completed/released历史查询原样，不能为过新门伪造回填。现有门人工批准语义不变。

## 1. 当前候选

沿用 `goal artifacts register --run-id R --kind qa_candidate --path candidate.json --by maker`。候选JSON：

```json
{
  "schema": "beacon-qa-candidate.v1",
  "run_id": "R",
  "feature_slug": "slug",
  "truth_sha256": "当前truth文件摘要",
  "tests_sha256": "当前tests文件摘要",
  "source_commit": "确切实现commit",
  "source_files": [{"path": "internal/orchestrator/example.go", "sha256": "实现源码文件摘要"}],
  "build_command": "go build -o /absolute/candidate ./cmd/beacon",
  "maker": "maker",
  "artifacts": [{"path": "/absolute/candidate", "sha256": "产物摘要"}]
}
```

登记时校run/feature、所给摘要与磁盘、git commit存在；artifact至少一个普通文件。候选身份=登记文件hash；同run最新qa_candidate登记事件决定current candidate，不能用mtime猜。修改已登记manifest不重新登记视为stale，替换须新登记并保留历史事件。后续所有机器结果/独立报告绑定该hash。当前truth/tests或任一产物变化就无资格；不以“只是文档改了”搬运旧QA。注册器是受信任本机CLI，by是声明留痕不是认证服务。

source_commit必须指向实际实现提交，source_files非空且为仓内非docs/evidence源码：每个文件的git show <commit>:<path>字节摘要及当前工作树字节摘要与manifest一致；至少一项在该实现提交中发生变化，纯QA文档提交不得代替实现提交。非git项目明确采用源码文件快照、source_commit留空，不伪造提交。build_command非空记录构建来源；编译产物只按固定文件hash绑定，不要求二进制进入git，不声称源码摘要可从数学上证明编译器产出。Go产物存在VCS元数据时，核对其revision/modified与声明的构建来源，若用-buildvcs=false则明确记录此限制。独立QA核对同一源码与构建记录，不把仅登记manifest当编译可复现证明。

## 2. 实际执行结果

tests.md矩阵保留既有列；可加 `Required Tests` 列，逗号分隔精确case名，未填时仍要求实际执行>0。本包4TC分别声明对应具名Go入口；最低要求不是代码覆盖率百分比。

qa autotest保留原调用，执行前取得current candidate，并在每次invocation新目录产生result/raw日志，不复用旧report/JUnit。测试源码/脚本只能在本机受信任仓内执行。

结果适配优先：
1. Go test `-json` 的run/pass/fail/skip事件，case名必须真实出现run且以pass结束；无Test的package PASS不算执行用例。也支持既有`-v`命令的具名RUN/PASS/SKIP解析或内部追加`-json`（选择一种实现并测试）。
2. 新鲜 `BEACON_JUNIT_PATH` 的JUnit XML，testsuite/testcase实际case节点与总数一致；tests>0，failures/errors/skipped=0；不存在case节点的空汇总不算证明。
3. 脚本写 `BEACON_TEST_RESULT_PATH` 的JSON：

```json
{"schema":"beacon-test-execution.v1","run_id":"R","tc":"TC-001","candidate_hash":"候选hash","cases":[{"name":"case","status":"passed","assertions":1}]}
```

环境提供run/TC/candidate hash（`BEACON_QA_CANDIDATE_HASH`）和新结果路径。custom cases唯一、至少一个、每条assertions>0，声明required项全出现。三种适配只证明runner声明的机器执行，不能声称内核证明业务断言语义。JUnit计数命名为test_count，不把tests属性叫assertion_count。

无有效适配结果、总执行0、任意skip/fail、缺required项、命令非0/超时均不通过；有命令exit0但证据无效用明确reason code阻断。产物/manifest/合同在执行前后都核验；report带candidate_hash及机器case详情。测试不通过不写passed stage；最新invocation失败不能被旧report或旧stage成功遮住。

## 3. 独立产品结论

沿用artifact登记kind=qa_report，要求JSON结构：

```json
{"schema":"beacon-product-qa.v1","run_id":"R","candidate_hash":"候选hash","scope":"product","verdict":"pass","maker":"maker","checker":"different-checker","covered_tcs":["TC-001"],"evidence_refs":["/absolute/raw.log"]}
```

checker非空、不同于candidate maker、run executor（已设置时）；maker和候选一致；covered_tcs包含tests.md所有TC，不以只验一条扩为全量。evidence_refs至少一个存在文件且登记其hash/检查freshness。注册者应为checker；身份是本机声明边界，不引证书体系。

同candidate最新qa_report登记顺序决定结论。最新fail即阻断，不能择旧pass；新候选的旧报告不参与有效性。scope=contract/其它、纯文字PASS、缺字段、空证据、错run/hash和被篡改文件均不满足产品QA。attach-verifier refs必须指向当前有效报告/其证据，不能默认source字符串+任意md就通过；报告保留fail审计不必拒绝其登记，只拒绝其放行。

## 4. 唯一判断与推进

抽一个最小共享当前QA事实/资格计算（复用Engine与artifact/events），formal EvaluateGate qa、ExitGate/Tick QA、driver最终tick、manual/external、适用receive-result和Complete都调用；不得另建第二个evidence database。formal pass后文件变化仍要拒绝stage exit/complete，不能只读旧gate_results快照。

正式QA gate必须有当前candidate、最新执行全TC通过、最新独立产品report通过、可核verifier以及surface要求（已声明时）；非空目录、artifact数量、stage_executed仅审计，不能单独放行。实现阶段回执不等于QA回执。receive-result保持实现收活能力，不额外阻塞与QA无关的实现记录，但它的whole-lake completion/QA声称不能跳过校验。失败不清除既有hard blocker，恢复仍走显式recover。

## 5. 向后兼容与非目标

用户过去登记的普通md仍可作为附件/历史证据读取，不能在新主动QA中充产品报告。历史旧单查询不重判改写，新的评估写新reason记录。更新旧测试fixture使其提供真实机器结果+候选+独立报告，不能使用legacy开关、关闭门或批量改expected PASS为BLOCKED掩盖正常链损坏。全量E2E必须有强合同成功链和拒绝链，不能“旧基线也失败”当新链成功。

## 6. 验收输入

先用隔离DB的真实CLI登记候选→运行至少一个实际case→登记独立QA→formal QA成功；另一条 `python3 -c 'pass'` 退出0必须拒绝。随后扩到Go/JUnit/custom、无verifier、cross-run、篡改、换候选、latest fail、合同scope及其它推进入口。固定同一候选binary实测，不只调helper。全部原始结果留包内evidence或独立候选目录并登记；不触碰默认DB做负控。
