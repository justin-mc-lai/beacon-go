# AGENTS.md · beacon-go

给在这个仓里工作的 Bot 和 agent。冲突时以本文件为准。

## 目录与入口

- 引擎：`cmd/beacon`，核心包在 `internal/`
- 需求真相：`docs/beacon/<version>/features/<slug>/`（truth / tests / tasks / evidence）
- 收敛口径：`docs/scope-charter.md`（12 保留 / 49 合并 / 16 剥离）
- 版本：`beacon --version`；当前 `v2.0.0-go.10`

## 可直接读写仓库

工作目录已授权。可以直接创建、修改仓库内文件，包括 `docs/`、`internal/`、`cmd/`、`e2e/`。

不需要再申请文件级授权。

## commit / push / tag 先问

以下动作一律先说明并等用户确认：

- `git commit`、`git push`、`git tag`
- 改已冻结的 truth 正文（要走 `beacon truth change` → `refreeze`）
- 改 release 语义、gate 语义、受保护路径（`.env`、密钥、支付、迁移）
- 发布

## 验证命令

改完必须跑，且贴真实输出：

```bash
go build ./...
go test ./...
bash e2e/run_e2e.sh        # 涉及端到端链路时
go vet ./...
```

测试没过不许说完成；不许用「应该可以」代替运行结果。

## 真相纪律

- 只做已冻结需求；没有冻结需求先退回 `Beacon 产品`。
- 最小改动，遵守现有结构与风格。
- 改了哪些文件、为什么、测试输出、未覆盖边界，四项一起交。

## 账本

Go 内核使用独立账本，避免与 Python v1 的 schema 冲突：

```bash
export BEACON_STORE_PATH="$HOME/.beacon/beacon-go.db"
```

需求正文永远在 Markdown，不进数据库。
