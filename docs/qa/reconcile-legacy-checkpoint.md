# Reconcile legacy checkpoint

## 实际遇到的四类阻塞

1. 初始 repair 守卫禁止 parent/path 投影漂移、多个 target 相互阻塞，且全树容量检查阻塞 legacy 超 7 节点。
2. canonical module 路径重复，同时 SQLite 缓存缺成员或重建失败；单纯投影修复无法解决正文路径冲突。
3. legacy-only `legacy_truth_path` 正文与省略 `truth_path` 的合法 index 被误判为未授权漂移。
4. draft module 有 frozen 后代；递归路径修复要求全子树 draft，因此正确拒绝。

## 最终窄策略

- 保持显式 targets + 当前有效正文 base，清单外 body/index 漂移拒绝。
- canonical ID/kind、父关系、有效指纹、全局路径唯一/规范及无环检查不放宽。
- 缓存仅为派生物，按显式版本以完整 journal 和事务重建。
- legacy-only truth link 可以沿用 index 省略形式；不把 legacy 节点隐式授权为 target。
- `path` 默认递归（`subtree: true`），仍要求目标 draft module 及全部后代 draft。
- 显式 `path` + `subtree: false` 只修改被授权 draft module 自身路径并重新 stamp。
  parent 不变，保留原 ChildFingerprints，后代正文及路径一律不写，包括 frozen 后代。
  canonical 路径不要求父路径前缀；不借目录消重重写冻结合同。
- action 仍为 `repair-path`，记录的 nodes 明确保存 `path` 和 `subtree: false`。
  不更新未授权祖先，不声称已重新冻结或完成交付验证。

本 checkpoint 根据用户提供的只读诊断记录；本执行者未操作 Portfolio。
本轮仅实现上述非递归开关并跑定向测试，不继续泛化修复范围。
