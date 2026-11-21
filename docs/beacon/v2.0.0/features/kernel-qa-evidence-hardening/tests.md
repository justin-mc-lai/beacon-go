---
slug: kernel-qa-evidence-hardening
version: v2.0.0
materials_status: current
---

# Tests: kernel-qa-evidence-hardening

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/cli -run '^TestKernelQAExecutionEvidence$' -count=1 -v` | 真实CLI合法执行通过；空跑/skip/缺必跑/缺机器结果拒绝；Go/JUnit/自定义适配同判定。 |
| AC-002 | TC-002 | reject | `go test ./internal/cli -run '^TestKernelQACandidateBinding$' -count=1 -v` | 真实CLI登记候选并执行；错run/产物篡改/需求变化/新候选/跨run拒绝。 |
| AC-003 | TC-003 | reject | `go test ./internal/cli -run '^TestKernelQAIndependentReview$' -count=1 -v` | 同候选独立产品结论可通过；合同PASS、自验、缺绑定、最新FAIL被旧PASS遮掩拒绝。 |
| AC-004 | TC-004 | behavior | `go test ./internal/cli -run '^TestKernelQAUnifiedGate$' -count=1 -v` | formal/exit/manual/external/driver/receive-result适用入口一致，正常成功和拒绝链均实际到达目标门。 |

以上为待实现acceptance seed，不把当前无匹配测试的exit0算验收。独立QA必须核具名用例实际执行、候选hash与源码commit；留stdout、stderr、返回码、持久效果。每TC包含相关FSM非法跳步负控。

## 必跑场景

1. TC-001：Go -json至少一个真实pass，JUnit testcase与汇总一致，自定义有名称和正断言数；命令pass、Go no tests、全部skip、缺一个required case、错误格式、复用旧机器结果均非通过。
2. TC-002：两run同feature，A证据不得替B；同run候选1→2后QA1失效；修改产物、truth/tests后旧证据失效；未知commit、不存在path、错hash登记拒绝。
3. TC-003：同候选执行完整，maker!=checker product/pass完成当前TC覆盖；scope=contract、纯文字PASS、checker=maker、缺一个TC、最新fail后仍有旧pass、未注册或篡改报告全部拒绝。
4. TC-004：先在隔离DB跑一条完整合法产品QA链和一条空跑拒绝链，再覆盖其余入口；仅tick --by、文件存在、stage_executed、非空autotest目录不得替代有效QA。complete不得用旧formal pass绕过当前证据变化，人工release门仍拒绝无批准。

## 分层

CLI用临时项目/独立BEACON_STORE_PATH和候选命令，不改默认DB。源码helper回归用于细分解析；独立验收须运行固定候选的真实CLI。默认回归不发模型请求、平台事件或安装。旧released只读fixture不重写；活跃链路fixture按强合同更新，不能关强校验凑旧绿。
