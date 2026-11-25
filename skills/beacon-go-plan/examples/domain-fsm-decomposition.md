# Route: Domain FSM Decomposition (pln-fsm)

A user brings an operational or lifecycle workflow requirement (e.g. errand platform, dispatch, approval, payment, order fulfillment). The planner MUST NOT leave the states implicit or rely on generic happy-path text. It must extract entities, aggregate transitions, and illegal transitions before truth freeze.

## The Ask

> "设计一个跑腿平台状态机，要求有抢单、取件、送达，并且防止作弊刷单。"

## The Chain

1. **Entity Precedence Scan**:
   Identify core nouns and dependency hierarchy:
   - `User` (10)
   - `Rider` (10)
   - `DeliveryOrder` (20, requires User)
   - `DispatchTask` (30, requires DeliveryOrder, Rider)
   - `FulfillmentProof` (40, requires DispatchTask)
   - `Settlement` (50, requires DeliveryOrder, FulfillmentProof)

2. **Aggregate State Transition Matrix**:
   Build the 3-column table for the aggregate root `DeliveryOrder`:
   - `State | From | Guard`
   - Explicitly model exceptions (cancel, arbitration, refund).

3. **Illegal Transition Slots (Anti-cheat & Security)**:
   Explicitly declare what must NEVER happen:
   - `WAITING_DISPATCH -> PICKED_UP (cannot pickup before rider assigned)`
   - `RIDER_ASSIGNED -> DELIVERED (cannot deliver without pickup)`
   - `CREATED -> RIDER_ASSIGNED (cannot assign unpaid order)`
   - `PICKED_UP -> CANCELED (cannot cancel directly after cargo picked)`
   - `COMPLETED -> CANCELING (terminal state cannot cancel)`

4. **Static Verification**:
   Run `beacon truth fsm check --feature <slug>` to verify zero orphan states, valid entity precedence, and aggregate illegal slot coverage.
