# 冻结子树变更（chain `change-refreeze`）

输入：「退款 AC-003 单位改为元」。宿主从当前树读取节点 ID、指纹、版本，
准备 patch（不手改 canonical）。以下为命令模板，不是已执行证据。

```bash
beacon change request --project "$PROJECT" --version "$VERSION" --node "$NODE" --base-fingerprint "$HASH" --summary "AC-003 单位改为元" --by "$HOST" --json
beacon truth node update --project "$PROJECT" --version "$VERSION" --node "$NODE" --file "$PATCH" --json
beacon truth tree --project "$PROJECT" --version "$VERSION" --format html
```

核对 change id、受影响祖先与真实 HTML/source_hash，用户审阅批准后从叶到父执行：

```bash
beacon truth node freeze --project "$PROJECT" --version "$VERSION" --node "$NODE" --json
beacon change list --project "$PROJECT" --version "$VERSION" --json
```

baseline 保持不可变，本轮 delta 记录变更；future draft 留在明确的未来范围，
本轮有依赖则先处理。draft 调整也进入 change 工作流，正文用 truth node update，
不对 draft 盲调 frozen-only request；无子 draft 废弃用 change deprecate。
新增 future draft 使用现有入口：

```bash
beacon change grow --project "$PROJECT" --version "$VERSION" --node "$PARENT" --proposal "$PROPOSAL" --base-fingerprint "$PARENT_HASH" --summary "登记未来范围" --by "$HOST"
```

成功：变更记录、新指纹、HTML 与冻结回执；拒绝：并发指纹不符、未授权改冻结内容、
依赖未满足或将 future draft 的存在冒充本轮完成。回执包含当前 run/阶段/产物/下一步/owner。
