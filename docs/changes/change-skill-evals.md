# 变更记录：技能示例与 evals 接入 `qa score`

> 日期：2026-10-28 ｜ 承接：`docs/skill-gap-2026-10-28.md`（技能层合同缺失诊断）
> 状态：已实现并验证

---

## 1. 做了什么

### 1.1 每个技能补 3 条示例

`skills/<id>/examples/*.md`，共 21 个文件。统一格式：

```
**用户说**：<真实一句话>
**mode_id**：<走哪条路>
**做法**：编号命令序列
**产出**：可检查的东西（文件/指纹/裁决）
**不要做**：这一步最常见的偷懒方式
```

示例是 few-shot 锚点：模型看到"别人怎么干"，比看到"你要好好干"有效。

### 1.2 每个技能补 1 组 evals

`skills/<id>/evals/evals.json`，schema `skill-evals.v1`：

```json
{
  "schema": "skill-evals.v1",
  "skill": "beacon-goal",
  "cases": [
    {"id": "goal-00-boundary", "prompt": "…", "expectations": ["…", "…"],
     "checks": [{"kind": "contains", "text": "## HARD GATE"}]}
  ]
}
```

`expectations` 是给人和模型看的（需要判断）；`checks` 是机器可判定的断言：

| kind | 断言 |
|---|---|
| `contains` | SKILL.md 正文包含某段话 |
| `references_contains` | `references/` 下任一文件包含某段话 |
| `min_examples` | `examples/` 文件数 ≥ N |

**边界（写进代码注释与卡片）**：checks 只回答"技能有没有写清它声称的规矩"，
不回答"模型有没有照做"。后者需要模型跑一遍 + 人打分，留在 effectiveness 维度。

### 1.3 `qa score` 增加三项结构检查

| check | 分值 | 判定 |
|---|---:|---|
| `evals_present` | 3 | 文件存在且 schema 正确 |
| `evals_cases` | 3 | ≥3 用例，每个 id/prompt/≥2 expectations |
| `evals_checks_pass` | 4 | 所有机器检查通过，失败点名到 case id |

`name_matches_package` 同时修正：接受 `beacon-<dir>`（Scheme A 安装名）作为匹配，
否则每个技能都会被判"名字与目录不一致"。

### 1.4 安装面同步

`skills install` 现在复制整个包（SKILL.md + references/ + examples/ + evals/），
不再只复制 SKILL.md + references。7 个技能 = 42 个文件。

---

## 2. 第一次跑就抓到真问题

`plan` 的用例断言技能里写了 `"Never answer"`，但技能正文实际写的是
`"answer your own question"` —— 断言与正文不一致，`evals_checks_pass` 直接 0/4，
分数 82.61/B。改成正文真实存在的表述后回到 100/A。

这不是演示，是 evals 的正常工作方式：**它给漂移报警，不给人看热闹。**

---

## 3. 验证

```
$ beacon qa score skills/goal --project /tmp/beacon-score --feature demo --evaluator operator
structural     100.00 / 100 (weight 60, mechanical_static)
  [ok] evals_present       3/3  evals/evals.json parses (3 cases)
  [ok] evals_cases         3/3  3 cases, each with id/prompt/2+ expectations
  [ok] evals_checks_pass   4/4  all 8 machine checks pass
effectiveness  not scored (not_scored)
total: 100.00  grade: A

$ beacon skills install --host workbuddy --force
installed 7 skills (42 files) into /Users/apple/.workbuddy-ai/skills
```

七个技能全部 100/100（结构层）。测试：`go test ./...` 全绿；新增
`internal/orchestrator/score_evals_test.go`（4 条）与 `skills/contract_test.go`（2 条）守卫。

---

## 4. 没做的

1. **行为评测**：没有接模型跑 evals 的自动打分。`effectiveness` 仍然显示 `not_scored`，
   要人跑一遍再 `--human-score`。装作自动打分会变成另一种假证据。
2. **router / progressive map**：意图 → 技能的路由仍缺。
3. **14 个 `beacon-gov-*`**：仍未迁移。
