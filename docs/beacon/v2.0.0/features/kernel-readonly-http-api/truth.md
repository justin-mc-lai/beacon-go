---
slug: kernel-readonly-http-api
version: v2.0.0
status: frozen
revision_id: R2-draft
language: zh
change_reason: "R2: rewrite the plan package against the Go kernel repo; seeds drive the go-build binary and assert schema/ordering over a live loopback server"
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-readonly-http-api/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-readonly-http-api/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-readonly-http-api/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-readonly-http-api/evidence.md
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
truth_map_contract: beacon-truth-map-v1
domain_required: false
ux_required: false
package_maturity: draft
materials_status: current
intent_confirmed: true
---

# Requirement Truth: Read-only HTTP/JSON Kernel Surface (R2-draft)

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "给内核加只读 HTTP/JSON 服务面，让 OMB 能从轮询改成拉取"
goal: "beacon-go 提供默认关闭、仅回环的只读 HTTP API，暴露 health、spine runs、spine gates、runs/{id}、features/{slug} 五个 GET 端点，schema 与排序固定，非 GET 405，无任何写或 release 路径"
non_goals: "不做写操作、不做鉴权体系、不做 release/publish、不改 gate 语义、不改既有 CLI 输出"
constraints: "只读打开账本；默认不启动；只绑 127.0.0.1 或 ::1；不新增依赖；不 push 不发布"
scope_mode: focused
```

## 人话

内核需要一个只读 HTTP/JSON 面：本机脚本或面板可以通过固定的 JSON 契约查看
GoalRun spine（run、gate、feature）状态，不用解析 CLI 文本。它默认关闭，只有显式
`--http` 才监听，只绑回环地址，不提供任何写操作、release 或 publish 入口。

本包只定义读契约。运行时真源仍是内核自己的运行库；HTTP 层是只读投影，
不得成为第二个写入口。

## User Story

作为内核运维者，我希望用 `beacon goal serve --http --addr 127.0.0.1:PORT`
启动一个只读 JSON 接口，这样我可以在本机用脚本或面板查看 run、gate 和 feature
状态，同时接口不会改状态，也不会触发发布。

## User Intent (verbatim)

> 给内核加只读 HTTP/JSON 服务面：beacon goal serve --http --addr 127.0.0.1:PORT，
> 提供 GET /health、/spine/runs、/spine/gates?feature=、/runs/{id}、/features/{slug}；
> 只读、仅回环、默认关闭；不得暴露写操作或 release 能力

## 单一真源

- 仓库：`/Users/apple/Developer/Personal/products/beacon-go`（Go 内核）。
- 本 feature 包、seed 与运行时实现同属该仓库；seed 只驱动 `go build ./cmd/beacon`
  的产物，不引用 Python v1 仓。
- 运行库路径由 `BEACON_STORE_PATH` 指定；未设置时为全局 `~/.beacon/beacon-go.db`。
  seed 一律使用临时库，不触碰全局库。

## Scope

### In scope

- `beacon goal serve` 子命令：`--http` 开关、`--addr` 监听地址。
- 五个只读 GET 端点，JSON 字段集合固定。
- 默认不监听；非回环地址拒绝启动。
- 统一错误 JSON；已知路径的非 GET 方法返回 405；未知路径返回 404。

### Out of scope / Non-goals

- 任何写端点（POST/PUT/PATCH/DELETE），包括 tick / freeze / release / publish。
- 鉴权、TLS、远程绑定、CORS、分页、流式推送、WebSocket。
- 修改 truth 包、推进 run stage、写库、写 evidence。
- 替代现有 CLI；CLI 仍是完整能力面。

## CLI Contract

```bash
beacon goal serve                              # 不监听；打印含 disabled 的说明后退出 0
beacon goal serve --http                       # 监听默认 127.0.0.1:8787
beacon goal serve --http --addr 127.0.0.1:PORT
beacon goal serve --http --addr 0.0.0.0:PORT   # 拒绝，非 0 退出
```

| ID | Rule |
|----|------|
| CLI-1 | 只有显式 `--http` 才启动监听；缺省必须不绑定任何端口。 |
| CLI-2 | 无 `--http` 时在 5s 内退出 0，并输出一行包含 `disabled` 的说明。 |
| CLI-3 | `--addr` 缺省为 `127.0.0.1:8787`；`--http` 不带 `--addr` 时监听该地址。 |
| CLI-4 | `--addr` 的 host 必须是回环地址（`127.0.0.1` 或 `::1`）；否则拒绝启动并非 0 退出，且不留下监听。 |
| CLI-5 | 服务前台运行直到 SIGINT/SIGTERM；收到信号后释放监听。 |
| CLI-6 | 服务进程只以只读方式访问运行时库。 |

## HTTP Contract (common)

- 仅 `GET`；对已知路径使用其他方法返回 `405` + 错误 JSON。
- 所有响应 `Content-Type: application/json; charset=utf-8`（`application/json` 前缀）。
- 成功响应为对象；字段集合固定，不附加未声明字段。
- 未知路径返回 `404` + 错误 JSON。
- 错误统一为：

```json
{"error": {"code": "not_found", "message": "..."}}
```

`error` 对象字段集合固定为 `{code, message}`。已定义 code：
`not_found`（404）、`bad_request`（400）、`method_not_allowed`（405）。

## Endpoints

### GET /health

字段集合固定为 `{status, version, read_only}`：

```json
{"status": "ok", "version": "2.0.0-go.15", "read_only": true}
```

- `status` 恒为 `"ok"`；`version` 为非空字符串；`read_only` 恒为 `true`。

### GET /spine/runs

字段集合固定为 `{runs, count}`；`runs` 按 `started_at` 倒序、同刻按 `run_id` 升序。

```json
{
  "runs": [
    {"run_id": "run_...", "status": "Running", "current_stage": "plan", "blocked": false, "feature": "kernel-readonly-http-api"}
  ],
  "count": 1
}
```

- 每个 run 字段集合固定为 `{run_id, status, current_stage, blocked, feature}`。
- `count` 为整数且等于 `len(runs)`；`blocked` 为布尔；`feature` 为 feature slug。
- `current_stage` 为字符串或 `null`。
- `started_at` 为 RFC3339（纳秒精度，UTC），排序以其字符串值比较。

### GET /spine/gates?feature={slug}

`feature` 必填。字段集合固定为 `{feature, gates}`；gates 按 `evaluated_at` 倒序。

```json
{
  "feature": "kernel-readonly-http-api",
  "gates": [
    {"gate_id": "gate_...", "gate_type": "truth_integrity", "verdict": "pass", "evaluated_at": "2026-11-11T11:24:37.397124Z", "reason_codes": []}
  ]
}
```

- gate 字段集合固定为 `{gate_id, gate_type, verdict, evaluated_at, reason_codes}`。
- `reason_codes` 为字符串数组。
- 缺少或空 `feature` → `400` + `bad_request`；未知 feature → `404` + `not_found`。

### GET /runs/{id}

字段集合固定为 `{run}`；run 对象字段集合固定为
`{run_id, status, current_stage, blocked, feature, started_at, iteration}`。

```json
{
  "run": {
    "run_id": "run_...",
    "status": "Running",
    "current_stage": "plan",
    "blocked": false,
    "feature": "kernel-readonly-http-api",
    "started_at": "2026-11-11T11:24:37.397124Z",
    "iteration": 0
  }
}
```

- 未知 id → `404` + `not_found`。

### GET /features/{slug}

字段集合固定为 `{feature}`；feature 对象字段集合固定为
`{slug, status, truth_path, tests_path}`。

```json
{
  "feature": {
    "slug": "kernel-readonly-http-api",
    "status": "draft",
    "truth_path": "docs/beacon/v2.0.0/features/kernel-readonly-http-api/truth.md",
    "tests_path": "docs/beacon/v2.0.0/features/kernel-readonly-http-api/tests.md"
  }
}
```

- 未知 slug → `404` + `not_found`。

## Acceptance Criteria

每条 AC 都可由本包的一条 seed 或一条明确的命令判定；seed 断言的是可观察的
HTTP 响应、监听地址与退出码，不依赖实现内部结构。

| AC ID | Description | Must |
|-------|-------------|------|
| AC-HTTP-001 | 未显式 `--http` 时 `beacon goal serve` 在 5s 内退出 0，输出含 `disabled` 的说明，且不新增任何 TCP 监听（默认端口 8787 的 LISTEN pid 集合不变） | Must |
| AC-HTTP-002 | `--http --addr 127.0.0.1:PORT` 只有一个 LISTEN socket，地址恰为 `127.0.0.1:PORT`；`0.0.0.0`/非回环 host 拒绝启动并非 0 退出且不留监听；`--addr` 缺省为 `127.0.0.1:8787` | Must |
| AC-HTTP-003 | `GET /health` 返回 200、`application/json`，字段集合恰为 `{status, version, read_only}`，`status=ok`、`version` 非空、`read_only=true` | Must |
| AC-HTTP-004 | `GET /spine/runs` 返回 200，顶层恰为 `{runs, count}`，每个 run 恰为 `{run_id, status, current_stage, blocked, feature}`；`count == len(runs)`；相邻 run 满足 `started_at` 倒序、同刻 `run_id` 升序 | Must |
| AC-HTTP-005 | `GET /spine/gates?feature=` 返回 200，顶层恰为 `{feature, gates}`，每个 gate 恰为 `{gate_id, gate_type, verdict, evaluated_at, reason_codes}`，gates 按 `evaluated_at` 倒序；缺参返回 400 `bad_request`，未知 feature 返回 404 `not_found` | Must |
| AC-HTTP-006 | `GET /runs/{id}` 返回 200 且顶层恰为 `{run}`（run 字段集合固定）；`GET /features/{slug}` 返回 200 且顶层恰为 `{feature}`（字段集合固定）；未知 id/slug 返回 404 `not_found` | Must |
| AC-HTTP-007 | 对五个已知路径使用 POST/PUT/PATCH/DELETE 均返回 405 `method_not_allowed`；`/release`、`/publish`、`/complete` 及其 run/feature 子路径返回 404 `not_found`，不存在任何写或 release 入口 | Must |
| AC-HTTP-008 | 上述全部请求前后，`runs`、`feature_packages`、`gate_results`、`lifecycle_events` 的行数完全一致；服务只读访问运行库 | Must |

## Constraints & Authority

- 只读：不调用 tick / freeze / release / publish，不写库，不写 evidence。
- 仅回环：监听地址必须是 `127.0.0.1` 或 `::1`；默认关闭。
- 单一真源：HTTP 层只读 `BEACON_STORE_PATH`（缺省 `~/.beacon/beacon-go.db`），与 CLI 读取同一运行库。
- 不改本包与 docs 之外的生产代码，直到本包 frozen。

## Freeze checklist

- 确认端点与字段集合（本文件即 seed 断言来源）。
- 三条 seed 在实现前为红，在实现后转绿；canon sha256 与 evidence 记录一致。
- `go test ./...` 全绿，CLI 既有命令行为不变。
- QA 由独立 reviewer 出具 verdict；maker 不自证。
