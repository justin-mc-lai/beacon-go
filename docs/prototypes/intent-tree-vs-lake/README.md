# 意图树 vs 湖海：双拆解原型（ticket #17）

用同一个真实例子（羽毛球运动社区小程序，含赛事与社区）比较两套需求拆解模型。产物只用于决策，不进入内核包。

- 例子原句：`构建一个羽毛球运动社区小程序，有赛事功能、社区功能`
- 决策票：[#17](https://github.com/justin-mc-lai/beacon-go/issues/17)
- 上游 spec：[#15](https://github.com/justin-mc-lai/beacon-go/issues/15)

## 文件

| 文件 | 说明 |
|---|---|
| `metrics.py` / `render.py` | 通用指标脚本与 HTML 渲染器（第一个参数为样本目录） |
| `samples/badminton/` | 羽毛球社区样本：基准项、湖海、意图树、原始引擎输出、指标、返工记录 |
| `samples/cli-note-sync/` | 命令行笔记同步样本：同结构 |
| `decision.md` | 两样本判定、限制与下一步 |
| `samples/*/out/tree.html` | 意图树 HTML 视图（gitignored 生成物） |

## 复跑

```bash
cd docs/prototypes/intent-tree-vs-lake
python3 metrics.py samples/badminton     # 写该样本的 metrics.json / metrics.md
python3 metrics.py samples/cli-note-sync
python3 render.py samples/badminton      # 写 samples/<x>/out/tree.html
python3 render.py samples/cli-note-sync
# 真实引擎原始输出（可选，需要 Go；参数为需求原句）
go run ./_tools/decompose_lake.go "构建一个羽毛球运动社区小程序，有赛事功能、社区功能" \
  > samples/badminton/raw-engine-lake.json
```

## 指标口径（测量前固定）

1. **叶子覆盖率**：叶子单元声明覆盖的基准项 / 15。湖海的叶子单元是 `feature`；意图树的叶子单元是 `touchpoint`。
2. **歧义点数**：显式歧义记录 + 叶子陈述中的模糊词命中，按“单元+词”去重。
3. **人工修改次数**：`edit-log.json` 中该模型的条目数。
4. **叶子验证绑定率**：带非空验证规格的叶子 / 全部叶子。

判定线：意图树 ≥3 项不劣于湖海，且验证绑定率 100%。

## 非目标

- 不实现内核 schema、命令或冻结逻辑（那是 #18 之后）。
- 不用原型代码证明内核行为；原型只比较两种拆解的可见结构。
- 不做 UI 设计；HTML 只是一个视图样例。
