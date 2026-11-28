# 本机安装候选独立检查

verdict: PASS
checker: independent-install-qa
maker: implementation-host
source_commit: 62bfca1ca79970607e92636730123154034b9037
binary_sha256: f53352f9fc25ead7289d3410c7d6a7f22d1d9d455ebf4d7a0944a2720006b2d5

独立Task session：ses_ddf0a1a09ffeEIjRcRxEkwI5NU，只读，未安装、未写文件。
检查二进制hash、版本、workflow/grow/continue help和内嵌plan/truth skill阶段规则通过。

实跑：
`go test ./internal/cli ./internal/orchestrator ./skills -run 'TestStableWorkflow|TestChangeContinueApplyTwoDeliveries|TestChangeGrowDeprecateRefreeze|TestCaptureSelector|TestTreeWorkflow' -count=1`

```text
ok beacon-go/internal/cli 7.601s
ok beacon-go/internal/orchestrator 0.539s
ok beacon-go/skills 0.797s
```

范围：本机安装前候选功能及阶段规则检查，不等于全树verified、正式发布gate或真实Portfolio交付通过。
第一次缺Go PATH，补充已有SDK后实跑通过。源码/skills干净，已有e2e日志改动不属于源码候选。
