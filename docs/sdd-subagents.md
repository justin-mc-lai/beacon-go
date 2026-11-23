# SDD subagent plan (on demand)

Source research: [VoltAgent/awesome-claude-code-subagents](https://github.com/VoltAgent/awesome-claude-code-subagents)
(161 agents, 10 categories). beacon-go only needs the SDD subset; the rest stay
deferred until a signal wakes them.

## Why

A one-sentence intent cannot be frozen as truth without three things:
intent questions, a state model when the work is stateful, and tests that cover
every acceptance criterion. The kernel ships the role plan that produces those,
not a chat UI.

## Command

```bash
beacon truth roles --utterance "订单审批流程，带支付和发布" --json
```

Output: `signals`, ordered `stages`, `deferred` specialists with their trigger,
and `coverage` (intent/fsm/tests/tasks/ready).

## Role map (awesome category -> SDD stage)

| SDD stage | Role | Source category | Wakes on |
|---|---|---|---|
| intake | `business-analyst` | product | always |
| spec | `product-manager` | product | always |
| spec | `domain-fsm-architect` | architecture | workflow/state/approval/order |
| spec | `microservices-architect` | architecture | api/service/gateway |
| spec | `ui-designer` | design | page/ui/app/screen |
| spec | `database-administrator` | data | schema/migration/table |
| spec | `security-engineer` | security | auth/payment/secret |
| spec | `devops-engineer` | infrastructure | deploy/release/ci/container |
| spec | `integration-specialist` | architecture | multi-repo/third-party/webhook |
| test | `test-automator` | quality | always |
| plan | `task-distributor` | meta-orchestration | always |
| review | `architect-reviewer` | quality | always |

Deferred roles are not started and not hidden: each carries the trigger that
would wake it.

## Rules

1. No model call in selection; deterministic keyword rules, testable.
2. A workflow signal without `domain-fsm-architect` fails coverage.
3. A specialist never runs just because it exists in the catalog.
4. Actual spawning stays with the external harness; the plan is the contract.

## Relation

- Issue: justin-mc-lai/beacon-go#11 (SDD drift and truth template reduction).
- FSM kernel work: justin-mc-lai/beacon-go#9.
