package sddroles

import "testing"

func roleIDs(p Plan) map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Stages {
		for _, r := range s.Roles {
			out[r.ID] = true
		}
	}
	return out
}

func TestWorkflowWakesFSM(t *testing.T) {
	p := Select("订单审批流程，状态机从待支付到已发布，需要接口和数据库表")
	ids := roleIDs(p)
	for _, want := range []string{"business-analyst", "product-manager", "domain-fsm-architect", "microservices-architect", "database-administrator", "test-automator", "task-distributor", "architect-reviewer"} {
		if !ids[want] {
			t.Fatalf("missing %s in %v", want, ids)
		}
	}
	if !p.Coverage.FSM || !p.Coverage.Ready {
		t.Fatalf("coverage = %+v", p.Coverage)
	}
}

func TestUIDoesNotWakeFSM(t *testing.T) {
	p := Select("做一个设置页面，带空状态和错误提示")
	ids := roleIDs(p)
	if !ids["ui-designer"] {
		t.Fatal("ui-designer must wake on 页面")
	}
	if ids["domain-fsm-architect"] {
		t.Fatal("ui-only work must not wake the FSM architect")
	}
	if p.Coverage.FSM != true {
		t.Fatal("FSM coverage is not required without a workflow signal")
	}
}

func TestDeferredSpecialistsStayOff(t *testing.T) {
	p := Select("写一个命令行小工具")
	ids := roleIDs(p)
	for _, off := range []string{"domain-fsm-architect", "ui-designer", "security-engineer", "devops-engineer"} {
		if ids[off] {
			t.Fatalf("%s woke without a signal", off)
		}
	}
	if len(p.Deferred) == 0 {
		t.Fatal("deferred list must explain what would wake a specialist")
	}
	for _, d := range p.Deferred {
		if d.Trigger == "" {
			t.Fatalf("deferred %s has no trigger", d.ID)
		}
	}
}

func TestStageOrderStable(t *testing.T) {
	p := Select("支付流程，带前端页面，需要部署")
	order := []string{}
	for _, s := range p.Stages {
		order = append(order, s.Name)
	}
	want := []string{"intake", "spec", "test", "plan", "review"}
	for i := range want {
		if i >= len(order) || order[i] != want[i] {
			t.Fatalf("stage order = %v", order)
		}
	}
}

func TestDeterministic(t *testing.T) {
	a := Select("多仓对接第三方回调，带登录和部署")
	b := Select("多仓对接第三方回调，带登录和部署")
	if len(a.Signals) != len(b.Signals) || len(a.Deferred) != len(b.Deferred) {
		t.Fatal("selection must be deterministic")
	}
	for i := range a.Signals {
		if a.Signals[i] != b.Signals[i] {
			t.Fatalf("signals differ: %v vs %v", a.Signals, b.Signals)
		}
	}
}
