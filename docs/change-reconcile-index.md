# 显式目录一致性修复

`change reconcile-index` 必须指定 `--project`、`--version`、`--summary`、`--by`。
单节点使用 `--node` / `--base-fingerprint`；批量使用与它们互斥的 `--file`：

```json
{
  "nodes": [
    {"node_id": "projection-a", "base_fingerprint": "当前正文指纹"},
    {"node_id": "projection-b", "base_fingerprint": "当前正文指纹"},
    {"node_id": "draft-module", "base_fingerprint": "当前正文指纹", "path": "root/unique-module"}
  ]
}
```

- 不含 `path`：action 为 `reconcile-index`，不改正文，只投影已授权节点。
- 含 `path`：action 为 **`repair-path`**，会改正文。仅接受 draft module；其按
  `parent_id` 确定的全部后代必须 draft。后代新路径为新父路径加原路径的末段，
  不依赖历史路径是否以祖先路径为前缀。ID、parent、AC 身份不变，指纹自底向上重算。
- 多个路径修复根的子树不得重叠；不隐式重命名冲突的末段。最终路径必须规范、相对、
  非空且全树唯一。kind、根、父关系和环也必须有效。
- `path` 目标可显式带 `"subtree": false`：只改该 draft module 自身路径，保留其
  ChildFingerprints、parent 及全部后代正文/路径（包括 frozen 后代）。默认省略或
  `true` 仍执行上述全 draft 子树递归规则。canonical 路径不要求父路径前缀。
  action 为 `repair-path`，记录的 nodes 保留 `subtree: false`，modified 只含实际
  修改节点及显式投影目标。这是非递归目录消重，不是冻结子树变更授权。
- 每个显式目标必须提供当前有效正文指纹。无效正文、ID/kind 不一致、未列出的
  body/index 漂移均拒绝，包括位于授权路径子树内但未单列的既有投影漂移。
- SQLite 是派生缓存：允许缺失、旧成员和旧投影。读取全树有效正文并完成授权检查后，
  从 canonical index 成员重建该版本缓存。历史超 7 孩子不阻塞修复；move 仍限制目的父容量。
- 整批持树锁；journal 保存旧 index、待改正文、ChangeRecord 和完整旧版本缓存。
  缓存以事务替换，恢复也按原版本替换。记录 `nodes` 保存显式目标、base 和所请求的路径；
  `affected` / `modified` 包含目标和实际修改的 draft 后代。未授权祖先（含 frozen）正文不写，
  本操作不声称完成冻结祖先的重新冻结或交付验证。

调用示例：

```bash
beacon change reconcile-index --project /path/to/project --version 0.0.1 \
  --file repair.json --summary '修复明确列出的目录不一致' --by operator
```
