---
name: beacon-release
description: Final go/no-go. Release passes only when every gate is green and a human confirms; otherwise it stays blocked with named reasons.
triggers: 发布, release, 上线, gate
---

# beacon-release

1. `beacon release evaluate --run-id <id> --evaluator <id>` computes the gate.
2. Fix the named blocks (truth, design, implement, qa) — do not lower the bar.
3. `beacon release complete --run-id <id> --human-confirmed...` completes only at the
   release gate, with a verifier attached and the human confirmation recorded.

Rules: no human nod, no release. The release gate never infers consent.
