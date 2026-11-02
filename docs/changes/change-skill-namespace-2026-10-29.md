# 变更记录：技能安装命名空间化（opencode/codex 选不到子技能）

> 日期：2026-10-29 ｜ 触发：宿主里 `beacon-go` 只能作为一个路由器被选中，
> 六个子技能无法被发现；`beacon skills install` 还缺少 opencode 宿主。
> 状态：代码已改、本机 8 个宿主已重装，`go test ./...` 与 `e2e` 全绿。

## 1. 根因（实测）

| # | 根因 | 证据 |
|---|---|---|
| 1 | 宿主表没有 opencode | `skills/hosts.go` 的 `hostSpecs` 只有 workbuddy/claude/codex/agents/cursor/windsurf/trae；`beacon skills install --host opencode` 返回 `skill_install_host_unknown` |
| 2 | 三个共享宿主目录被 Python v1 的 `beacon-<id>` 软链占满，Go 同名安装直接冲突 | `~/.claude/skills/beacon-goal -> products/beacon/skills/beacon/beacon-goal`；`skills install --host claude` 返回 `skill_install_conflict`（7 个目录） |
| 3 | `skills hosts` 少报安装数 | `installedSkills` 用 `DirEntry.IsDir()`，软链目录不算 |
| 4 | 唯一可见的 `beacon-go` 是手写技能，明令禁止安装子技能，且版本停在 go.9 | `~/.config/opencode/skills/beacon-go/SKILL.md` |
| 5 | 部署二进制停在 go.9 | `~/.local/opt/beacon-go/beacon --version` = `v2.0.0-go.9` |

## 2. 修复

1. `hostSpecs` 增加 `opencode`（`~/.config/opencode/skills`）。
2. 安装名加轨道命名空间：`beacon-<id>` → `beacon-go-<id>`（`InstallPrefix`），
   frontmatter `name` 同步；v1 的 `beacon-<id>` 永不被覆盖。
3. `installedSkills` 改用 `os.Stat` 跟随软链，`skills hosts` 如实报数。
4. 新增随二进制生成的 `beacon-go` 路由技能（`RouterSkillBody`），
   不写死版本号（版本由二进制自己报），安装时总是刷新。
5. 部署二进制刷新到当前构建。

## 3. 本机安装结果

```
$ beacon skills hosts
workbuddy present 8   (beacon-go + 7 beacon-go-*)
claude    present 23  (Go 8 + Python v1 beacon-* 软链)
codex     present 23
opencode  present 8
agents    present 23
cursor    present 9
windsurf  present 9
trae      present 9
```

`~/.workbuddy-ai/skills` 里 go.9 时代留下的 7 个扁平 `beacon-<id>` 已删除
（确认为 Go 模板且无 metadata 后才删）；claude/codex/agents 的 v1 软链原样保留。

## 4. 验证

```
gofmt -l .                 # 空
go build ./...             # 0
go vet ./...               # 0
go test ./... -count=1     # 全绿
bash e2e/run_e2e.sh        # PASS
beacon skills install --dir <tmp>   # 生成 beacon-go + 7 个 beacon-go-*
```

新增/更新的测试：`TestHostDirsReportsInstalled`（含软链与 router 计数）、
`TestInstallMaterializesHostLayout`（命名空间布局）、`TestSkillsInstallCLI`、
`TestSkillsInstallIntoWorkBuddyDir`、`TestPostReleaseHookTriggerAndBuiltinInstall`。

## 5. 使用提醒

宿主在启动时扫描技能目录；已安装的 8 个宿主需要**重启 opencode / Codex 会话**
（或新开会话）才会重新列出 `beacon-go-goal` 等技能。旧会话不会热加载。
