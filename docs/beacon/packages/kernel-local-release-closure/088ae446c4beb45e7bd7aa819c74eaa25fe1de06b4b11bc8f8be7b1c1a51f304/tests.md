---
slug: kernel-local-release-closure
version: v2.0.0
materials_status: current
---

# Tests: kernel-local-release-closure

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `python3 scripts/test_local_release.py manifest` | 用临时目录验证摘要绑定、证据及产物变更拒绝，不凭文档存在判通过。 |
| AC-002 | TC-002 | behavior | `python3 scripts/test_local_release.py install` | 临时安装目标验证备份、两安装者竞争、期望旧哈希过期拒绝及替换后中断恢复；保护工作文件。 |
| AC-003 | TC-003 | behavior | `python3 scripts/test_local_release.py status` | 模拟外部替换和删除，查询非零且历史不变，不自动修复未知产物。 |

命令是验收合同，脚本在实现后提供。独立 QA 另跑真实 CLI、Go 构建/全测/vet、E2E 并记录退出码；既有红灯须修正测试前置，不能跳门。Illegal transitions 由 TC-001/002 的未批准、过期批准和失败恢复用例覆盖。
