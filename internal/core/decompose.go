// Intent decomposition (R2d, parity audit M3): the Go port of
// Python utils/intent_decomposition.py (decompose_product_intent)
// plus the sentence capture of utils/intent_first.py. Pure and
// deterministic: the same utterance always decomposes the same
// way — keyword/structure rules copied from the source, never a
// model call. The materialization into a package lives in
// orchestrator/decompose.go.
//
// NOT ported: the v1.6.11 task-closure advisory block at the end
// of decompose_product_intent (it depends on
// plan_goal_task_closure's inherited-must machinery, an advisory
// surface with no Go consumer yet). Noted in the R2d report.
package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DecompositionSchema is intent_decomposition.SCHEMA.
const DecompositionSchema = "intent_decomposition.v1"

// Token vocabularies (verbatim from intent_decomposition.py).
var (
	decompCapabilityTokens = []string{
		"认证", "登录", "注册", "auth", "支付", "billing", "订阅", "stripe",
		"多语言", "i18n", "插件", "plugin", "skill", "agent skill", "dashboard",
		"控制台", "部署", "deploy", "seo", "后台", "admin", "邮件", "email", "resend",
	}
	decompPlatformTokens = []string{
		"cloudflare", "d1", "postgres", "postgresql", "vercel",
		"better auth", "nextauth", "drizzle", "tanstack",
	}
	decompOceanTokens = []string{
		"拆海", "海级", "整包", "完整复刻", "parity", "底座", "foundation",
		"开源底座", "多模块", "多湖", "蒸干",
		"l0", "l1", "l2", "l3", "l4", "l5", "l6",
		"oss parity", "全功能",
	}
	decompSourceTokens = []string{
		"github.com", "对着代码", "源码", "对照", "参考", "get-saas", "别瞎编", "仓库",
	}
)

// JourneyHint is one _JOURNEY_HINTS row: id, display name, trigger
// tokens, acceptance seeds.
type JourneyHint struct {
	ID             string
	Name           string
	Tokens         []string
	AcceptanceSeed []string
}

// DecompJourneyHints is _JOURNEY_HINTS, verbatim (order matters:
// journeys are emitted in table order).
var DecompJourneyHints = []JourneyHint{
	{"J-deploy", "冷启动与部署",
		[]string{"部署", "deploy", "cloudflare", "pages", "d1", "wrangler"},
		[]string{"可按文档在目标平台拉起环境", "密钥与绑定清单完整"}},
	{"J-identity", "注册登录与会话",
		[]string{"认证", "登录", "注册", "auth", "better auth", "session"},
		[]string{"用户可注册/登录", "会话可校验", "用户数据在自有存储"}},
	{"J-billing", "订阅支付与权益",
		[]string{"支付", "billing", "订阅", "stripe", "权益", "entitlement", "webhook"},
		[]string{"可发起 checkout", "webhook 幂等开通权益", "无密钥泄漏到插件"}},
	{"J-shell", "产品壳与多语言",
		[]string{"dashboard", "控制台", "i18n", "多语言", "landing", "壳"},
		[]string{"双语言壳可访问", "登录后可见账户/权益状态"}},
	{"J-plugin", "插件化新产品",
		[]string{"插件", "plugin", "多产品", "plugin-v0", "扩展"},
		[]string{"manifest 可挂载路由/导航", "插件不直连支付密钥"}},
	{"J-agent", "Agent skill 操作面",
		[]string{"skill", "agent", "orient", "smoke", "子代理"},
		[]string{"orient/add-plugin/smoke 技能可用", "冒烟有证据路径"}},
	{"J-qa-release", "验收与发布门",
		[]string{"验收", "qa", "release", "发布", "smoke"},
		[]string{"旅程有测试或清单证据", "release 需人工门"}},
}

func decompHasAny(text string, tokens []string) []string {
	low := strings.ToLower(text)
	var hits []string
	for _, tok := range tokens {
		if strings.Contains(low, strings.ToLower(tok)) || strings.Contains(text, tok) {
			hits = append(hits, tok)
		}
	}
	return hits
}

var decompClauseSplitRe = regexp.MustCompile(`[。；;\n]|→|->`)

