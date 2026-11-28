# Project module tree to workflow (chain `full-delivery`)

输入：接手已有项目，调查模块、需求来源、当前阶段与下一步。

1. plan 先读项目说明、可用源码/测试与已有需求；不存在树也能调查。
   读取明确 run_id；无账本记录就写未确认，不补造 run。
2. 区分产品模块、实现组件、迭代包、阅读分类；输出 module_count 的计数口径。
   coverage_manifest 列来源、covered/excluded/unknown；重复来源先映射合并。
3. 候选树可选。若需要，宿主提出 `intent -> module -> feature -> touchpoint`，
   每次展开最多七子节点，叶子拟定 AC/验证；不把方案提议当作 canonical 冻结。
4. 交 truth 将本轮范围形成唯一 canonical 树，使用已有 CLI 工具命名
   `plan init/expand/apply`；已有树的后续节点修改经 change。
5. truth 必交 `beacon truth tree --project <root> --version <v> --format html`，
   核对真实 html_path/source_hash，用户审阅批准，再按内核缺项冻结。
6. 每阶段回执列 run_id/current_stage/existing_artifacts/next_action/owner，
   依据当前 run 与实际命令，不凭文档状态宣称完成。

## worked sample（非通用数量规则）

来源：本仓 2026-11-28 存量项目模块梳理示例；五个基础模块加六个扩展模块，
得到 **11** 个交付模块，与 **28** 个历史迭代包不同。该数字只解释该样本口径，
新项目必须从实际来源重新计数，不采用日期、11/28 或固定模块数作为规则。

成功：有来源的调查结论与范围取舍交给 truth；若实际形成树，另附其回执。
拒绝：缺来源、零覆盖却报完整、重复 canonical 归属、虚构 run 或未生成 HTML 却报已交。
