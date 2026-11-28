# Stable workflow review checkpoint

## 已处理的两轮审查

1. 第一轮 BLOCK：prepare 被当成确认；来源与目标一对一导致 feature AC 下沉失败。
   修法：独立 confirm，显式身份与 expected prepared hash，绑定合同/source/HTML/manifest；来源 AC 各覆盖一次，同文多来源可归并目标。
2. 第二轮 BLOCK：deprecated 子树在不同阶段使用不同集合；confirm 多次读取可能将 A 的预期批准写给 B。
   本轮修法：验证、scope、truth/implement 共用活跃集合；HTML 与合同指纹保留墓碑，不要求废弃 draft 的 AC 完整；拒绝零活跃目标。
   bind/prepare/confirm 使用同 run 跨进程写锁；confirm 只写首次捕获且再次比较 expected hash 的产物 A。
   使用两个执行者同步屏障，证明确认 A 与并发 prepare B 不会把 A 的批准套到 B。

## 协作边界

- 本 lane 修改 workflow 专用文件、tree_stage 活跃集合判断和测试。
- lineage lane 独占 disk 与 tree_stage 路径解析段；本 lane 不修改这些解析段。
- 不修改真实 canonical 树，不提交。

## 既有验证

上一轮 go build/vet、go test ./... -count=1、E2E、版本与内核依赖检查均通过。
本轮结果需按实际执行另行汇报，不沿用上一轮结论。

## 本轮实跑结果

- 定向 workflow（含跨进程 barrier）通过。
- `go test ./... -count=1` 全包通过；CLI 92.608s，orchestrator 121.528s。
- `go build ./...`、`go vet ./...` 通过。
- version-consistency: ok (0.0.1)；kernel-deps: ok。
- E2E RESULT: PASS，run `run_e74aa2843121dba6ad376df4`。
- 并发证明：confirm 捕获 A 后持有现有 project program flock，第二 OS 进程非阻塞取锁收到 EWOULDBLOCK 并到达 stdout barrier；confirm 只落 A，随后 prepare B，复验 B 未确认。
- 锁复用现有 project 级 program.lock（比同 run 更严格，跨 run 也串行），未引入第二锁实现。
