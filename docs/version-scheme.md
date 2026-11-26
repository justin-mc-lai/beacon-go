# Beacon Go 版本规范

> 决定：纯 `MAJOR.MINOR.PATCH`，tag 为 `vX.Y.Z`。
> 适用范围：`beacon-go` 仓库、二进制、文档、tag、release。
> 本文是规则，不是需求真相；实现以 `internal/version/version.go` 为准。
> 版本轴已从 `0.0.1` 重新开始；历史版本线随旧冻结工件一并清除，不追溯。

## 1. 规则

| # | 规则 |
|---|---|
| V-01 | 版本号只能是 `MAJOR.MINOR.PATCH`，例如 `0.0.1`、`0.1.0`。 |
| V-02 | Git tag 是 `v` + 版本号，例如 `v0.0.1`。 |
| V-03 | 禁止把 `-go.N`、`-simplified`、`-debug` 作为发布版本。 |
| V-04 | 预发布只允许 `-rc.N`，例如 `0.1.0-rc.1`，且必须显式标记。 |
| V-05 | 版本唯一源是 `internal/version/version.go`；`internal/cli.Version` 只是再导出。 |
| V-06 | 禁止在 Go 源码里再写第二份版本字面量（ACP clientInfo 等必须引用同一常量）。 |
| V-07 | README、README.zh-CN、`.claude-plugin/plugin.json` 的版本必须与唯一源一致；一致性由 `scripts/check_version_consistency.sh` 检查。 |
| V-08 | 每个发布必须有：tag、GitHub Release、release notes、CHANGELOG 条目。 |
| V-09 | 发布前必须通过版本一致性检查与全量测试；tag 后检查失败要撤销 tag 或立即发修复版。 |
| V-10 | 文档版本线与运行时版本分离：`docs/beacon/<version>/` 是按需生成的需求树线，二进制版本是运行时版本；两者在发布时对齐。 |
| V-11 | AGENTS.md 不写死当前版本，只写权威规则与版本轴起点（0.0.1）。 |

## 2. 版本轴起点

首个版本是 `0.0.1`——按 AGENTS.md「唯一真源」重新设计的内核起点。递增规则：

| 变更类型 | 版本 |
|---|---|
| 首个设计版本 | `0.0.1` |
| 向后兼容的修复 | `0.0.2`、`0.0.3` … |
| 向后兼容的功能 | `0.1.0`、`0.2.0` … |
| 不兼容/契约重设 | `0.x` 内可重设，或进入 `1.0.0` 后按 SemVer major 递增 |

`0.x` 阶段契约尚未冻结，允许在 minor 内调整；进入 `1.0.0` 后严格 SemVer。

## 3. 单一源与代码

```go
// internal/version/version.go — 唯一源；实际值随发布迭代，勿在别处复制字面量
package version

const Version = "x.y.z" // 占位：真实值见该文件
```

- `internal/cli/root.go`：`const Version = version.Version`。
- `internal/harness/acp.go`：`version.Version`（探针超时默认值）。
- `e2e/run_e2e.sh`：只校验 `^Beacon CLI v[0-9]+\.[0-9]+\.[0-9]+$`，不写死版本。
- `scripts/check_version_consistency.sh`：代码 vs README/README.zh-CN/plugin.json；AGENTS.md 禁止写死当前版本。

## 4. 发布流程（可执行）

1. 确认 `internal/version/version.go` 是目标版本。
2. 更新 README / README.zh-CN 当前版本行、`.claude-plugin/plugin.json` 的 version。
3. 更新 `CHANGELOG.md` 条目。
4. 若发布带需求线：新增 `docs/beacon/<version>/`，至少含 `SUMMARY.md` 与 `release/<version>.md`（非生成存根）。
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

## 5. 校验清单

- [ ] `beacon --version` 输出纯 `vX.Y.Z`
- [ ] tag 是 `vX.Y.Z`
- [ ] README / README.zh-CN / plugin.json 版本一致
- [ ] AGENTS.md 没有写死当前版本
- [ ] CHANGELOG 有条目
- [ ] 全量测试绿
- [ ] e2e 绿

## 6. 边界

- 本规范不追溯修改历史 tag/Release；旧版本线已随冻结工件清除。
- Go module 路径 `module beacon-go` 在 `0.x` 阶段无需改（主版本 0）；进入 `1.0.0` 且需 `go install` 时再评估完整 module path。
- 本规范不把 Python 的 `1.6.19` 当作 Go 的版本；只借用表达方式，不共用版本号。
