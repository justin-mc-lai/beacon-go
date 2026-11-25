# Implement core reference

## Entry

- `beacon goal admit --run-id <id>` is the shared admission check.
- A blocked admission stops every write.
- `beacon implement worktree create|merge|cleanup|status` isolates parallel work.
- Satellites merge into the main star only; protected paths never merge.

## Execution

The kernel does not run agents. The host does the work, then records it:

```bash
beacon implement report --project . --node <touchpoint> --by <identity> \
  --artifact <path> --outcome done
```

Rules:

- Only frozen touchpoints can be reported.
- Success moves `frozen -> implemented`; failure keeps the frozen fact and a reason code.
- Repeating a successful report is idempotent.
- An artifact path is required for success; the kernel records its SHA-256.
- Evidence is registered under the node scope; `verify leaf` then moves it to verified.

## Refusals

- No admission, no write.
- No artifact, no success claim.
- Never edit `index.json` or `node.json` by hand.
