# 冻前对齐（chain `full-delivery`）

前提：宿主已完成 plan 调查与本轮范围取舍。以下命令是可执行模板，
变量由当前项目账本与 proposal 的实际回执填写；本示例未声称执行成功。

```bash
beacon plan expand --project "$PROJECT" --version "$VERSION" --node "$PARENT" --children "$PROPOSAL"
beacon plan apply --project "$PROJECT" --version "$VERSION" --node "$PARENT" --proposal "$PROPOSAL"
beacon truth tree --project "$PROJECT" --version "$VERSION" --format html
```

首次树不存在时先 `beacon plan init --project "$PROJECT" --title "$TITLE" --intent "$INTENT"`。
CLI plan apply 是工具名；这里属于 truth 阶段。已有树修改先走 change。
宿主核对输出真实 html_path/source_hash、当前节点和覆盖清单，交用户审阅批准。
然后依照内核缺项从叶到父逐个调用：

```bash
beacon truth node freeze --project "$PROJECT" --version "$VERSION" --node "$NODE" --json
```

成功产出：本轮树、HTML、批准记录、各节点真实冻结回执和指纹。
拒绝：缺 HTML、哈希陈旧、未批准、零有效叶或验证规格缺失；保留 truth 和 next/missing。
回执列 run_id/current_stage/existing_artifacts/next_action/owner，不将本模板视为执行证据。
