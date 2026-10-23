<!-- Source: distilled from Python v1.6.19 skills/beacon/beacon-design/
     (SKILL.md hard gate + references/modes index: gen-design,
     gen-design-init, gen-design-explore, gen-design-system,
     gen-design-polish, gen-prototype). The Go kernel tracks the
     binding contract only; drawing belongs to the host. -->

# beacon-design — core reference

## Hard gate (Python original)

Design/prototype artifacts only. Forbidden: silent truth rewrite,
implement claimed as design completion.

## Mode index (Python)

| mode | purpose |
|------|---------|
| gen-design-init | set up the design surface for a feature |
| gen-design-explore | explore directions before committing |
| gen-design-system | the design-system / primitive layer |
| gen-design | produce the design itself |
| gen-design-polish | polish pass over an accepted design |
| gen-prototype | prototype generation through the host bridge |

## The Go kernel's boundary

The kernel minds exactly three questions — are the design
primitives (the frozen visual rules) DEFINED, BOUND to this run,
and SUFFICIENT to start implement:

- `beacon design status --run-id <id>` / `beacon goal design-stage status`
- `beacon design bind --run-id <id> ...` (same constructor)

No binding, no implement: the implement admission bundle checks
the binding. Prototype drawing and UI authoring happen host-side
via the design bridge; nothing in the kernel renders.
