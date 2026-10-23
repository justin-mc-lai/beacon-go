---
name: beacon-plan
description: Think first, in the open. Socratic questions before freeze, multi-angle planner review, source-anchored research, sea-to-lake program planning. The planner never implements, never freezes truth, never issues a QA or release verdict.
triggers: 规划, 先想清楚, plan, review, brainstorm, 审查, 拆海, program, 源码
---

# beacon-plan

Planning is a harness of its own — it ends at a signed intent and a
reviewed plan, never at code. Modes (pick exactly one primary mode,
then work it):

| mode | what it does | Go surface |
|------|--------------|------------|
| pln-brainstorm | Socratic questions + first-principles decomposition before freeze | `beacon truth ask` / `beacon truth answer` (one question at a time, each with a recommendation) |
| pln-review | multi-angle planner review of a plan or package | `beacon goal planner-review` / `beacon qa planner-review` (real subprocess reviewers, one block blocks all) |
| pln-source | source-anchored research — read the code, never invent | host reads the repo; findings land in the intent (`beacon truth intent`) |
| pln-program | sea-to-lake: split the big thing into boilable lakes | `beacon goal program plan` (dependency graph; cycles and missing deps refused) |
| pln-friction | capture friction into the improvement input | record it as a truth question (`beacon truth ask`) until intake exists |

The chain inside a package is **ask -> sign**: questions close one
by one, the intent snapshot records the user's own words, goal,
non-goals and constraints, and `beacon truth confirm` refuses to
sign while questions stay open or an AC has no intent source. The
planner proposes; the user owns scope and freeze.

Hard rules (from the Python beacon-plan core):

1. Lake vs sea — boil lakes; split or defer seas, never boil a sea.
2. Search before invent — truth, source, evidence, memory first.
3. No fake delivery — placeholders and docs-only evidence are not a plan.
4. Stay in the harness: planner != implementer; route onward to
   beacon-truth / beacon-goal instead of stretching this skill.
