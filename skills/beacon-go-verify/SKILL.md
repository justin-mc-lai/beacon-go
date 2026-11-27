---
name: beacon-go-verify
description: Evidence-based verification and code review. Use for 验收, QA, evidence, scorecard, 证据, code review, 代码审查, or 多代理评审; register real artifacts with hashes, aggregate independent reviewer findings, evaluate the qa gate, verify leaves and roll parents up, and score skills without editing them.
---

# beacon-verify

1. Produce the artifact (report, test output, screenshot) and register it:
   `beacon goal artifacts register --run-id <id> --kind qa_report --path <file> --by <id>`
2. Evaluate: `beacon verify evaluate --run-id <id> --evaluator <id>` (persists the verdict).
3. Inspect: `beacon gate status --project . --feature <slug>`.
4. Optional skill health: `beacon verify score <skill>` records only; it never edits the skill.
5. Tampering is caught: every gate re-hashes registered artifacts and marks stale ones.

## Parallel code review (qa-code-review)

Use this mode for an inherited codebase or a large diff. The **host** fans out
reviewers; the kernel does not spawn agents, and it never claims parallelism it
did not observe.

1. Give each reviewer an isolated worktree or read-only snapshot at one commit.
   The reviewer must not be the author of the change.
2. Launch the reviewers in parallel if the host supports it. Each writes one
   receipt: reviewer, commit, scope, verdict, findings, evidence.
3. The aggregator preserves receipt hashes, de-duplicates findings, keeps
   dissent, and marks any blocker. A missing or malformed receipt is not a pass.
4. Register the aggregate JSON:
   `beacon goal artifacts register --feature <slug> --kind qa_report --path <aggregate.json> --by <id>`
   Then evaluate: `beacon verify evaluate --feature <slug> --evaluator <id>`.
5. If the host cannot run reviewers in parallel, run them sequentially and
   state `execution_mode=sequential`; never describe that as parallel review.

A review verdict is evidence, not a release approval. Any unresolved blocker
stays a blocker; a human waiver must be recorded explicitly.

Rules: scores are health checks, not verdicts; qa never rewrites truth to pass.

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority):

- `examples/register-then-evaluate.md` — chain `full-delivery`: register the artifact, then evaluate the qa gate.
- `examples/failed-verdict.md` — chain `honest-block`: a blocked verdict is reported with its reason codes, not retried silently.
- `examples/score-does-not-edit.md` — chain `honest-block`: a score is a health check and never edits the skill or passes a gate.
- `examples/parallel-code-review.md` — chain `full-delivery`: independent reviewers run on isolated worktrees, receipts are aggregated, and the kernel only records evidence.

## HARD GATE

- 严格交付（验收）：先完整读取 `references/strict-delivery.md`；
  核对六点协议的真实执行证据，宿主启动全新上下文只读盲审；缺证据不判通过。

- Only registered artifacts count. A file on disk that was never registered is
  not evidence; a gate re-hashes what is registered and marks the rest stale.
- Scores never pass or block a gate, and never edit a skill.
- Never rewrite the truth package, the tests, or the artifact to make a verdict pass.
- A failed verdict is a result to report, not a step to retry silently.
- A code review is not complete without one receipt per reviewer and an
  aggregate artifact whose hash is registered. The kernel did not run the
  reviewers; do not claim it did.

## Load before acting

Read `references/core.md` completely before the first command. For code
review, also read `references/code-review.md` completely; it carries the
receipt schema, fan-out rules and aggregation contract.

## Output contract (minimum)

- `mode_id`: one of `qa-gate|qa-review|qa-score|qa-code-review`
- `verdict` with its reason codes, or the score with its dimensions
- `evidence_refs`: registered artifacts (path + hash) and the command output
- `next`: one of `implement|release|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the artifact you are about to judge.
2. You would report "pass" without a gate evaluation output.
3. The evidence is a document describing tests rather than test output.
4. You are about to write "测试应该都过了" instead of running them.
5. The tests do not cover the AC they claim to cover.
6. You are the maker of the code you are judging.
7. You are waiting for evidence that nobody produced, and have not said so.
8. You would score a skill and treat the score as a verdict.
