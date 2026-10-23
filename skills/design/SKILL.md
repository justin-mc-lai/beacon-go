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
