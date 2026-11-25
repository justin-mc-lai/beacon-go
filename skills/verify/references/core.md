# Verify core reference

## Evidence

- Only registered artifacts count.
- Registration is explicit: `beacon goal artifacts register --scope-kind node|feature --scope-id <id> --kind <kind> --path <file> --by <id>`.
- Every gate re-hashes the file; a changed file turns stale and blocks.

## Leaf verification

```bash
beacon verify leaf --project . --node <touchpoint> --artifact <file> --by <identity>
```

- The leaf must already be `implemented`.
- The artifact id and hash are stored on the node.

## Parent rollup

```bash
beacon verify rollup --project . --node <parent>
```

- A parent verifies only when every child is verified.
- The rollup rebinds the parent's child fingerprints.

## Audit

```bash
beacon verify audit --project . --json
```

Read-only. Checks kind rules, duplicate ids, orphans, empty modules,
touchpoint verification scope, and fingerprints.

## Scores

`beacon verify score <skill>` records a health check. It never edits the skill
and never passes or blocks a gate.
