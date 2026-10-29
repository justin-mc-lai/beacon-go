# 变更记录：skills 安装补齐宿主目录（含 WorkBuddy）

> 日期：2026-10-28 ｜ 触发：用户反馈「安装到本机的 skill 没有 beacon-go 选择」
> 状态：已实现并验证

---

## 1. 根因

`beacon skills install` 的宿主表只有三个名字：

```
--host claude|codex|generic   (default "generic")
```

- `generic` 解析到 `~/.agents/skills`；
- `claude` 解析到 `~/.claude/skills`；
- 没有 WorkBuddy —— 而 WorkBuddy 读的是 `~/.workbuddy-ai/skills`。

所以执行 `beacon skills install` 之后，7 个技能确实落盘了，但落在宿主不读的目录里，
本机技能列表自然「没有 beacon-go 可选」。另外默认值不报错、不提示，问题被藏起来了。

对照 Python（`beacon/utils/agent_surface.py`）：它支持
`trae|claude|codex|cursor|windsurf`，并有 `doctor setup-agent-surface` /
`verify-agent-surface` 校验；Go 侧宿主表更窄，也没有校验面。

## 2. 修复

| 项 | 之前 | 现在 |
|---|---|---|
| 宿主表 | cli 与 orchestrator 各写一份，3 个名字 | `skills/hosts.go` 一份表，7 个宿主：workbuddy / claude / codex / agents / cursor / windsurf / trae |
| 默认行为 | 静默写 `~/.agents/skills` | 自动探测**第一个存在**的宿主目录；一个都不存在就报错并列出候选（fail closed） |
| 诊断 | 无 | 新增 `beacon skills hosts`：每个宿主的目录、是否存在、已装哪些 `beacon-*` |
| 别名 | `generic` | `generic` 仍可用（指向 agents），另接受 `workbuddy-ai`、`claude-code` |
| hook | 自己维护 claude+agents 映射 | 复用同一张表（只对齐 claude+agents，行为不变） |

`beacon skills install` 现在会先打印 `target: ... (host workbuddy)`，再逐文件列出写入计划。

## 3. 实测

```
$ beacon skills hosts
workbuddy  present  none            /Users/apple/.workbuddy-ai/skills
claude     present  1 (beacon-go)   /Users/apple/.claude/skills
codex      present  1 (beacon-go)   /Users/apple/.codex/skills
agents     present  1 (beacon-go)   /Users/apple/.agents/skills
cursor     present  none            /Users/apple/.cursor/skills
windsurf   present  none            /Users/apple/.windsurf/skills
trae       present  none            /Users/apple/.trae/skills

$ beacon skills install --host workbuddy
installed 7 skills (14 files) into /Users/apple/.workbuddy-ai/skills
```

落盘结构（与 Python 的 Scheme A 1+6 一致）：

```
~/.workbuddy-ai/skills/
  beacon-goal/SKILL.md + references/core.md
  beacon-plan/…  beacon-truth/…  beacon-design/…
  beacon-implement/…  beacon-qa/…  beacon-release/…
```

## 4. 仍然缺的（对照 Python，不掩盖）

1. **技能包数量**：Python `skills/beacon/` 有 22 个技能包（7 个公共 + 14 个 `beacon-gov-*`
   治理技能 + `agents` 路由包 + examples/references/scripts）；Go 只嵌入了 1+6 共 7 个。
   所以就算装好了，Go 的技能列表也比 Python 短一截。
2. **项目内安装**：Python 支持 `.cursor/skills`、`.windsurf/skills` 及对应的 rules 文件与 hooks；
   Go 目前只支持全局目录。
3. **校验面**：Python 有 `doctor setup-agent-surface` / `verify-agent-surface --strict`；
   Go 的 `doctor` 只有 `rebuild`，装完没有机器校验「宿主真的能发现」。
4. **route examples**：AC-127 / AC-144 要求的示例链正文仍为空（`grep examples skills/*/SKILL.md` → 0）。

## 5. 验证

```
$ go test ./skills/ ./internal/cli/ -count=1     # 全绿（新增 7 条宿主/安装测试）
$ go build ./... && go vet ./...                 # 通过
```
