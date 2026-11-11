---
slug: kernel-autotest-shell-tc
version: v2.0.0
status: draft
revision_id: R3-draft
---

# Tasks

全部未勾选；plan 阶段只产出 truth，不写业务代码。所有路径以 beacon-go 仓库根为基准。

- [ ] TASK-SH-01 冻结 truth：确认 AC-001 到 AC-005、解释器选择、证据字段与聚合规则 · ac=AC-001,AC-002,AC-003,AC-004,AC-005
- [ ] TASK-SH-02 实现 shell TC 识别：.sh、.zsh、无扩展名 shebang；显式解释器与 PATH 命令行为不变 · ac=AC-001
- [ ] TASK-SH-03 相对脚本路径按项目根解析，保留含空格路径为单一 argv 元素 · ac=AC-002
- [ ] TASK-SH-04 保留真实 exit code 与 stdout、stderr、result.json 证据 · ac=AC-003
- [ ] TASK-SH-05 保持聚合门：total 大于 0 且全绿才写 qa passed；失败、blocked、空集不写 · ac=AC-004
- [ ] TASK-SH-06 回归五条不可协商规则：BLOCKED、空命令 fallback、无 job 硬拒绝、shell 语法拒绝、门禁真实事件 · ac=AC-005
- [ ] TASK-SH-07 seed1 与 seed2 转绿，seed3 保持守护绿；三条 seed 与 sha256 复跑一致 · ac=AC-001,AC-003,AC-004,AC-005
- [ ] TASK-SH-08 go test ./... 全绿；只在 beacon-go 改动；不引入 shell · ac=AC-001,AC-005
- [ ] TASK-SH-09 freeze 前置检查（本阶段不执行 freeze） · ac=AC-001,AC-002,AC-003,AC-004,AC-005

