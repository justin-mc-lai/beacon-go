# Draft 节点废弃

冻结需求：`n_97cff751330d` AC-001。唯一入口：

```sh
beacon change deprecate --project . --version <version> --node <draft-id> \
  --base-fingerprint <reviewed-hash> --summary "废弃原因" --by <identity> --json
```

保留 canonical 节点、正文及 ID，将 draft 转 deprecated，同时生成 tree/changes 下的 ChangeRecord。
入口采用现有树锁与 mutation journal，失败恢复 canonical/index/SQLite 投影，不新增账本。
只有无子节点的非根 draft 可废弃；非空父节点要求先明确子节点处置，操作不隐式级联。
祖先必须为 draft/change_pending，不通过废弃 draft 偷改冻结祖先；冻结节点应先走 change request。
缺原因/身份/基线、过期基线、重复操作、非 draft 状态均非零拒绝。

本入口只交付 draft 废弃及记录，不自动清理父子关系，也不把 deprecated 视作 verified。
父级 freeze/rollup 继续遵守现有子节点状态合同；若产品需要忽略 deprecated，须另行定义门禁语义。
同类入口扫描：CLI change deprecate → intenttree.DeprecateDraft → 共用 setNodeStatus/journal；
测试 TestNodeDeprecateRecordsChangeAndRetainsBody、TestNodeDeprecateRefusals。
