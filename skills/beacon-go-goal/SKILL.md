---
name: beacon-go-goal
description: Long-run delivery for one feature. Use when the user says goal, 长跑, 全流程, deliver this feature, or drive it to release; it turns one utterance into a staged run (plan → truth → implement → verify → release), survives interruption, and never completes without an independent verifier and a human-confirmed release.
---

# beacon-goal

Use `goal` when the work will outlive one turn.

## HARD GATE

- 严格交付（编排）：先完整读取 `../beacon-go-verify/references/strict-delivery.md`；
  以六点协议核对阶段证据和账本，再记录推进；未验证不记为完成。

- Orchestrate only: never write product code, never judge your own work, never release.
- Goal routes investigation to plan and canonical formation/HTML review/freeze to
  truth; subsequent node edits go through change. The host proposes and executes.
- 用户只决定目标、取舍、批准。宿主读取内核 next/missing，自行完成技术映射、
  材料准备和必要绑定，不让用户重复绑定或解释技术清单。内核不调 LLM、不跑 agent。
- No stage advances because time passed. It advances when the stage exit gate
  passes; a filled form is not a pass.
- Completion needs all four: at `release_gate` + human confirmation on record +
  an independent verifier attached (maker != checker) + latest `verify` verdict pass.
  `--force` bypasses none of them.
- With no executor registered the run parks at `plan` and says so. Parking is
  correct; inventing progress is not.

## Workflow

1. `beacon goal run "<utterance>" --project . --feature <slug> --executor-id <id>`
   creates the run and its stage plan (plan → truth → implement → verify → release).
2. The host does the stage work; `beacon goal tick --run-id <id> --by <identity>`
   records it and advances one stage. The tree stage gate blocks on draft or
   unverified nodes.
3. Read state only through `beacon spine show --project . --feature <slug>`.
4. Blockers are recorded, not hidden: `beacon goal block --reason-code ...`.
5. Completion requires `attach-verifier` + verify pass + `complete` at the release gate.
6. plan 调查无需树或 HTML；truth 阶段形成本轮 canonical 树并必交 HTML：
   `beacon truth tree --project <root> --version <v> --format html`。
   复用同一树，版本保留不可变 baseline + 本轮 delta；future draft 可共存，
   不属于本轮依赖闭包就不拖本轮发布。若内核仍阻塞，报告实际缺项，不伪造推进。
7. 使用当前 run 的 `goal workflow check --run-id <id>` 读取 next/missing，
   宿主完成动作后再 `goal tick --run-id <id> --by <identity>`；
   文档状态、prepare 成功或文件存在都不能冒充阶段交付。

## Load before acting

Read `references/core.md` completely before the first command. It carries the
run's shape, the reason codes and the exit conditions. Do not act from this
summary alone.

## Output contract (minimum)

Every turn ends with:

- `mode_id`: `loop-goal`
- `run_id` and the stage you actually advanced (or `blocked` + reason code)
- `current_stage`, `existing_artifacts`, `next_action`: read the explicit run,
   list real artifacts (including HTML), name the next executable action or human decision
- `owner`: the host responsible for the next action, or the human when a goal/scope/approval decision is genuinely needed
- `evidence_refs`: commands run, files read, artifacts registered
- `next`: one of `goal|plan|truth|implement|verify|release|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the truth package or the code you are about to change.
2. You would report "done/passed" without a command output or a registered artifact.
3. The only change is documentation, comments, or renaming.
4. You are about to write "should work", "应该没问题", "大概可以" instead of running it.
5. No frozen AC covers the work and no owner decision exists.
6. You are re-planning work that is already planned instead of executing it.
7. You are waiting for a human but have not recorded the blocker
   (`beacon goal block --reason-code ...`).
8. You would mark a gate pass you did not evaluate.
