# 变更记录：2026-10-29 全仓审查缺陷收口

> 触发：整体审查发现「假验资、账本分裂、冻结真相与代码脱节、AC 账本未闭环」四类问题。
> 状态：代码、冻结包、AC 矩阵三层已同步收口；`go test ./...` / `e2e` 全绿。未提交、未打 tag（按仓库纪律需人工确认）。

## 1. 代码级修复

| 问题 | 修复 | 证据 |
|---|---|---|
| `qa verify-harness` 无条件返回 passed（常量哈希、不校验 slug） | 改为真验资：从账本取 QA 工件、校验 qa 阶段执行记录、逐件复核内容哈希；缺证据/被篡改/未知 feature 一律非零退出并带 reason code，且不写 gate 结果 | `internal/orchestrator/harness_verify.go`；`internal/cli/cli_test.go:TestHarnessVerifyJSON`（未知 feature / 无证据 / 真证据通过 / 篡改后失败四个分支） |
| 默认账本 `~/.beacon/beacon.db` 与文档 `beacon-go.db` 分裂，桌面与 CLI 会打开空库；且默认路径与 Python v1 同文件 | `store.DefaultPath` 默认改为 `~/.beacon/beacon-go.db`（v1 的 `beacon.db` 不再被 Go 轨打开） | `internal/store/store_test.go:TestDefaultPathIsGoTrackLedger`；`beacon spine list --project .` 默认即可读到本仓 5 个包 |
| 迁移前无备份（AC-092） | `store.BackupTo` 用 `VACUUM INTO` 生成一致性快照；`ImportV1` 非 dry-run 先备份并写入报告；dry-run 不落任何东西 | `internal/store/store.go`；`internal/migration/migration_test.go:TestImportBacksUpLedgerFirst`、`TestDryRunWritesNoBackup` |
| 长任务无法被 Ctrl-C 安全中断（AC-105 信号部分） | `cmd/beacon` 用 `signal.NotifyContext`；`DriverRunLoop` 在轮次间停下并记 `host_interrupted`，run 保持可恢复 | `cmd/beacon/main.go`；`internal/orchestrator/driver_test.go:TestDriverRunLoopStopsOnInterrupt` |
| 流式实现只转发原始读缓冲，一次可能只有一个 chunk（AC-STREAM-002 实际不达标） | 改为解析 SSE `data:` 行，按 delta 回调；非 SSE 响应整体回退；usage 从事件读取 | `internal/llm/openai_compatible.go`；`internal/llm/llm_test.go:TestStreamCompleteChunks`、`TestStreamCompleteNilHandlerFallsBack` |
| benchmark 只有基线、没有回退门禁（AC-095） | 新增 `internal/benchgate` 比较门禁 + `beacon doctor bench-gate`；跨 OS/CPU 数拒绝判定；接入 `run_bench.sh` 与 CI | `internal/benchgate`；`TestOverToleranceFails`、`TestCrossEnvironmentIsNotJudged` |
| skill 的 route examples 不在 SKILL.md 里（AC-127/144） | 六个 SKILL.md 增加 Route examples 段（引用真实 `examples/*.md`，声明不是 requirement authority）；新增 `beacon skills examples` | `skills/*/SKILL.md`；`skills/contract_test.go:TestRouteExamplesCoverFourChains` |
| 13 个非测试文件 >500 行且无 ADR（AC-025） | 逐条 ADR 记录例外 + CI 拦截新增超限文件 | `docs/adr/ADR-0001-file-size-exceptions.md`；`scripts/check_file_size.sh` |
| 冻结包可停留在 `materials_status: draft` | 冻结/重冻时把 stale 与 draft 一并归位 `current`（`truth_sha256_before` 仍取归位前字节） | `internal/orchestrator/materials.go`、`truth.go`；`TestTruthFreezeSettlesMaterialsToCurrent` |
| 分发无流水线 | 新增 tag 触发的 release workflow，产出四平台 CLI 二进制 + macOS 桌面包 + SHA256 | `.github/workflows/release.yml` |

## 2. 冻结真相收口（change → 重写 → refreeze）

| 包 | 处置 |
|---|---|
| `agent-runtime-dual-mode` | AC-AGENT-005（Web Console）按 2026-10-28 决定改为「单一桌面界面」；命令名从 `beacon harness verify` 改为 `beacon qa verify-harness`；证据要求从 `evidence.json` 改为已登记工件 + 哈希；AC 编号改为机器可识别的 `AC-00x`；补齐 tests/tasks/evidence 并由内核冻结 |
| `agent-desktop-workspace` | AC-DESKTOP-002 从 `/api/workspace/*` 改为 agentapi 账本读模型；证据表换成真实存在的测试；tasks 里 `--web` 的过期描述删除 |
| `agent-session-streaming` | 补齐 `tasks.md`、`evidence.md`；SSE 表述与 Wails 事件实现对齐；AC 编号改为 `AC-00x` 并冻结 |
| `p0-executor-desktop` | `package_maturity: draft → filled`、`materials_status: draft → current`；证据表改用测试名，去掉未登记路径引用 |
| `p1-skill-router-acp` | 8 条任务全部按真实测试勾选；证据索引从 draft 改 frozen；移除「仅两步探针」的过期描述 |

`beacon truth list --project .` 现在 5/5 为 `frozen/Freeze`。

## 3. 账本口径更新

- `docs/ac-matrix.md`：88 tested / 62 implemented / 12 deferred-external / 9 dropped-by-charter（共 171）。
- `docs/deferred-external.md`：移除 AC-092、AC-095、AC-127、AC-144（已用测试收口）；AC-105 仅剩厂商二进制多轮一项。
- `docs/ac-matrix-part1/2.md` 标注为历史分片，权威指向 `docs/ac-matrix.md`。

## 4. 仍未闭环（外部条件，非代码）

- 真 codex / claude 二进制的多轮成功证据（AC-043/045/105）：本机模型不可用。
- 真实 ACP agent 目标（AC-046）：协议路径已用脚本化 agent 验证。
- v1 账本样本（AC-009/077/078）与 Chrome 扩展 e2e：样本/宿主未出现。
- AC-157 双轨选轨面：仍需产品拍板（独立名或 `--runtime`）。

## 5. 验证（真实输出见交付回报）

```
gofmt -l .            # 空
go build ./...        # 0
go vet ./...          # 0
go test ./... -count=1# 全部包 ok
bash e2e/run_e2e.sh   # E2E RESULT: PASS
bash scripts/check_file_size.sh          # file-size guard: ok
beacon doctor bench-gate --baseline ...  # 7 条 AC 全 ok
```
