package cli

import (
	"testing"

	"beacon-go/internal/core"
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
	res1 := evaluateFSMTransition(truthMD, "Order", "PENDING", "PAID", "", []string{"payment_confirmed"})
	if !res1.Allowed || res1.Verdict != "allowed" {
		t.Fatalf("expected allowed transition, got %+v", res1)
	}
	if res1.Guard != "payment_confirmed" {
		t.Fatalf("expected guard 'payment_confirmed', got '%s'", res1.Guard)
	}

	// 2. Explicit illegal transition
	res2 := evaluateFSMTransition(truthMD, "Order", "PENDING", "SHIPPED", "", nil)
	if res2.Allowed || res2.Verdict != "illegal_transition" {
		t.Fatalf("expected illegal_transition verdict, got %+v", res2)
	}

	// 3. Undefined transition
	res3 := evaluateFSMTransition(truthMD, "Order", "PAID", "PENDING", "", nil)
	if res3.Allowed || res3.Verdict != "transition_not_found" {
		t.Fatalf("expected transition_not_found verdict, got %+v", res3)
	}
}

func TestFSMRuntimeNormalizedGuardsAndEvents(t *testing.T) {
	truth := "## FSM\n| From | To | Trigger | Guard | Side effect |\n|---|---|---|---|---|\n| draft | approved | approve | review.complete == true | publish |\n## Illegal transitions\n- illegal: approved -> draft\n"
	for _, tc := range []struct {
		name, event string
		guards      []string
		verdict     string
	}{
		{"guard denied", "", nil, "guard_failed"},
		{"guard exact", "approve", []string{"another guard", "review.complete == true"}, "allowed"},
		{"guard not evaluated", "approve", []string{"true"}, "guard_failed"},
		{"guard case sensitive", "approve", []string{"Review.complete == true"}, "guard_failed"},
		{"wrong event", "reject", []string{"review.complete == true"}, "transition_not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := evaluateFSMTransition(truth, "", "draft", "approved", tc.event, tc.guards)
			if res.Verdict != tc.verdict || res.Allowed != (tc.verdict == "allowed") || res.GuardExplanation == "" {
				t.Fatalf("result: %+v", res)
			}
		})
	}
	if res := evaluateFSMTransition(truth, "", "approved", "draft", "", nil); res.Verdict != "illegal_transition" {
		t.Fatalf("direction: %+v", res)
	}
}

func TestFSMRuntimeRefusals(t *testing.T) {
	for _, tc := range []struct{ name, truth, verdict string }{
		{"missing", "# Truth", "model_missing"},
		{"empty", "## FSM\n| State | Event | Guard | To | Action |\n|---|---|---|---|---|\n", "model_missing"},
		{"contradiction", "## FSM\n| From | To | Trigger | Guard |\n|---|---|---|---|\n| a | b | go | none |\n## Illegal transitions\n- illegal: a -> b\n", "illegal_transition"},
		{"invalid model", "## FSM\n| From | To | Trigger | Guard |\n|---|---|---|---|\n| a | b | go | none |\n", "invalid_model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if res := evaluateFSMTransition(tc.truth, "", "a", "b", "", nil); res.Allowed || res.Verdict != tc.verdict {
				t.Fatalf("result: %+v", res)
			}
		})
	}
}

func TestFSMRuntimeAmbiguousEvents(t *testing.T) {
	truth := "## FSM\n| State | Event | Guard | To | Action |\n|---|---|---|---|---|\n| a | one | true | b | run |\n| a | two | none | b | run |\n## Illegal transitions\n- illegal: b -> a\n"
	if res := evaluateFSMTransition(truth, "", "a", "b", "", nil); res.Verdict != "ambiguous_transition" || res.Allowed {
		t.Fatalf("ambiguous: %+v", res)
	}
	for _, event := range []string{"one", "two"} {
		if res := evaluateFSMTransition(truth, "", "a", "b", event, nil); !res.Allowed {
			t.Fatalf("event %s: %+v", event, res)
		}
	}
}

func TestFSMTransitionGuardFlagIsRepeatable(t *testing.T) {
	cmd := newFSMTransitionCmd()
	if err := cmd.ParseFlags([]string{"--event", "go", "--guard", "a,b", "--guard", "c"}); err != nil {
		t.Fatal(err)
	}
	guards, err := cmd.Flags().GetStringArray("guard")
	if err != nil || len(guards) != 2 || guards[0] != "a,b" || guards[1] != "c" {
		t.Fatalf("exact repeatable guards=%v err=%v", guards, err)
	}
}

func TestFSMTransitionRejectedResultReturnsTypedError(t *testing.T) {
	err := transitionResultError(TransitionResult{Verdict: "guard_failed", Reason: "guard not attested"})
	if code, ok := core.ReasonCodeOf(err); !ok || code != "fsm_guard_failed" {
		t.Fatalf("expected typed guard refusal, got %v", err)
	}
	if err := transitionResultError(TransitionResult{Allowed: true}); err != nil {
		t.Fatalf("allowed transition: %v", err)
	}
}

func TestFSMRuntimeUnconditionalGuards(t *testing.T) {
	for _, guard := range []string{"", "none", "true", "NONE", "TRUE"} {
		truth := "## FSM\n| From | To | Trigger | Guard |\n|---|---|---|---|\n| a | b | go | " + guard + " |\n## Illegal transitions\n- illegal: b -> a\n"
		if res := evaluateFSMTransition(truth, "", "a", "b", "", nil); !res.Allowed {
			t.Fatalf("unconditional guard %q rejected: %+v", guard, res)
		}
	}
}
