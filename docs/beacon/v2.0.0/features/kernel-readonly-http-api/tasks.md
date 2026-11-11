---
slug: kernel-readonly-http-api
version: v2.0.0
status: draft
revision_id: R2-draft
---

# Tasks

实现阶段只在 truth 冻结后开始；勾选一项必须附可复跑证据。

- [ ] TASK-HTTP-00 冻结本包：确认端点、字段集合、排序、默认地址与退出码 · ac=AC-HTTP-001
- [ ] TASK-HTTP-01 `beacon goal serve` 命令与 `--http` / `--addr` 参数；无 `--http` 时打印 disabled 并 exit 0 · ac=AC-HTTP-001
- [ ] TASK-HTTP-02 回环地址校验：非 `127.0.0.1`/`::1` 拒绝启动、非 0 退出、不留监听 · ac=AC-HTTP-002
- [ ] TASK-HTTP-03 `GET /health` · ac=AC-HTTP-003
- [ ] TASK-HTTP-04 `GET /spine/runs`（`started_at` 倒序、同刻 `run_id` 升序） · ac=AC-HTTP-004
- [ ] TASK-HTTP-05 `GET /spine/gates` 带 feature 查询参数（`evaluated_at` 倒序；缺参 400、未知 404） · ac=AC-HTTP-005
- [ ] TASK-HTTP-06 `GET /runs/{id}` 与 `GET /features/{slug}`（未知 404） · ac=AC-HTTP-006
- [ ] TASK-HTTP-07 统一 JSON 错误、已知路径非 GET 405、未知路径 404；不注册任何写/release 路由 · ac=AC-HTTP-007
- [ ] TASK-HTTP-08 只读访问运行库；SIGINT/SIGTERM 释放监听 · ac=AC-HTTP-008
- [ ] TASK-HTTP-09 三条 seed 转绿（TC-001..003），canon sha256 更新进 tests.md · ac=AC-HTTP-001..008
- [ ] TASK-HTTP-10 `go test ./...` 全绿，既有 CLI 行为不变 · ac=AC-HTTP-001..008
- [ ] TASK-HTTP-11 独立 QA 三方 verdict（spec/AC、运行时、负向回归），maker 不自证 · ac=AC-HTTP-008
