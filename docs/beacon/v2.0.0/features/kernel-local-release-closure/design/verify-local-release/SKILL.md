---
name: verify-local-release
description: Check single-machine release approval, preimage, backup and restart invariants.
---

From the parent directory run `java -cp "$HOME/.local/opt/tla/tla2tools.jar" tlc2.TLC -config LocalRelease.cfg LocalRelease.tla`. All properties must hold. Remove the `approved` guard and phase check from Begin to construct a negative control; the invariant must fail. Check CLI tests for evidence and binary tampering, competing installers, stale preimage, crash after replace, missing target and preservation of working files.

The bounded model describes the contract, not a proof of filesystem or Go/Python implementation. Values 0/1/2 identify old/candidate/unknown contents; Pause represents interruption, not deletion. External drift may happen even after installed; historical installation is not a claim of current alignment. Inspect the implemented installer and record the implementation commit and model limitations with independent QA.
