# Parallel code review contract

The kernel does **not** spawn reviewers. The external host runs them; the
kernel records the aggregate as evidence. Parallel means the host actually
started reviewers concurrently, not that a command lists several names.

## Fan-out

- One commit and one scope per review round.
- Each reviewer gets an isolated git worktree or a read-only snapshot.
- Reviewer identity must differ from the author identity.
- If the host cannot run reviewers concurrently, execute sequentially and say
  `execution_mode=sequential`.

## Reviewer receipt

Each reviewer writes one JSON document:

```json
{
  "schema": "beacon-code-review.v1",
  "reviewer": "reviewer-a",
  "execution_mode": "parallel",
  "commit": "<sha>",
  "scope": ["path/one", "path/two"],
  "verdict": "pass|block",
  "findings": [
    {
      "id": "F-1",
      "severity": "blocker|major|minor",
      "file": "path/file.go",
      "line": 42,
      "ac": "AC-001",
      "summary": "what is wrong",
      "evidence": "test output, reproduction, or code reference"
    }
  ]
}
```

A receipt with an invalid verdict, missing commit, or non-existent evidence is
invalid; invalid is not a pass.

## Aggregation

The aggregator writes `code-review.json`:

- preserves every receipt plus its SHA-256;
- de-duplicates the same finding across reviewers without dropping dissent;
- carries the highest severity;
- reports `verdict: block` if any reviewer blocks and no explicit human waiver
  is recorded;
- never rewrites or deletes a reviewer receipt.

Register the aggregate under the feature scope, then evaluate:

```bash
beacon goal artifacts register --feature <slug> --kind qa_report \
  --path code-review.json --by <aggregator>
beacon verify evaluate --feature <slug> --evaluator <independent-id>
```

## Refusals

- No reviewer receipt, no code-review verdict.
- No author self-review.
- No "parallel" claim when reviewers ran sequentially.
- No pass by majority vote when a blocker remains.
- No release approval from a code review alone; release still needs its gate
  and a human confirmation.
