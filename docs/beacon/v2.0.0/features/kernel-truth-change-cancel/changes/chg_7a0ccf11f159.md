---
change_id: chg_7a0ccf11f159
slug: kernel-truth-change-cancel
rebased_by: apple(via opencode)
rebased_at: 2026-11-11T13:26:21.125745Z
status: cancelled
created_by: apple(via opencode)
created_at: 2026-11-11T13:26:11.938088Z
base_truth_sha256: 38ffceea1eff20e46cded50687287773e0f2197150ecd6d0db2b6aef814c031d
base_package_fingerprint: f2cc9bb059ff2442463aa15e70927429a51a211d965192e7ec2c36101d8fad2c
base_requirement_fingerprints:
  AC-001: bfaa9d806661c2228fa19abea6edf46c0a23499164b5e5071c2e779f96b19dcc
  AC-002: 4211f53f1158b4fa856497d5c2f2684bd1a1fdc46df41854fac72ee6c79ea45e
  AC-003: 1f37d797b59cc3373a9c1dc3b0235fcd3e95e93724579ab464d3e82525304e83
  AC-004: 5557b8daae9408d56131b9b1a0cc428ccf92b417db41af2f5426c011ddd39a96
  AC-005: 9ba126b6f81ad207cf8cd4a495e0afe87cbbbd20dbbfadbcc71ec364ac6a415d

---

# Change Request: 记录 HEAD 推进到 199e875 并重跑 seed

记录 HEAD 推进到 199e875 并重跑 seed

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.

## Rebase Note

回退打包期文档改动，接受冻结正文为基线
