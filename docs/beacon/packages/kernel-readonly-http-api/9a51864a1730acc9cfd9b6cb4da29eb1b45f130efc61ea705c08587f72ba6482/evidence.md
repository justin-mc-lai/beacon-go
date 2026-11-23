---
slug: kernel-readonly-http-api
version: v2.0.0
status: draft
revision_id: R2-draft
---

# Evidence

## Plan stage (2026-11-11, R2, beacon-go 根)

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v2.0.0/features/kernel-readonly-http-api/truth.md | draft，未冻结 | truth |
| tests | test_truth | docs/beacon/v2.0.0/features/kernel-readonly-http-api/tests.md | 3 条 shell seed + TC 矩阵 | qa |
| tasks | support_advisory | docs/beacon/v2.0.0/features/kernel-readonly-http-api/tasks.md | 未勾选 | truth |
| evidence | support_advisory | docs/beacon/v2.0.0/features/kernel-readonly-http-api/evidence.md | 本文件 | truth |
| seeds | support_advisory | docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed1_default_off.sh | 可执行，go-build 驱动 | qa |
| seed log | support_advisory | docs/beacon/v2.0.0/features/kernel-readonly-http-api/evidence/plan-seed-red.log | HEAD `36c40b6` 实跑 | qa |

## 单一真源

- 仓库：`/Users/apple/Developer/Personal/products/beacon-go`，HEAD `36c40b6`。
- 二进制：`go build -o /tmp/beacon-ro-api ./cmd/beacon` → `Beacon CLI v2.0.0-go.15`。
- 本包不再包含 Python seed；上一版的 Python 用例已移出仓库，包内只留 shell seed。
  全包检索无 Python v1 仓引用、无 Python CLI 调用。

## Seeds (currently red)

`BEACON_BIN=/tmp/beacon-ro-api`，每条 seed 使用独立临时项目与
`BEACON_STORE_PATH`，不触碰全局账本目录或仓库的 .beacon 状态目录。

| Seed | exit | canon sha256 | Why red |
|------|------|--------------|---------|
| `seed1_default_off.sh` | 1 | `c5dbbb5e7ca5edf2a247e0c1a72ee801842832635aa79ba32d9f3e4487c8c592` | `goal serve` 不是子命令：cobra 打印 goal 帮助并 exit 0，`disabled_notice=no`；未新增监听但缺少 disabled 说明 |
| `seed2_loopback_schema_order.sh` | 1 | `026aa9655e43ebdea7eac49c6315a960b56cbf1924efaeb17120e38fa620769c` | `goal serve --http` → `Error: unknown flag: --http`，`serve_exit=1`，五端点不可探测 |
| `seed3_no_write_surface.sh` | 1 | `89dca9db3ff1af172a6236c22812831d8a0adea330e0114fab9d2e9be67f46da` | 同上 `serve_stderr=unknown_flag`；405/404 与只读计数断言无法执行 |

原始输出见 docs/beacon/v2.0.0/features/kernel-readonly-http-api/evidence/plan-seed-red.log 。
（sha256 `89af04c635c224444a2c80008e2e8a80d61f22ebe0acca72978f8aa8d478f609`）。

## Fixture

`docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/lib.sh` 的 `fixture_init` 建立：临时项目 → `truth create demo` →
标记 `status: frozen` → 两次 `gate evaluate truth_freeze` → 先后两条 manual run。
seed2 用它验证 gate 倒序与 run 倒序；seed3 用它比较请求前后的表行数。

## 环境限制（如实记录）

- 本机 `8787` 已被无关进程监听，seed2 的 `--http` 默认地址子检查记为
  `default_addr=skipped`；显式 `--addr 127.0.0.1:<free port>` 的断言不受影响。
- 三条 seed 均已连跑两轮，canon 哈希一致；随机端口与耗时未写入 canon。

## Guardrails

- 生产代码零改动：cmd、internal、skills、bench、desktop、e2e、scripts、bin 八个目录均未触碰。
- 未 freeze、未 commit、未 push、未写任何 QA verdict。
- 新增/改动仅限本 feature 包内文件。
