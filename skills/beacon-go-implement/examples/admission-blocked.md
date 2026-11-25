# 示例：准入被拦

**用户说**：「开始写代码」

**mode_id**：`im-single`

**做法**
1. `beacon implement admit --run-id <id>` → 返回 `truth_not_frozen`
2. 停下，报告原因与下一步（先冻需求），不绕过
3. 冻结后重跑 admit → pass

**产出**：一次 blocked 记录 + 一次 pass 记录

**不要做**：换条 lane 再试；手动改状态