// DecompSplitClauses ports _split_clauses: split, trim the
// punctuation set, keep clauses of at least 4 characters.
func DecompSplitClauses(utterance string) []string {
	var out []string
	for _, p := range decompClauseSplitRe.Split(utterance, -1) {
		p = strings.Trim(p, " ：:，,")
		if len([]rune(p)) >= 4 {
			out = append(out, p)
		}
	}
	return out
}

var (
	decompLakeLRe  = regexp.MustCompile(`(?i)\bL([0-9]+)\s*[-:.]?\s*([A-Za-z0-9_-]+)?`)
	decompLakeCNRe = regexp.MustCompile(`湖\s*([0-9]+)`)
)

// DecompExtractLakes ports _extract_lakes.
func DecompExtractLakes(utterance string, lakes []string) []string {
	var found []string
	seen := map[string]bool{}
	add := func(label string) {
		if label != "" && !seen[label] {
			seen[label] = true
			found = append(found, label)
		}
	}
	for _, x := range lakes {
		add(strings.TrimSpace(x))
	}
	for _, m := range decompLakeLRe.FindAllStringSubmatch(utterance, -1) {
		slug := strings.Trim(m[2], "-_ ")
		label := "l" + m[1]
		if slug != "" {
			label += "-" + slug
		}
		add(strings.ToLower(label))
	}
	for _, m := range decompLakeCNRe.FindAllStringSubmatch(utterance, -1) {
		add("lake-" + m[1])
	}
	return found
}

// DecompIntent is one decomposed sub-intent (_build_intents row).
type DecompIntent struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Kinds     []string `json:"kinds"`
	SourceRef string   `json:"source_ref"`
}

// DecompJourney is one journey seed (_build_journeys row).
type DecompJourney struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	MatchedTokens   []string `json:"matched_tokens"`
	Actors          []string `json:"actors"`
	StepsSeed       []string `json:"steps_seed"`
	AcceptanceSeeds []string `json:"acceptance_seeds"`
	DetailLevel     string   `json:"detail_level"`
	RequiresFreeze  bool     `json:"requires_freeze"`
	ForcedBy        string   `json:"forced_by,omitempty"`
}

// TruthReadiness is the decomposition's readiness verdict.
type TruthReadiness struct {
	Ready                 bool     `json:"ready"`
	Missing               []string `json:"missing"`
	BlocksAutoTick        bool     `json:"blocks_auto_tick_past_plan"`
	RequiresJourneyFreeze bool     `json:"requires_journey_freeze"`
	UserFacing            string   `json:"user_facing"`
}

// Decomposition is decompose_product_intent's result payload.
type Decomposition struct {
	Schema      string              `json:"schema"`
	GeneratedAt string              `json:"generated_at"`
	SurfaceAsk  string              `json:"surface_ask"`
	DeepWant    string              `json:"deep_want"`
	Intents     []DecompIntent      `json:"intents"`
	Journeys    []DecompJourney     `json:"journeys"`
	ScopeMode   string              `json:"scope_mode"` // lake | ocean (decomposition axis)
	Lakes       []string            `json:"lakes"`
	OceanTokens []string            `json:"ocean_tokens,omitempty"`
	Methodology map[string]any      `json:"methodology"`
	Signals     map[string][]string `json:"signals"`
	HighRisk    []string            `json:"high_risk_domains"`
	Readiness   TruthReadiness      `json:"truth_readiness"`
	Recommended []string            `json:"recommended_truth_artifacts"`
	Status      string              `json:"status"`
}

func runeTrunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func decompBuildIntents(utterance string) []DecompIntent {
	clauses := DecompSplitClauses(utterance)
	if len(clauses) == 0 && strings.TrimSpace(utterance) != "" {
		clauses = []string{strings.TrimSpace(utterance)}
	}
	var intents []DecompIntent
	for i, clause := range clauses {
		var kinds []string
		if len(decompHasAny(clause, decompPlatformTokens)) > 0 {
			kinds = append(kinds, "platform")
		}
		if len(decompHasAny(clause, decompCapabilityTokens)) > 0 {
			kinds = append(kinds, "capability")
		}
		if len(decompHasAny(clause, decompOceanTokens)) > 0 {
			kinds = append(kinds, "scope")
		}
		if len(decompHasAny(clause, decompSourceTokens)) > 0 {
			kinds = append(kinds, "source")
		}
		if len(kinds) == 0 {
			kinds = []string{"delivery"}
		}
		intents = append(intents, DecompIntent{
			ID:        fmt.Sprintf("I%02d", i+1),
			Text:      runeTrunc(clause, 300),
			Kinds:     kinds,
			SourceRef: fmt.Sprintf("utterance_clause_%d", i+1),
		})
	}
	return intents
}

