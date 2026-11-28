# 冻后修改（chain `change-refreeze`）

输入：「将退款 AC-003 金额单位改为元」。
后续节点变更统一交 `beacon-go-change`，完整读取其 core 与 change-and-refreeze 示例。
冻结节点先 `beacon change request`，授权后 `truth node update`，
重交当前 HTML 审阅批准，再 `truth node freeze` 自底向上。
旧包兼容使用 `beacon change legacy --feature order-refund --summary "AC-003 单位改为元"`，
编辑获准正文后 `beacon truth refreeze --feature order-refund`。

成功：change id、当前指纹、真实回执；拒绝：直接改冻结 JSON、未审阅或旧证据冒充新验收。
