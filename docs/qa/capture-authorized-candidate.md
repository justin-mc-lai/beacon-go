# 已授权候选与真实采集阻塞

## 授权执行结果

- 用户授权：九叶文字 Spec 变更为可执行采集配置、refreeze，并提交本轮候选。
- 变更记录：`chg_2e66e84f000a`、`chg_4815ee13bf72`。
- 新快照：`v0.0.1-20261127T105738.295677000Z`。
- 候选源码提交：`0b9c2b26eb34a6d76b1a5f9fd4672676fa87329e`。
- 候选二进制 SHA256：`d02210bf5b65443c629be9059ef0c2064527f31a1e601ca69fa4a47e1e00ff7b`。
- 本轮来源、测试、skill、合同与审查文件已提交；已有 `.gitignore`、运行日志和 LoopX 脚本改动未纳入。
- 提交前 go build、全量 go test -count=1、go vet、版本/依赖/合同脚本均通过。

## 首次真实 Red 采集

```sh
beacon implement capture --project . --version v0.0.1 \
  --node n_a11841bbc44c --ac AC-001 --phase red \
  --by implementation-host --timeout-sec 180
```

系统返回：

```text
source_commit: 0b9c2b26eb34a6d76b1a5f9fd4672676fa87329e
exit_code: 0
status: rejected
reason: expected behavior Red or all passing Green (capture_result_invalid)
artifact_id: art_c53c70954b2f758ee8bece1e
test_sha256: 238facc68db5b0ca8d4911d6a60d5812e1907aca7231a16410d8a3adcf4398dc
source_sha256: 5daef933f9066e8d6821eb37f8f6fb80357a014013cc27e3d19344be3da3c984
```

原始结果及 stdout/stderr 由采集器保留在本机 `.beacon/captures/attempt-3101462718/`，
并登记在 node scope。本页是派生摘要，不替代已登记日志和结果。

## 判定

当前代码已经实现，测试通过不能构成 Red。此前主机终端的真实 Red 发生在采集合同配置和
采集器出现前，不能事后改写为本轮保留种类 capture_result。
因此本轮不创建假的 Red、不删除正确实现制造失败、不放松退出码要求、不推进 implemented/verified。
九叶均仍 frozen。其他叶子未继续批量采集；同一历史 Red 缺失问题需先决定合法迁移策略。

后续需要明确历史基线重放协议（将同一测试固定到真实旧源码 snapshot，清楚绑定旧/新候选）
或经 change 定义非 TDD/既有实现的验收合同。当前 API 不支持该协议；不能把批准提交
等同于批准豁免 Red。新迁移规则及实现必须单独经过确认与验证。
