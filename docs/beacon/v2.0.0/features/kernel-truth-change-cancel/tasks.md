---
slug: kernel-truth-change-cancel
materials_status: stale
version: v2.0.0
status: draft
revision_id: R1-draft
---

# Tasks

全部未勾选；plan 阶段只产出 truth，不写业务代码。所有路径以 beacon-go 仓库根为基准。已冻结的 truth/tests/tasks/evidence/seeds 在本 feature 内只读。

- [ ] TASK-CAN-01 冻结 truth：确认 AC-001 到 AC-005、三支语义、事件字段与退出码 · ac=AC-001,AC-002,AC-003,AC-004,AC-005
- [ ] TASK-CAN-02 CLI：`truth change` 增加 `--cancel` 与 `--accept-current`；带 `--cancel` 时取消 `--summary` 必填，非 cancel 路径保持原校验 · ac=AC-001,AC-003
- [ ] TASK-CAN-03 引擎：新增 `CancelTruthChange(ctx, acceptCurrent, by)`，解析唯一挂起变更与其 `base_truth_sha256`，与当前正文 sha256 比对 · ac=AC-001,AC-002,AC-003
- [ ] TASK-CAN-04 分支实现：一致则关闭；有证据不一致则拒绝并指向 `truth refreeze`；基线缺失则要求 `--accept-current`，带旗标时以当前正文关闭 · ac=AC-001,AC-002,AC-003
- [ ] TASK-CAN-05 幂等：无挂起变更时退出 0、stderr 为空、不写事件、状态不变 · ac=AC-004
- [ ] TASK-CAN-06 追加事件：写 `truth_change_cancelled`（change_id、decision、accepted_current、baseline_missing、base_truth_sha256、truth_sha256、change_doc、by）；不删历史、不改正文、不删变更文档 · ac=AC-005
- [ ] TASK-CAN-07 挂起扫描器：`pendingChangeIDs` 与 `trailRecords` 同时把 `truth_change_cancelled` 当作清空事件，保证 `truth show` 与门禁一致 · ac=AC-005
- [ ] TASK-CAN-08 单测：引擎三支 + 幂等 + 缺基线 + 追加事件不变量；CLI 标志组合（`--accept-current` 单独出现报错） · ac=AC-001,AC-002,AC-003,AC-004,AC-005
- [ ] TASK-CAN-09 seed1/seed2/seed3 全部转绿；三条 seed 与 sha256 复跑一致 · ac=AC-001,AC-002,AC-003,AC-004,AC-005
- [ ] TASK-CAN-10 `go test ./...` 全绿；只改 `cmd/**` 与 `internal/**`；不 push、不发布 · ac=AC-001,AC-005
