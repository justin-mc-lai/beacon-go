// Package sddroles turns one utterance into an on-demand subagent plan for
// spec-driven development. Selection is deterministic: signals are matched by
// keyword rules, roles are ordered by stage, and specialists that are not
// signalled stay deferred with the trigger that would wake them.
//
// The catalog borrows the taxonomy of VoltAgent/awesome-claude-code-subagents
// (core development, language, infrastructure, QA, meta-orchestration) but
// keeps only the roles an SDD truth package needs.
package sddroles

import (
	"sort"
	"strings"
)

// Signal is a requirement trait that decides which specialists wake.
type Signal string

const (
	SignalWorkflow  Signal = "workflow"
	SignalAPI       Signal = "api"
	SignalUI        Signal = "ui"
	SignalData      Signal = "data"
	SignalSecurity  Signal = "security"
	SignalDeploy    Signal = "deploy"
	SignalMultiRepo Signal = "multi_repo"
	SignalExternal  Signal = "external"
)

// Role is one subagent definition: what it consumes, what it produces, and
// when it is allowed to start.
type Role struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Stage    string `json:"stage"`
	Inputs   string `json:"inputs"`
	Outputs  string `json:"outputs"`
	Always   bool   `json:"always"`
}

// Catalog is the fixed role set. Order is stage order, then priority.
var Catalog = []Role{
	{ID: "business-analyst", Title: "Intent interviewer", Category: "product", Stage: "intake", Inputs: "user utterance", Outputs: "open questions, constraints, non-goals", Always: true},
	{ID: "product-manager", Title: "Spec author", Category: "product", Stage: "spec", Inputs: "answered questions", Outputs: "user stories, acceptance criteria, out-of-scope", Always: true},
	{ID: "domain-fsm-architect", Title: "FSM / lifecycle architect", Category: "architecture", Stage: "spec", Inputs: "user stories with state", Outputs: "entities, states, transitions, illegal slots"},
	{ID: "microservices-architect", Title: "System architect", Category: "architecture", Stage: "spec", Inputs: "user stories + repo context", Outputs: "services, contracts, data flow"},
	{ID: "ui-designer", Title: "Interaction designer", Category: "design", Stage: "spec", Inputs: "user stories with screens", Outputs: "flows, states, empty/error views"},
	{ID: "database-administrator", Title: "Data architect", Category: "data", Stage: "spec", Inputs: "entities + migrations", Outputs: "schema, indexes, migration order"},
	{ID: "security-engineer", Title: "Security reviewer", Category: "security", Stage: "spec", Inputs: "auth/payment/secret surfaces", Outputs: "threats, trust boundaries, controls"},
	{ID: "devops-engineer", Title: "Delivery engineer", Category: "infrastructure", Stage: "spec", Inputs: "runtime + release rules", Outputs: "pipeline, environments, rollback"},
	{ID: "integration-specialist", Title: "Integration engineer", Category: "architecture", Stage: "spec", Inputs: "external APIs / multi-repo edges", Outputs: "contracts, retries, idempotency"},
	{ID: "test-automator", Title: "Test designer", Category: "quality", Stage: "test", Inputs: "acceptance criteria + FSM", Outputs: "TCs per AC, negative cases, fixtures", Always: true},
	{ID: "task-distributor", Title: "Task DAG planner", Category: "meta", Stage: "plan", Inputs: "ACs, TCs, architecture", Outputs: "ordered tasks with dependencies", Always: true},
	{ID: "architect-reviewer", Title: "Independent reviewer", Category: "quality", Stage: "review", Inputs: "truth package + tasks", Outputs: "review verdict, blocking findings", Always: true},
}

// SelectedRole is one role in the plan, with why it woke.
type SelectedRole struct {
	Role
	Why string `json:"why"`
}

// DeferredRole is a specialist that stays off until its trigger appears.
type DeferredRole struct {
	ID      string `json:"id"`
	Trigger string `json:"trigger"`
}

// Stage groups selected roles in execution order.
type Stage struct {
	Name  string         `json:"name"`
	Roles []SelectedRole `json:"roles"`
}

// Coverage says whether the plan can produce a complete truth package.
type Coverage struct {
	Intent bool `json:"intent"`
	FSM    bool `json:"fsm"`
	Tests  bool `json:"tests"`
	Tasks  bool `json:"tasks"`
	Ready  bool `json:"ready"`
}

// Plan is the on-demand subagent plan for one utterance.
type Plan struct {
	Utterance string         `json:"utterance"`
	Signals   []Signal       `json:"signals"`
	Stages    []Stage        `json:"stages"`
	Deferred  []DeferredRole `json:"deferred"`
	Coverage  Coverage       `json:"coverage"`
}

