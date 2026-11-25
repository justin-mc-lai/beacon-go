# Parallel code review (chain: full-delivery)

Input: an inherited repository with three risky changes.

1. The host creates three isolated worktrees at the same commit and launches
   three independent reviewers in parallel.
2. Each reviewer writes a `beacon-code-review.v1` receipt with a verdict,
   findings, evidence, and the commit. None is the author.
3. The aggregator de-duplicates findings, preserves dissent, and keeps every
   receipt hash. One blocker keeps the aggregate `block`.
4. Register `code-review.json` under the feature scope, then run
   `verify evaluate` with an independent evaluator.
5. If the host ran reviewers one after another, the aggregate says
   `execution_mode=sequential`; it does not claim parallel review.

Refusal: a list of reviewer names is not evidence; a missing receipt is not a
pass, and a code review is not a release approval.
