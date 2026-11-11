---
slug: kernel-readonly-http-api
version: v2.0.0
status: draft
revision_id: R2-draft
materials_status: current
---

# Tests: kernel-readonly-http-api

当前阶段为 plan（HEAD `36c40b6`）。三条 seed 全部由 `go build ./cmd/beacon`
的产物驱动，纯 shell（zsh）+ `jq`/`curl`/`lsof`/`sqlite3`，不调用
Python v1 CLI，也不引用 Python v1 仓。

## 复跑方式

```bash
cd /Users/apple/Developer/Personal/products/beacon-go
go build -o /tmp/beacon-ro-api ./cmd/beacon
BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed1_default_off.sh; echo exit=$?
BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh; echo exit=$?
BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed3_no_write_surface.sh; echo exit=$?
```

每条 seed 自建 `BEACON_STORE_PATH` 临时库与临时项目，不触碰 `~/.beacon`
或仓库 `.beacon/state`。三条 seed 在 HEAD `36c40b6` 上均 `exit=1`。

## Seed 契约（当前必然 FAIL）

| TC ID | 覆盖 AC | 命令 | 当前失败现象 | canon sha256 |
|-------|---------|------|--------------|--------------|
| TC-001 | AC-HTTP-001 | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed1_default_off.sh` | `goal serve` 不是子命令，cobra 打印 goal 帮助并 exit 0；`disabled_notice=no`，无监听但没有任何 disabled 说明 | `c5dbbb5e7ca5edf2a247e0c1a72ee801842832635aa79ba32d9f3e4487c8c592` | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed1_default_off.sh` |
| TC-002 | AC-HTTP-002..006 | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | `goal serve --http` 立即退出：`Error: unknown flag: --http`，`serve_started=no`、`serve_stderr=unknown_flag`，五个端点均无法探测 | `026aa9655e43ebdea7eac49c6315a960b56cbf1924efaeb17120e38fa620769c` |
| TC-003 | AC-HTTP-007..008 | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed3_no_write_surface.sh` | 同上，`serve_started=no`；405/404 矩阵与只读计数断言无法执行 | `89dca9db3ff1af172a6236c22812831d8a0adea330e0114fab9d2e9be67f46da` |

canon 内容已去掉随机端口与耗时，连续两轮复跑哈希一致；canon 原文见
`docs/beacon/v2.0.0/features/kernel-readonly-http-api/evidence/plan-seed-red.log`。

## 修复后行为契约

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-HTTP-001 | TC-001 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed1_default_off.sh` | exit=0、5s 内退出、输出含 `disabled`、8787 的 LISTEN pid 集合不变 |
| AC-HTTP-002 | TC-002 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | 只有一个 LISTEN socket 且地址为 `127.0.0.1:PORT`；非回环拒绝并不留监听；8787 空闲时验证默认地址 |
| AC-HTTP-003 | TC-002 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | `/health` 200、`application/json`、键集合 `{status, version, read_only}`、`status=ok`、`version` 非空、`read_only=true` |
| AC-HTTP-004 | TC-002 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | `/spine/runs` 键集合 `{runs, count}`；每个 run 键集合固定；`count == len(runs)`；`started_at` 倒序、同刻 `run_id` 升序 |
| AC-HTTP-005 | TC-002 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | `/spine/gates?feature=demo` 键集合 `{feature, gates}`；gates 数量 2、键集合固定、`evaluated_at` 倒序；缺参 400 `bad_request` |
| AC-HTTP-006 | TC-002 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | `/runs/{id}` 顶层 `{run}`、run 七字段；`/features/{slug}` 顶层 `{feature}`、四字段；未知 id/slug 404 `not_found` |
| AC-HTTP-007 | TC-003 | negative | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed3_no_write_surface.sh` | 五个已知路径 × POST/PUT/PATCH/DELETE = 20 次均 405 `method_not_allowed`；6 条 release/publish/complete 路径 × GET/POST = 12 次均 404 `not_found` |
| AC-HTTP-008 | TC-003 | behavior | `BEACON_BIN=/tmp/beacon-ro-api zsh docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed3_no_write_surface.sh` | 请求前后 `runs` / `feature_packages` / `gate_results` / `lifecycle_events` 计数一致；SIGTERM 后监听消失 |
## Fixture

每条 seed 通过 `docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/lib.sh` 建立同一份临时夹具：

- 注册临时项目（`projects register`），`truth create` 生成 `demo` 包后标记 `status: frozen`；
- 两次 `gate evaluate truth_freeze`，得到两条 `evaluated_at` 不同的 gate；
- 先后创建两条 manual goal run（A、B），`started_at` 不同，用于验证倒序与 `run_id` 次序。

## 负向断言

- 非 GET 不得返回 2xx/3xx，且错误体必须是 `{"error":{"code":"method_not_allowed","message":...}}`。
- `/release`、`/publish`、`/complete`、`/runs/{id}/release`、`/runs/{id}/complete`、`/features/{slug}/publish` 不得返回 2xx。
- `/spine/gates` 缺 `feature` 不得返回 200。
- 未知 run id、未知 feature slug 不得返回 200。
- 全部请求不得改变运行库行数。

## 证据文件 sha256

| 文件 | sha256 |
|------|--------|
| `docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/lib.sh` | `b196159af08d37d0373f4053acd57264b81b231639349530f00bfb50146fd72e` |
| `docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed1_default_off.sh` | `45152dde271107f43b4d420dff1f1a8bb9ffb3c3b5edc6562c4886e91777a7ea` |
| `docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed2_loopback_schema_order.sh` | `62db7d50d62f0dd3b9d5c0e28334e52acd685b3ab0e935555420264124c5d33a` |
| `docs/beacon/v2.0.0/features/kernel-readonly-http-api/seeds/seed3_no_write_surface.sh` | `4edaa29e8b32e3884c5636410e05d0e1b2b76d6be2e8c444cf35bed9b7436b4f` |
| `docs/beacon/v2.0.0/features/kernel-readonly-http-api/evidence/plan-seed-red.log` | `89af04c635c224444a2c80008e2e8a80d61f22ebe0acca72978f8aa8d478f609` |

## 环境说明

- 默认端口 `8787` 在本机已被无关进程占用（`lsof -nP -iTCP:8787 -sTCP:LISTEN`）。
  seed2 的默认地址子检查在该端口忙碌时记为 `default_addr=skipped`，不伪造结果；
  其余回环绑定与五端点断言仍强制执行。
- seed1 不依赖 8787 空闲：它比较命令前后的 LISTEN pid 集合，只断言不新增监听。
