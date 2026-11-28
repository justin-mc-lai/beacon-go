# beacon-goal — core reference

## 当前 run 与阶段

阶段固定为 `plan -> truth -> implement -> verify -> release`。
plan 负责调查、头脑风暴和分析，可选候选树；truth 形成本轮唯一 canonical 树，
必交 HTML 供审阅批准后冻结；后续节点变更统一交 change。
实现归宿主并经 admission（包括 worktree）；验证使用独立 verifier；发布由人批准。
内核不调 LLM、不跑 agent，`goal tick --by` 记录宿主真实执行。

## 推进与恢复

1. `goal run` 创建 run；恢复时读取明确 run_id，不能凭聊天或隐式 latest 选状态。
2. `goal workflow check --run-id <id>` 返回 next/missing。宿主整理来源、准备材料、
   完成必要绑定与执行；用户只决定目标、取舍、批准，不承担重复技术清单解释。
3. 对照当前 run、阶段产物、指纹和命令回执完成动作，再 `goal tick --run-id <id> --by <identity>`。
4. 输出 run_id/current_stage/existing_artifacts/next_action/owner；
   不存在的 run、未运行的命令、旧 HTML 或文档 status 一律不能报成实际交付。
5. 中断后用 `goal resume-pack --run-id <id>` 读取恢复包，再按缺项继续；
   无执行者或证据就保留阻塞与责任人。

## 持续树与本轮完成

逻辑一棵树持续成长；版本保存不可变 baseline + 本轮 delta。
future draft 可与本轮同时存在，范围和依赖闭包外的草稿不拖本轮发布。
若实际内核尚未支持所需范围隔离，明确报告阻塞，不能靠文档改变 gate 事实。
完成仍需 release_gate、人工批准、独立 verifier 与 verify pass 的真实记录。
本轮完成、hash verified、gate pass、tree verified、version ready 分开报告。