func journeySteps(name string) []string {
	return []string{
		"前置：满足 " + name + " 的环境/账户条件",
		"主路径：完成 " + name,
		"失败路径：记录可恢复错误与权限拒绝",
		"证据：测试、清单或日志路径可引用",
	}
}

func lakeForcesJourney(lakes []string, jid string) bool {
	for _, lake := range lakes {
		low := strings.ToLower(lake)
		switch jid {
		case "J-identity":
			if strings.Contains(low, "identity") {
				return true
			}
		case "J-billing":
			if strings.Contains(low, "billing") {
				return true
			}
		case "J-shell":
			if strings.Contains(low, "shell") {
				return true
			}
		case "J-plugin":
			if strings.Contains(low, "plugin") {
				return true
			}
		case "J-deploy":
			if strings.Contains(low, "deploy") {
				return true
			}
		case "J-qa-release":
			if strings.Contains(low, "smoke") || strings.Contains(low, "qa") {
				return true
			}
		case "J-agent":
			if strings.Contains(low, "skill") || strings.Contains(low, "smoke") {
				return true
			}
		}
	}
	return false
}

func decompBuildJourneys(utterance string, lakes []string) []DecompJourney {
	var journeys []DecompJourney
	for _, hint := range DecompJourneyHints {
		hits := decompHasAny(utterance, hint.Tokens)
		if len(hits) == 0 && !lakeForcesJourney(lakes, hint.ID) {
			continue
		}
		actors := []string{"end_user", "builder"}
		if strings.HasPrefix(hint.ID, "J-agent") {
			actors = []string{"end_user", "builder", "agent"}
		}
		journeys = append(journeys, DecompJourney{
			ID: hint.ID, Name: hint.Name, MatchedTokens: hits, Actors: actors,
			StepsSeed: journeySteps(hint.Name), AcceptanceSeeds: append([]string{}, hint.AcceptanceSeed...),
			DetailLevel: "seed", RequiresFreeze: true,
		})
	}
	// Ocean with few matches forces the core SaaS journeys.
	if len(journeys) < 3 && (len(decompHasAny(utterance, decompOceanTokens)) > 0 || len(lakes) >= 3) {
		forced := map[string]bool{"J-identity": true, "J-billing": true, "J-plugin": true, "J-agent": true}
		have := map[string]bool{}
		for _, j := range journeys {
			have[j.ID] = true
		}
		for _, hint := range DecompJourneyHints {
			if forced[hint.ID] && !have[hint.ID] {
				journeys = append(journeys, DecompJourney{
					ID: hint.ID, Name: hint.Name, MatchedTokens: []string{},
					Actors:          []string{"end_user", "builder"},
					StepsSeed:       journeySteps(hint.Name),
					AcceptanceSeeds: append([]string{}, hint.AcceptanceSeed...),
					DetailLevel:     "seed", RequiresFreeze: true, ForcedBy: "ocean_minimum",
				})
			}
		}
	}
	return journeys
}

