---
name: beacon-qa
description: Evidence-based acceptance. Register real artifacts with hashes, evaluate the qa gate, and score skills without editing them.
triggers: 验收, QA, evidence, scorecard, 证据
---

# beacon-qa

1. Produce the artifact (report, test output, screenshot) and register it:
   `beacon goal artifacts register --run-id <id> --kind qa_report --path <file> --by <id>`
2. Evaluate: `beacon qa evaluate --run-id <id> --evaluator <id>` (persists the verdict).
3. Inspect: `beacon gate status --project . --feature <slug>`.
4. Optional skill health: `beacon qa score <skill>` records only; it never edits the skill.
5. Tampering is caught: every gate re-hashes registered artifacts and marks stale ones.

Rules: scores are health checks, not verdicts; qa never rewrites truth to pass.
