---
name: beacon-go-implement
description: Implementation lanes behind one admission check and one harvest rule. Use for 实现, implement, 写代码, worktree, or 回报; single and team lanes share the entry gate and result contract, and executed work is reported with an artifact.
---

# beacon-implement

1. `beacon implement plan --run-id <id>` shows the lane routing and admission verdict.
2. `beacon implement admit --run-id <id>` is read-only; a blocked admission stops all writes.
3. `beacon implement worktree create|merge|cleanup|status --run-id <id>` isolates parallel work.
   Satellites merge into the main star only; protected paths never merge.
4. Do the work in the admitted worktree, then report it:
   `beacon implement report --project . --node <touchpoint> --by <id> --artifact <file>`.
5. Verification evidence goes through `beacon verify leaf`, never by editing node JSON.

Rules: no admission, no implement; no evidence, no completion claim.

Go 测试真实采集：`beacon implement capture --project . --version <v> --node <leaf>
--ac <AC> --phase red|green --by <maker>`。Green 必须带 `--red <artifact-id>`；
命令取自冻结节点的 `beacon-capture-command.v1` 验证规格，不接收手填退出码。
捕获 accepted 仅表示本次阶段有效，不等于 verified；接口与限制见仓内 `docs/capture-runtime.md`。

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority):

- `examples/admission-blocked.md` — chain `honest-block`: an unfrozen package or missing admission blocks before code is written.
- `examples/parallel-worktrees.md` — chain `full-delivery`: parallel lakes stay in separate worktrees; satellites merge into the main star.

## HARD GATE

- 严格交付（执行）：先完整读取 `../beacon-go-verify/references/strict-delivery.md`；
  按六点协议执行确认计划、原子任务、Red/Green、断言保护、证据与账本核销。
  缺检查或独立审查回执时，只报告“未验证”及阻塞。

- No admission, no write. A blocked admission stops every lane, not just one.
- Satellites merge into the main star only; protected paths (`.env`, keys, auth,
  payment, migrations) never merge.
- The maker never certifies: the same executor that wrote the code may not attach
  the verifier or pass the qa gate.
- Never mark a stage done without an artifact registered under the harvest rule.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
lane routing, the admission items and the harvest rule. Do not act from this
summary alone.

## Output contract (minimum)

- `mode_id`: one of `im-single|im-team|im-ralph`
- `run_id`, the lane, and the worktree path you actually used
- `evidence_refs`: commits, commands, registered artifacts
- `next`: one of `qa|implement|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the truth package or the files you are about to change.
2. You would claim "implemented" without a command output or a commit hash.
3. The only change is documentation, comments, or renaming.
4. You are about to write "应该能跑", "大概没问题" instead of running the build and tests.
5. Admission is blocked and you are looking for a way around it.
6. You are re-planning instead of writing the code the plan already names.
7. You are blocked by a dependency but have not recorded it.
8. You would attach a verifier to your own work.