// DecomposeIntent ports decompose_product_intent (minus the
// task-closure advisory; see the file header). generatedAt is
// injected so the pure function stays deterministic in tests.
func DecomposeIntent(utterance string, lakes []string, subintent, generatedAt string) Decomposition {
	text := strings.TrimSpace(utterance)
	lakeList := DecompExtractLakes(text, lakes)
	intents := decompBuildIntents(text)
	journeys := decompBuildJourneys(text, lakeList)
	oceanHits := decompHasAny(text, decompOceanTokens)
	sourceHits := decompHasAny(text, decompSourceTokens)
	capabilityHits := decompHasAny(text, decompCapabilityTokens)
	platformHits := decompHasAny(text, decompPlatformTokens)
	scopeMode := "lake"
	if len(oceanHits) > 0 || len(lakeList) >= 3 {
		scopeMode = "ocean"
	}

	var missing []string
	if text == "" {
		missing = append(missing, "empty_utterance")
	}
	if scopeMode == "ocean" && len(journeys) < 3 {
		missing = append(missing, "insufficient_journey_seeds")
	}
	if scopeMode == "ocean" {
		missing = append(missing, "journey_detail_not_frozen", "ac_matrix_not_materialized")
	}
	if len(sourceHits) > 0 {
		missing = append(missing, "source_anchors_pending")
	}
	highRiskText := false
	for _, k := range []string{"支付", "billing", "stripe", "auth", "认证", "权限"} {
		if strings.Contains(text, k) {
			highRiskText = true
		}
	}
	if (highRiskText || len(decompHasAny(text, []string{"支付", "stripe", "auth", "认证"})) > 0) && scopeMode == "ocean" {
		missing = append(missing, "domain_fsm_notes_missing")
	}

	ready := false
	if scopeMode == "lake" && len(intents) >= 1 && len(missing) == 0 {
		ready = true
	}
	if scopeMode == "lake" && len(oceanHits) == 0 && len(lakeList) <= 1 {
		drop := map[string]bool{
			"journey_detail_not_frozen": true, "ac_matrix_not_materialized": true,
			"domain_fsm_notes_missing": true,
		}
		var kept []string
		for _, m := range missing {
			if !drop[m] {
				kept = append(kept, m)
			}
		}
		missing = kept
		ready = len(intents) >= 1 && len(missing) == 0
	}
	blocksAuto := scopeMode == "ocean" || len(missing) > 0

	var bits []string
	if len(platformHits) > 0 {
		bits = append(bits, "平台约束："+strings.Join(head(platformHits, 6), ", "))
	}
	if len(capabilityHits) > 0 {
		bits = append(bits, "能力面："+strings.Join(head(capabilityHits, 8), ", "))
	}
	if len(oceanHits) > 0 {
		bits = append(bits, "范围：海级/多湖，需拆旅程后 freeze")
	}
	if len(sourceHits) > 0 {
		bits = append(bits, "源码锚定："+strings.Join(head(sourceHits, 5), ", "))
	}
	deepWant := "推进可验证交付；需补齐成功标准与失败边界。"
	if len(bits) > 0 {
		deepWant = strings.Join(bits, "；")
	}
	if text != "" {
		deepWant += "｜表层：" + runeTrunc(text, 240)
	}

	var secondary []string
	if len(oceanHits) > 0 || len(lakeList) >= 2 {
		secondary = append(secondary, "program")
	}
	if len(sourceHits) > 0 {
		secondary = append(secondary, "source")
	}
	if len(decompHasAny(text, []string{"支付", "auth", "安全", "状态机", "parity", "完整复刻"})) > 0 {
		secondary = append(secondary, "review")
	}

	riskSet := map[string]bool{}
	if len(decompHasAny(text, []string{"支付", "stripe", "billing", "订阅"})) > 0 {
		riskSet["payment"] = true
	}
	if len(decompHasAny(text, []string{"认证", "auth", "登录", "权限"})) > 0 {
		riskSet["auth"] = true
	}
	if len(decompHasAny(text, []string{"插件", "plugin"})) > 0 {
		riskSet["plugin_isolation"] = true
	}
	var highRisk []string
	for k := range riskSet {
		highRisk = append(highRisk, k)
	}
	sort.Strings(highRisk)

	finalReady := ready
	if scopeMode == "ocean" {
		finalReady = ready && !blocksAuto
	}
	userFacing := "意图拆解可进入 truth 细化。"
	if blocksAuto {
		userFacing = "海级/多旅程诉求：已生成 journey seeds，但尚未冻结用户旅程与 AC 矩阵；禁止 auto_tick 跳过 truth。"
	}
	if missing == nil {
		missing = []string{}
	}
	if secondary == nil {
		secondary = []string{}
	}
	if highRisk == nil {
		highRisk = []string{}
	}
	return Decomposition{
		Schema:      DecompositionSchema,
		GeneratedAt: generatedAt,
		SurfaceAsk:  text,
		DeepWant:    deepWant,
		Intents:     intents,
		Journeys:    journeys,
		ScopeMode:   scopeMode,
		Lakes:       lakeList,
		OceanTokens: oceanHits,
		Methodology: map[string]any{
			"declared_subintent": subintent,
			"secondary_tracks":   secondary,
		},
		Signals: map[string][]string{
			"platform": platformHits, "capability": capabilityHits,
			"source": sourceHits, "ocean": oceanHits,
		},
		HighRisk: highRisk,
		Readiness: TruthReadiness{
			Ready:                 finalReady,
			Missing:               missing,
			BlocksAutoTick:        blocksAuto,
			RequiresJourneyFreeze: scopeMode == "ocean" || len(journeys) >= 2,
			UserFacing:            userFacing,
		},
		Recommended: []string{
			"docs/beacon/<ver>/user-story/*-journeys.md",
			"docs/beacon/<ver>/features/<lake>/truth.md (per lake)",
			"docs/beacon/<ver>/.machine/user-story/*.json",
			"domain FSM notes for payment/auth if high_risk",
		},
		Status: "ok",
	}
}

