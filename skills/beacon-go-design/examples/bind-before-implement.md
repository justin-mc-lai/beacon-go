# 示例：绑定后才放行

**用户说**：「视觉规范定了，可以开工」

**mode_id**：`ds-bind`

**做法**
1. 把原语写成文件：颜色、间距、组件状态
2. `beacon design bind --run-id <id> --primitives design.json --by operator`
3. `beacon implement admit --run-id <id>` → 设计项通过

**产出**：绑定记录 + 通过的准入

**不要做**：绑一堆 TBD 骗过准入
