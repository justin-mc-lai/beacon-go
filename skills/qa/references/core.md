<!-- Source: distilled from Python v1.6.19 skills/beacon/beacon-qa/
     (SKILL.md hard gate + references/modes: eval-qa, eval-qa-deep,
     eval-agent, eval-real-project). Re-expressed for the Go qa face. -->

# beacon-qa — core reference

## Hard gate (Python original)

Evaluator only. Forbidden: rewrite truth to pass, silent
implement, release verdict.

## Mode index (Python) -> Go surface

| mode | Go surface |
|------|------------|
| eval-qa | `beacon qa evaluate` — the formal qa gate over registered evidence; `beacon qa autotest run` EXECUTES the tests.md commands and registers the per-TC evidence first |
| eval-qa-deep | deep acceptance routing (the run's qa_deep_routing state); the full deep face is still host-side |
| eval-agent | judging the working agent itself — the unified score engine's territory (`beacon qa score`), scores only, never edits |
| eval-real-project | validate against the real project, not a fixture — auto-test runs in the real project root for this reason |

## Evidence rules

- Evidence is a registered pointer + sha256, re-hashed at every
  gate read: a tampered artifact turns stale and blocks.
- A TC whose command only reads documents (docs-only) is not
  evidence of behavior; the integrity gate rejects it at freeze.
- Auto-test results are ordinary artifacts (kind test_result /
  test_results): the gate cannot tell them apart from hand-fed
  evidence, by design.
- Scores are health checks (structural 60 mechanical +
  effectiveness 40 human). A high score never rescues a blocked
  gate; a low score never voids a pass.
