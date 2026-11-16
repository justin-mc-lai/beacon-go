package cli

import (
	"testing"
)

func TestEvaluateFSMTransition(t *testing.T) {
	truthMD := `---
status: frozen
domain_kind: general
---

## 实体前置序
| 实体 | 业务标识 Key | 前置序 Precedence | 强依赖 Requires |
|---|---|---|---|
| Order | order_id | 10 | Customer |

## 业务状态机 - Order
| State | From | Guard |
|---|---|---|
| PENDING | - | - |
| PAID | PENDING | payment_confirmed |
| SHIPPED | PAID | items_packed |

## 非法状态转移
| Description | Aggregate | Source |
|---|---|---|
| PENDING -> SHIPPED (cannot directly ship without pay) | Order | manual |
`

	// 1. Allowed transition
	res1 := evaluateFSMTransition(truthMD, "Order", "PENDING", "PAID")
	if !res1.Allowed || res1.Verdict != "allowed" {
		t.Fatalf("expected allowed transition, got %+v", res1)
	}
	if res1.Guard != "payment_confirmed" {
		t.Fatalf("expected guard 'payment_confirmed', got '%s'", res1.Guard)
	}

	// 2. Explicit illegal transition
	res2 := evaluateFSMTransition(truthMD, "Order", "PENDING", "SHIPPED")
	if res2.Allowed || res2.Verdict != "illegal_transition" {
		t.Fatalf("expected illegal_transition verdict, got %+v", res2)
	}

	// 3. Undefined transition
	res3 := evaluateFSMTransition(truthMD, "Order", "PAID", "PENDING")
	if res3.Allowed || res3.Verdict != "transition_not_found" {
		t.Fatalf("expected transition_not_found verdict, got %+v", res3)
	}
}
