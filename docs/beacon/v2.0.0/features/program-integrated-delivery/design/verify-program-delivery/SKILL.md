---
name: verify-program-delivery
description: Verify Beacon Go's integrated Program handoff and approval invariants.
---

In the parent design directory run `java -cp "$HOME/.local/opt/tla/tla2tools.jar" tlc2.TLC -config ProgramDelivery.cfg ProgramDelivery.tla`. Expect all invariants to hold. Run the same command with `ProgramDeliveryBuggy.cfg ProgramDeliveryBuggy.tla` and require a `NoPrematureClaim` violation.

This is a bounded model of the frozen contract (`8eead81`), not of Go code. `claims` represents durable reservations; idling models a pause without a new reservation. `closed` tracks closures for the current candidate only, not erasure of prior audit records. Physical worktrees, conflicts, overlapping workers and crash points are verified by the Go tests in `../../tests.md` against the implementation candidate. Review `internal/orchestrator/{program_route,program_drive,program_complete,worktree}.go` and related CLI entry points.
