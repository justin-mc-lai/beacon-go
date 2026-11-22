# Release evidence — kernel-qa-evidence-hardening @ 87e951a

verdict: PASS
checker: QA 验收 · 回归
maker: omb:kernel-qa-implementer
source_commit: 87e951a9a03507b2de8c393fef50a357c8eba97a
binary_sha256: 04d41b58b72f605965ef2b4d107a2e546d853e3322aa37160fa03d0083124e9b
run_id: run_932e20c89ae9c375a7a5cab5

## Authority

The verdict, the four AC results and the executed commands are the independent
checker's (QA 验收 · 回归), recorded in messages.db thread
9e3ba2af-4e6c-403d-a9b5-41ea7d79c544. The checker signed
`[VERDICT] PASS dsp_20261122034012_7351` — a dispatch id it invented, so the
dispatch board did not close; this file does not reuse that id.

The delivery manager only (a) transcribed the checker's Markdown report into the
`beacon-product-qa.v1` JSON the kernel gate requires (`transcribed_by` is recorded
inside it) and (b) re-ran the checker's isolated success/rejection chain to produce
live CLI surface evidence. No AC was re-judged by the manager.

## Bound artifacts

| artifact | sha256 |
|---|---|
| qa-87e951a.md | 25a140d62e8d3902bfcb97755f56591d0b2386f4c48a2037d3461adffdd78062 |
| qa-87e951a.json | d4bf96c3becc1e96995bd1dae8060ef5339ce3fdd2f5179d45b7f03e34b4be1b |
| qa-87e951a-e2e.md | ca8452a1d1f520c4de5be8661e4fff9fd677b7ed9a32f7470983a8775c8e183a |

## Gates that consumed this

- `goal artifacts register --kind qa_candidate` → candidate content hash 5e357616…
- `qa autotest run` → 4/4 TC passed, bound to that candidate hash
- `goal attach-verifier --source "QA 验收 · 回归"` → present
- `gate evaluate qa` → pass
- `goal tick` → implement → qa → release