type rule struct {
	signal  Signal
	roleID  string
	why     string
	trigger string
	words   []string
}

var rules = []rule{
	{SignalWorkflow, "domain-fsm-architect", "stateful flow detected", "workflow: 状态/审批/订单/生命周期/workflow/state/approve",
		[]string{"流程", "状态机", "状态流转", "审批", "订单", "工单", "生命周期", "发布", "workflow", "state machine", "approve", "order", "lifecycle", "payment", "支付"}},
	{SignalAPI, "microservices-architect", "multi-service or API surface detected", "api: 接口/服务/微服务/API/gRPC",
		[]string{"接口", "服务", "微服务", "api", "grpc", "bff", "网关", "gateway"}},
	{SignalUI, "ui-designer", "user-facing surface detected", "ui: 页面/交互/端/小程序/web",
		[]string{"页面", "交互", "前端", "小程序", "app", "web", "ui", "界面", "端"}},
	{SignalData, "database-administrator", "schema or migration detected", "data: 表/迁移/schema/索引",
		[]string{"表结构", "迁移", "数据库", "索引", "schema", "migration", "sql", "字段"}},
	{SignalSecurity, "security-engineer", "auth or payment surface detected", "security: 登录/权限/支付/密钥",
		[]string{"登录", "认证", "鉴权", "权限", "支付", "密钥", "auth", "permission", "secret"}},
	{SignalDeploy, "devops-engineer", "delivery or environment work detected", "deploy: 部署/发布/CI/容器",
		[]string{"部署", "发布", "流水线", "容器", "k8s", "docker", "ci", "cd", "环境"}},
	{SignalMultiRepo, "integration-specialist", "cross-repo or external edge detected", "integration: 三仓/多仓/三方/回调",
		[]string{"多仓", "三仓", "第三方", "回调", "webhook", "对接", "集成", "sdk"}},
	{SignalExternal, "integration-specialist", "external API dependency detected", "integration: 外部API/回调/webhook",
		[]string{"外部接口", "回调", "webhook", "第三方", "对接"}},
}

var stageOrder = []string{"intake", "spec", "test", "plan", "review"}

// Select builds the plan for one utterance.
func Select(utterance string) Plan {
	lower := strings.ToLower(utterance)
	signalled := map[Signal]bool{}
	why := map[string]string{}
	deferredByID := map[string]DeferredRole{}

	for _, r := range rules {
		if !hasAny(lower, r.words) {
			if _, exists := deferredByID[r.roleID]; !exists {
				deferredByID[r.roleID] = DeferredRole{ID: r.roleID, Trigger: r.trigger}
			}
			continue
		}
		signalled[r.signal] = true
		why[r.roleID] = r.why
		delete(deferredByID, r.roleID)
	}

	plan := Plan{Utterance: utterance}
	for sig := range signalled {
		plan.Signals = append(plan.Signals, sig)
	}
	sort.Slice(plan.Signals, func(i, j int) bool { return plan.Signals[i] < plan.Signals[j] })

	byStage := map[string][]SelectedRole{}
	for _, role := range Catalog {
		reason, on := why[role.ID]
		if !role.Always && !on {
			continue
		}
		if role.Always {
			reason = "always on for SDD"
		}
		byStage[role.Stage] = append(byStage[role.Stage], SelectedRole{Role: role, Why: reason})
	}
	for _, stage := range stageOrder {
		if roles := byStage[stage]; len(roles) > 0 {
			plan.Stages = append(plan.Stages, Stage{Name: stage, Roles: roles})
		}
	}

	for _, d := range deferredByID {
		plan.Deferred = append(plan.Deferred, d)
	}
	sort.Slice(plan.Deferred, func(i, j int) bool { return plan.Deferred[i].ID < plan.Deferred[j].ID })

	plan.Coverage = Coverage{
		Intent: hasRole(plan, "business-analyst") && hasRole(plan, "product-manager"),
		FSM:    !signalled[SignalWorkflow] || hasRole(plan, "domain-fsm-architect"),
		Tests:  hasRole(plan, "test-automator"),
		Tasks:  hasRole(plan, "task-distributor"),
	}
	plan.Coverage.Ready = plan.Coverage.Intent && plan.Coverage.FSM && plan.Coverage.Tests && plan.Coverage.Tasks
	return plan
}

func hasAny(haystack string, words []string) bool {
	for _, w := range words {
		if strings.Contains(haystack, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

func hasRole(p Plan, id string) bool {
	for _, s := range p.Stages {
		for _, r := range s.Roles {
			if r.ID == id {
				return true
			}
		}
	}
	return false
}
