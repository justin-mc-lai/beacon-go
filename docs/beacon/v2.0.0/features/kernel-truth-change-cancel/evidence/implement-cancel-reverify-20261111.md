# Implement evidence: kernel-truth-change-cancel（取消轮复验 · R2）

- 仓库：/Users/apple/Developer/Personal/products/beacon-go
- HEAD：cb7805f（派单写 f9ce5c5；此后 4918f1a、83978d9、d02d219 已把本 feature 落在 main）
- 时间：2026-11-11T14:35Z（Asia/Shanghai 22:35）
- 工具链：go1.27.3 darwin/arm64
- goal run：run_0ddd54ba93ff41cf9f27ea99，实测 stage: release / loop_stage: release_gate / blocked: false

## 起点

派单口径是「--cancel 尚不存在，三条 seed 红」。核对时控制流已由另一条线在 4918f1a 落地（CancelTruthChange + --cancel/--accept-current），三条 seed 在该提交后已能 PASS。本轮没有重写控制流。

R1 补齐了取消事件字段（decision=cancel、change_id、change_doc、base_truth_sha256）。R2 按返工裁定修了三处：

| 文件 | 改动 |
|------|------|
| internal/orchestrator/truth_change.go | 删除成功路径对 changes/<id>.md 的 status: pending → cancelled 回写（AC-005 字面：不改写变更文档）；事件只追加，正文与变更文档保持字节不变 |
| internal/cli/truth.go | RunE 最前面（openWriteStore 之前）加普通 usage error：--accept-current 单独出现时 return fmt.Errorf("--accept-current only applies with --cancel")，exit 1；未新增 ReasonCode |
| internal/orchestrator/truth_cancel_test.go | 成功取消后断言 changes/<id>.md 字节与取消前完全一致；--accept-current 成功路径同样断言字节不变 |

未改 seed、未改 lib.sh、未新增 reason code、未改 truth/tests/tasks、不自证 QA、不动 release。

## 构建

```
$ go build -o /tmp/beacon-cancel-verify2 ./cmd/beacon
BUILD_OK
$ /tmp/beacon-cancel-verify2 truth change --feature x --accept-current
Error: --accept-current only applies with --cancel
RC=1
```

## seed 实测（R2，当前构建）

命令（各自 mktemp 隔离 store 与项目；SEED_CANON_DIR 指向临时目录，避免覆盖仓库内红基线）：

```
BEACON_BIN=/tmp/beacon-cancel-verify2 SEED_CANON_DIR=$CANDIR zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh
BEACON_BIN=/tmp/beacon-cancel-verify2 SEED_CANON_DIR=$CANDIR zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed2_cancel_modified_refuses.sh
BEACON_BIN=/tmp/beacon-cancel-verify2 SEED_CANON_DIR=$CANDIR zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed3_cancel_no_pending_idempotent.sh
```

seed1，exit=0：

```
c1_cancel_rc=0
c1_status=frozen
c1_pending_ids=[]
c1_cancel_events=1
c2_norefuse_rc=1  err=...no base fingerprint; re-run with --accept-current...
c2_accept_rc=0
c2_accepted_current=True
c2_baseline_missing=True
c2_body_unchanged=yes
sha256=be8b20fbf2f2c3a2cfc2e89713324edcbcf649053c28424cbaebd5b7cf375d56
RESULT: PASS - identical body cancels to frozen; legacy change needs and accepts --accept-current
```

seed2，exit=0：

```
cancel_rc=1
status=change_pending
cancel_events=0
refrozen_events=0
body_untouched=yes
change_doc_present=yes
err=...has a modified body...; run `beacon truth refreeze` instead of cancelling
sha256=bb2ac42049964f08137df162d1d15b8aa825c71049a5d025b74e72b3e4db51c5
RESULT: PASS - modified body is refused, change_pending kept, refreeze route named, body untouched
```

seed3，exit=0：

