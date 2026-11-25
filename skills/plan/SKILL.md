---
name: beacon-go-plan
description: Think first, in the open. Socratic questions before freeze, multi-angle planner review, source-anchored research, and intent-tree planning. The planner never implements, never freezes truth, never issues a QA or release verdict.
triggers: 规划, 先想清楚, plan, review, brainstorm, 审查, 意图树, 源码
---

# beacon-plan

Planning is a harness of its own — it ends at a signed intent and a
reviewed plan, never at code. Modes (pick exactly one primary mode,
then work it):

| mode | what it does | Go surface |
|------|--------------|------------|
| pln-brainstorm | Socratic questions + first-principles decomposition before freeze | `beacon truth ask` / `beacon truth answer` (one question at a time, each with a recommendation) |
| pln-fsm | Domain FSM decomposition: entity precedence, aggregate transitions, illegal slots | `beacon truth fsm check` / `beacon fsm transition` (mandatory for any workflow, order, dispatch, approval, lifecycle, or multi-state feature) |
| pln-review | multi-angle planner review of a plan or package | `beacon goal planner-review` / `beacon verify planner-review` (real subprocess reviewers, one block blocks all) |
| pln-source | source-anchored research — read the code, never invent | host reads the repo; findings land in the intent (`beacon truth intent`) |
| pln-tree | intent-tree planning: root intent -> module -> feature -> touchpoint | `beacon plan init` / `plan expand` / `plan apply` (kind rules + <=7 children per expansion) |
| pln-friction | capture friction into the improvement input | record it as a truth question (`beacon truth ask`) until intake exists |

The chain inside a package is **ask -> sign**: questions close one
by one, the intent snapshot records the user's own words, goal,
non-goals and constraints, and `beacon truth confirm` refuses to
sign while questions stay open or an AC has no intent source. The
planner proposes; the user owns scope and freeze.

Hard rules (from the Python beacon-plan core):

1. Tree over prose — expand the intent tree; never invent a flat feature list.
2. Search before invent — truth, source, evidence, memory first.
3. No fake delivery — placeholders and docs-only evidence are not a plan.
4. Stay in the harness: planner != implementer; route onward to
   beacon-truth / beacon-goal instead of stretching this skill.

## Tree planning (pln-tree)

1. Create the root once: `beacon plan init --project . --title <title> --intent <intent>`.
2. Expand one draft parent at a time with a proposal JSON:
   `beacon plan expand --node <id> --children <proposal.json>` validates only;
   `beacon plan apply --node <id> --proposal <proposal.json>` writes drafts.
3. A proposal carries `provenance` (model/prompt hash/skill/tokens/cost); the host
   agent produces it, the kernel records it.
4. Never exceed 7 children per expansion; never skip a kind
   (`intent -> module -> feature -> touchpoint`).

## Route examples

Three worked routes; open the file before acting (they are examples, not
requirement authority — the frozen package is):

- `examples/vague-requirement.md` — chain `honest-block`: a vague request is turned into questions, not guesses.
- `examples/domain-fsm-decomposition.md` — chain `domain-fsm`: workflow/lifecycle features must decompose entity precedence, aggregate transitions, and illegal slots.
- `examples/vague-requirement.md` — chain `honest-block`: a one-line request without an outcome stops at questions.
- `examples/source-anchored-review.md` — chain `full-delivery`: every claim in a plan cites a file or command output.

## HARD GATE

- Any request that involves workflow, state transition, dispatch, lifecycle, approval, or order flow MUST trigger `pln-fsm`: decompose Entity Precedence, Aggregate FSM, and Illegal Transition Slots before routing to truth freeze.
- Planning ends at a signed intent and a reviewed plan. Never at code, never at a
  frozen package, never at a QA or release verdict.
- One question at a time, each with a recommendation. Never bundle five questions
  to look productive.
- A sea is split or deferred, not boiled. If the ask cannot be finished in one
  lake, say which lakes and in what order.
- Never write the plan from imagination: read the truth package, the code, the
  evidence, the memory first.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
mode definitions, the ask/sign chain and the boundaries. Do not act from this
summary alone.

## Output contract (minimum)

- `mode_id`: one of `pln-brainstorm|pln-fsm|pln-review|pln-source|pln-tree|pln-friction`
- the artifact for that mode (question list, FSM matrix, review verdict, findings, lake graph)
- `evidence_refs`: files read, commands run, sources cited
- `next`: one of `truth|goal|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the truth package or the code the plan talks about.
2. You would present a plan without naming what is explicitly out of scope.
3. The "plan" is a restatement of the request with no decisions in it.
4. You are about to write "方案应该可行" instead of checking it against the repo.
5. You would answer your own question on the user's behalf.
6. You are re-planning work that is already planned instead of executing it.
7. You are waiting for a decision but have not asked for it in one clear question.
8. You would drift into implementing or freezing because planning feels slow.

