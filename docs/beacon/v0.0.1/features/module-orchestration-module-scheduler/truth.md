---
slug: module-orchestration-module-scheduler
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_030e377c5621
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
意图树 module 节点目前不能展开为多个 feature run，也没有「同 module 内按声明顺序串行、跨 module 分配 worktree 并行」的调度逻辑；现有 internal/orchestrator/worktree.go 只提供 satellite pool 原语（ParallelPlan.Mode / Satellites、worktree create/status 等），不决定 run 的展开与顺序。结果是多 feature module 只能人工逐个启动，跨 module 并行靠手工分配 worktree。

## 目标用户
从一个 module 节点批量派生多个 feature run 的工作流作者，以及需要通过 worktree 状态视图确认跨 module 并行与同 module 串行顺序的调试者。

## 场景
意图树里有两个 module，每个 module 下各有若干 feature。操作者运行 module run 得到 N 个 run id，每个 run 的 FeatureID 都归属对应 module；跨 module 的 run 由 pool 模式自动分配各自 worktree 并行，同 module 内第二个 feature 的 run 在第一个 run 的 stage done 之前保持 pending。

## 范围
本 feature 冻结「从 module 节点展开多个 feature run，同 module 内按序、跨 module 分配 worktree 并行」这一需求边界。现有落点是 internal/orchestrator/worktree.go 的 satellite pool 原语（ParallelPlan.Mode=pool、Satellites、worktree 创建与 status 视图）。module run 展开命令、同 module 串行顺序控制、parallel_plan 的 mode/satellites 与 module 数自动对应，当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不定义 module 启动前的统一准入检查与 blocked 判定（由兄弟 feature module-admission 冻结），也不改动 git worktree 与分支合并本身的实现细节。

## 验收标准
`beacon goal worktree status --run-id <id> --json` 应显示 parallel_plan mode=pool 且 satellites 数等于 module 数；`beacon goal status --run-id <id2>` 在第一个 run 的 stage done 之前应显示该 run blocked=false 且 current_stage=plan，即尚未越序执行。module 展开的验收命令为 `beacon goal module run --node <module-id> --json | jq -e 'length > 0'`，该命令当前尚不存在，属待实现能力；就位后应输出 N 个 run id 且每个 run 的 FeatureID 属于该 module。

## 状态迁移

| From | To | Event |
|---|---|---|
| draft | planned | plan |
| planned | running | dispatch |
| running | done | complete |
| running | blocked | admission_fail |
| blocked | planned | replan |

### 非法迁移

- draft -> running 必须被拒绝
- done -> running 必须被拒绝
