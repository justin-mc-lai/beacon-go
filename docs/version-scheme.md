# Beacon Go 版本规范（2026-11-24）

> 决定：Go 线采用 Python Beacon 的版本表达方式——**纯 `MAJOR.MINOR.PATCH`，tag 为 `vX.Y.Z`**。
> 适用范围：`beacon-go` 仓库、二进制、文档、tag、release。
> 本文是规则，不是需求真相；实现以 `internal/version/version.go` 为准。

## 1. 规则

| # | 规则 |
|---|---|
| V-01 | 版本号只能是 `MAJOR.MINOR.PATCH`，例如 `2.0.0`、`2.1.3`。 |
| V-02 | Git tag 是 `v` + 版本号，例如 `v2.0.0`、`v2.1.3`。 |
| V-03 | 禁止把 `-go.N`、`-simplified`、`-debug` 作为发布版本。 |
| V-04 | 预发布只允许 `-rc.N`，例如 `2.1.0-rc.1`，且必须显式标记。 |
| V-05 | 版本唯一源是 `internal/version/version.go`；`internal/cli.Version` 只是再导出。 |
| V-06 | 禁止在 Go 源码里再写第二份版本字面量（ACP clientInfo 等必须引用同一常量）。 |
| V-07 | README、README.zh-CN 的“当前版本”必须与唯一源一致；其他历史提及不得冒充当前版本。 |
| V-08 | 每个发布必须有：tag、GitHub Release、release notes、CHANGELOG 条目、对应 docs 版本线。 |
| V-09 | 发布前必须通过版本一致性检查与全量测试；tag 后检查失败要撤销 tag 或立即发修复版。 |
| V-10 | 文档版本线与运行时版本分离：`docs/beacon/<version>/` 是需求真相线，二进制版本是运行时版本；两者在发布时对齐。 |

## 2. 历史映射

旧 Go 线的 `v2.0.0-go.N` 全部是 `2.0.0` 的**预发布**。按 SemVer，预发布小于正式版，因此：

| 旧 | 新 | 说明 |
|---|---|---|
| `v2.0.0-go.1` … `v2.0.0-go.17` | `v2.0.0` | 历史 tag 保留，不删除、不移动；首个正式版为 `2.0.0`。 |
| 未来的功能批次 | `2.1.0`、`2.2.0` … | minor 递增。 |
| 未来的修复 | `2.0.1`、`2.1.1` … | patch 递增。 |
| 未来的破坏性变更 | `3.0.0` | major 递增。 |

旧 tag 是历史事实，不再用于新发布；新发布只打 `vX.Y.Z`。

## 3. 单一源与代码

```go
// internal/version/version.go
package version

const Version = "2.0.0"
```

- `internal/cli/root.go`：`const Version = version.Version`。
- `internal/harness/acp.go`：`version.Version`（探针超时默认值）。
- `e2e/run_e2e.sh`：只校验 `^Beacon CLI v[0-9]+\.[0-9]+\.[0-9]+$`，不再写死版本。
- `scripts/check_version_consistency.sh`：代码 vs README/README.zh-CN；AGENTS.md 禁止写死当前版本。

## 4. 发布流程（可执行）

1. 确认 `internal/version/version.go` 是目标版本。
2. 更新 README / README.zh-CN 的当前版本行。
3. 更新 `CHANGELOG.md` 条目。
4. 新增 `docs/beacon/<version>/` 版本线，至少包含 `SUMMARY.md` 与 `release/<version>.md`；release notes 不得是生成存根。
5. 跑：
   ```bash
   bash scripts/check_version_consistency.sh
   go build ./...
   go vet ./...
   go test ./... -count=1
   bash e2e/run_e2e.sh
   ```
6. 在通过的提交上打 `vX.Y.Z`，创建 GitHub Release。
7. 发布后校验：`beacon --version`、Release 页面、docs 版本线三者一致。

## 5. Go module 路径（待办）

当前 `go.mod` 是 `module beacon-go`，主版本是 2。按 Go module 规则，主版本 2 的模块路径应以 `/v2` 结尾：

```
module github.com/justin-mc-lai/beacon-go/v2
```

影响面：所有内部 import 从 `beacon-go/internal/...` 改为 `github.com/justin-mc-lai/beacon-go/v2/internal/...`。这是纯机械改动，但 diff 很大；在只做 CLI 本机构建时不影响功能。**建议单独一个提交完成，不和版本号改动混在一起。**

## 6. 校验清单

- [ ] `beacon --version` 输出纯 `vX.Y.Z`
- [ ] tag 是 `vX.Y.Z`
- [ ] GitHub Release 存在且 notes 非存根
- [ ] README / README.zh-CN 当前版本一致
- [ ] AGENTS.md 没有写死当前版本
- [ ] `docs/beacon/<version>/release/<version>.md` 存在且非存根
- [ ] CHANGELOG 有条目
- [ ] 全量测试绿
- [ ] e2e 绿

## 7. 边界

- 本规范不追溯修改历史 tag/Release。
- 本规范不要求立即把 module 路径改成 `/v2`；但发布为可 `go install` 的模块前必须改。
- 本规范不把 Python 的 `1.6.19` 当作 Go 的版本；只借用表达方式，不共用版本号。