func head(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// MarkJourneysFrozen ports mark_journeys_frozen: after a real
// freeze pass, clear the corresponding missing items and
// recompute readiness.
func MarkJourneysFrozen(d Decomposition, frozenJourneyIDs []string, acMatrixPresent, domainFSMPresent bool, sourceAnchors []string) Decomposition {
	tr := d.Readiness
	var missing []string
	for _, m := range tr.Missing {
		if m != "empty_utterance" {
			missing = append(missing, m)
		}
	}
	fj := map[string]bool{}
	for _, id := range frozenJourneyIDs {
		fj[id] = true
	}
	if len(fj) > 0 {
		allFrozen := true
		for _, j := range d.Journeys {
			if j.RequiresFreeze && !fj[j.ID] {
				allFrozen = false
			}
		}
		if allFrozen {
			missing = removeMissing(missing, "journey_detail_not_frozen")
			for i := range d.Journeys {
				if fj[d.Journeys[i].ID] {
					d.Journeys[i].DetailLevel = "frozen"
				}
			}
		}
	}
	if acMatrixPresent {
		missing = removeMissing(missing, "ac_matrix_not_materialized")
	}
	if domainFSMPresent {
		missing = removeMissing(missing, "domain_fsm_notes_missing")
	}
	if len(sourceAnchors) > 0 {
		missing = removeMissing(missing, "source_anchors_pending")
	}
	if missing == nil {
		missing = []string{}
	}
	tr.Missing = missing
	tr.Ready = len(missing) == 0
	tr.BlocksAutoTick = !tr.Ready
	if tr.Ready {
		tr.UserFacing = "旅程/AC 已标记冻结，可进入 truth 物化。"
	} else {
		tr.UserFacing = "仍缺：" + strings.Join(missing, ", ")
	}
	d.Readiness = tr
	return d
}

func removeMissing(in []string, code string) []string {
	var out []string
	for _, m := range in {
		if m != code {
			out = append(out, m)
		}
	}
	return out
}

// ---- intent_first.py capture (M3's provenance source) ----

// IntentCapture is intent_first.IntentCapture.
type IntentCapture struct {
	Text       string `json:"text"`
	SourceType string `json:"source_type"` // user_intent
	SourceRef  string `json:"source_ref"`
}

var intentSentenceSplitRe = regexp.MustCompile(`[。\.\n;；]`)

// CaptureIntents ports capture_intents_from_utterance: split on
// sentence terminators, keep substantive sentences (>=10 chars),
// each becomes one user-intent capture with a per-sentence
// provenance ref.
func CaptureIntents(utterance string) []IntentCapture {
	var out []IntentCapture
	idx := 0
	for _, s := range intentSentenceSplitRe.Split(utterance, -1) {
		s = strings.TrimSpace(s)
		if len([]rune(s)) < 10 {
			continue
		}
		idx++
		out = append(out, IntentCapture{
			Text:       s,
			SourceType: "user_intent",
			SourceRef:  fmt.Sprintf("utterance_sentence_%d: %s", idx, runeTrunc(s, 120)),
		})
	}
	return out
}
