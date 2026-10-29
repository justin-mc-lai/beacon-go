---
name: beacon-design
description: Design-primitive binding gate for UI work. Beacon tracks whether the visual rules are bound; it never draws prototypes.
triggers: 设计, design, design primitive, UI gate
---

# beacon-design

1. `beacon design status --run-id <id>` tells you whether design is required and bound.
2. `beacon design bind --run-id <id> --primitives <json/file> --by <id>` records the binding.
3. Implement admission stays blocked until the binding is present when the run requires design.

Boundary: the kernel does not generate prototypes; the host agent does that work.

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority):

- `examples/no-ui-no-design.md` — chain `honest-block`: no UI means no design stage and no invented primitives.
- `examples/bind-before-implement.md` — chain `full-delivery`: primitives are bound before implement admission.
- `examples/insert-design-stage.md` — chain `full-delivery`: a UX requirement inserts the design stage into the run.

## HARD GATE

- No binding, no implement: when the run requires design, admission stays blocked
  until `design bind` records the primitives.
- Never draw or generate the prototype here; the kernel judges "are the rules
  bound", not "does the mockup look good".
- Never bind placeholders ("TBD", empty colour list) to unblock yourself.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
binding contract and the reason codes. Do not act from this summary alone.

## Output contract (minimum)

- `mode_id`: `ds-check|ds-bind`
- `run_id` and the binding status before/after
- `evidence_refs`: the primitives file and the command output
- `next`: one of `implement|design|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the requirement package the UI belongs to.
2. You would claim "design bound" without the `design status` output showing it.
3. The only change is a colour/spacing tweak with no primitive recorded.
4. You are about to write "样式应该没问题" instead of checking the binding.
5. The run does not require design and you are inventing a design stage.
6. You would bind placeholder values to pass admission.
7. You are waiting for the user's visual decision but have not asked it.
8. You would mark the design gate pass you did not evaluate.

