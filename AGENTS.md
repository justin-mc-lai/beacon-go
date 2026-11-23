# AGENTS.md · beacon-go

给在这个仓里工作的 Bot 和 agent。冲突时以本文件为准。

## 目录与入口

- 引擎：`cmd/beacon`，核心包在 `internal/`
- 需求真相：`docs/beacon/<version>/features/<slug>/`（truth / tests / tasks / evidence）
- 收敛口径：`docs/scope-charter.md`（12 保留 / 49 合并 / 16 剥离）
- 版本：`beacon --version`；当前 `v2.0.0-go.15`

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

## 防返工与交付收口（2026-11-20）

- 测试门或 CLI 合同变化时，同批更新真实入口 fixture；必须有新链路成功与拒绝的实跑，基线既有红灯不得当作新功能已验收。
- 改共享路由、执行器或准入判断前，列出所有调用者与入口，测试必须经过真实 CLI wiring；不要一个路径修完再逐个补相同漏洞。
- 验收固定到 source commit + binary SHA + 需求指纹。证据、批准和安装同物；窄口径 PASS 不扩大为全量发布结论。
- 真并发用两个执行者同步屏障证明，不以两个 active 状态或时间戳代替；恢复测试覆盖副作用之前、之后与回执之前。保留未知/未提交文件。
- 本机安装只经仓内 `scripts/local_release.py` 的批准清单入口（该文件未进入当前仓时不得安装）；观察/维护脚本只报告，禁止直接构建覆盖 `~/.local/opt/beacon-go/beacon`。
- 同根因两次返工、两次断线或两次上下文错位，管理方停止叠加任务，保存 checkpoint 后换方式；15 分钟无实证进展主动检查，30 分钟主动说明阻塞，不等用户问。
- 到发布门后给用户确切清单；提交、推送、发布仍遵守上面的独立授权要求，不因规则更新获得权限。