```
rc_first=0 rc_second=0
stderr_first= stderr_second=
status_first=frozen status_second=frozen
cancel_events_first=0 cancel_events_second=0
requested_events_first=0 requested_events_second=0
body_unchanged_first=yes body_unchanged_second=yes
sha256=18fb6b60cd3d41d5d44f8d7650c37a5f772a0b252b78628b8d05f91a968fbeea
RESULT: PASS (guard) - no-pending cancel is a successful no-op, twice
```

本轮绿 canon 另存（不覆盖既有）：

| 新文件 | canon sha256 |
|--------|--------------|
| evidence/seed1.canon.20261111-be8b20fb.txt | be8b20fbf2f2c3a2cfc2e89713324edcbcf649053c28424cbaebd5b7cf375d56 |
| evidence/seed2.canon.20261111-bb2ac420.txt | bb2ac42049964f08137df162d1d15b8aa825c71049a5d025b74e72b3e4db51c5 |
| evidence/seed3.canon.20261111-18fb6b60.txt | 18fb6b60cd3d41d5d44f8d7650c37a5f772a0b252b78628b8d05f91a968fbeea |

既有的 seed*.canon / seed*.out 是 e6536fd 的历史红基线（unknown flag: --cancel），按 DoD 保留未覆盖；上游 spine artifacts 仍把它们标为 stale(hash_mismatch)，属预期。canon 内含随机 chg_ id，原始 sha 每轮不同，跨轮只比较字段值。

## AC-005 CLI 级字节验证

隔离 fixture 下取消前后对 changes/<id>.md 取 sha256：

```
change_id=chg_e2ae3a609028
doc_sha_before=e34052d124809ee3fc316d385b2ce214ca7f0bb78c6c9ee751210db43eedebfc
doc_sha_after=e34052d124809ee3fc316d385b2ce214ca7f0bb78c6c9ee751210db43eedebfc
doc_status_line=status: pending
RESULT: PASS - changes/<id>.md byte-identical after cancel
```

## 新增单测

```
$ go test ./internal/orchestrator -run 'TestTruthCancel' -count=1
ok  beacon-go/internal/orchestrator  2.292s
--- PASS: TestTruthCancelIdenticalBodyReturnsFrozen (含 AC-005 字节断言)
--- PASS: TestTruthCancelModifiedBodyRefuses
--- PASS: TestTruthCancelMissingBaselineNeedsAcceptCurrent (成功路径含 AC-005 字节断言)
--- PASS: TestTruthCancelWithoutPendingIsIdempotent
```

## go test ./...

```
?   	beacon-go/cmd/beacon	[no test files]
ok  	beacon-go/desktop	0.696s
ok  	beacon-go/internal/agentapi	3.839s
ok  	beacon-go/internal/benchgate	(cached)
ok  	beacon-go/internal/changequality	(cached)
ok  	beacon-go/internal/cli	17.752s
ok  	beacon-go/internal/core	(cached)
ok  	beacon-go/internal/effectjournal	(cached)
ok  	beacon-go/internal/evalgate	61.334s
ok  	beacon-go/internal/gate	(cached)
ok  	beacon-go/internal/harness	13.372s
?   	beacon-go/internal/httpapi	[no test files]
ok  	beacon-go/internal/llm	(cached)
ok  	beacon-go/internal/migration	(cached)
ok  	beacon-go/internal/orchestrator	115.089s
ok  	beacon-go/internal/progressreview	(cached)
ok  	beacon-go/internal/session	(cached)
ok  	beacon-go/internal/store	(cached)
ok  	beacon-go/internal/stride	(cached)
ok  	beacon-go/internal/workspace	4.146s
ok  	beacon-go/skills	(cached)
```

## R1 遗留观察的处置

1. AC-005 歧义：按返工裁定取字面优先，删除 status 回写；变更文档成功路径也保持字节不变，单测与 CLI 各有一条字节断言。truth/tests 未改。
2. `--accept-current` 单独出现：现已 exit 1 并提示只能与 `--cancel` 同用，未新增 reason code。
3. 红基线 canon：保留原文件，新增本轮绿 canon 带日期与 sha8 后缀；证据里写明跨轮只比字段值。
