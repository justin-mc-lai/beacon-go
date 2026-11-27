# Verification evidence

Every `verified` touchpoint must point at an artifact in this directory.
`.beacon/artifacts/` is gitignored runtime state and can never be the
proof of a committed claim.

`beacon verify leaf --node <id> --artifact <file>` moves a raw
transcript here (`evidence/<node-id>/<file>`) and stamps the header:

```text
source_commit: <full 40-hex git commit>
binary_sha256: <full 64-hex sha256 of the running beacon binary>
requirement_fingerprint: <contract fingerprint of the tree node>
source_state: committed | dirty
```

`verify leaf` refuses dirty code (`internal/`, `cmd/`) or dirty
requirement packages (`features/`), an artifact outside the repository,
a `.beacon/` artifact, a missing header, or a `requirement_fingerprint`
that no longer matches the node contract. Tree status and this evidence
directory are excluded because verification writes them itself.
`verify audit` re-checks the same contract for every verified leaf on
each run.
