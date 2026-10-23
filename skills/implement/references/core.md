<!-- Source: distilled from Python v1.6.19 skills/beacon/
     beacon-implement/ (SKILL.md hard gate + references/modes/
     gen-implement.md). Re-expressed against the Go lanes. -->

# beacon-implement — core reference

## Hard gate (Python original)

Implement only after workspace admission + a frozen truth base.
Forbidden: QA self-pass, release claim.

## The one admission, the three lanes

Admission is a single bundle of checks shared by every entry
(`beacon implement admit` / `beacon goal admit`): frozen truth
(committed baseline), design binding when UX is required, topology
verdict, protected-path awareness, no undecided blocking review.
One scale, no lane gets a softer one.

| lane | when | Go surface |
|------|------|------------|
| single | one agent, one worktree | `beacon goal tick` with a configured stage executor |
| team | a handoff to another agent/host | `beacon goal delegate-ralph` pack + `receive-result` harvest: the handoff text is length-capped, the result must carry artifacts produced AFTER the pack |
| ralph | an autonomous loop on a command | real loop: exit 0 AND a stdout line `RALPH_DONE` completes an iteration's claim; harvesting re-validates like any lane |

## Worktree rules

- Satellites merge into the main star only, never sideways; a
  dirty main worktree refuses by default; unmerged satellites
  refuse cleanup.
- Protected paths (.env, keys, auth, payment, migrations) can
  never ride a satellite merge into the main line.
- Satellites never write truth on the main line.
