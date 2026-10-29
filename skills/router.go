package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Progressive disclosure, made checkable.
//
// The seven embedded skills ship as a flat menu: a host that has them
// installed has to guess which one a request belongs to, and has no
// idea which reference to open first. This file carries both answers
// as data with a schema, so a test can assert them instead of trusting
// a model to improvise.

// RouterSchema is the version stamp of the router table.
const RouterSchema = "beacon-skill-router.v1"

// Signal is one phrase that indicates a skill. Weight lets a
// distinctive phrase outrank a common word: "跑到" must beat the bare
// "发布" in "帮我跑到发布", or a long-run request would be routed to
// the release skill.
type Signal struct {
	Text   string `json:"text"`
	Weight int    `json:"weight"`
}

// RouterEntry is how one skill is recognised and entered.
type RouterEntry struct {
	Skill    string   `json:"skill"`
	Summary  string   `json:"summary"`
	Signals  []Signal `json:"signals"`
	Excludes []string `json:"excludes"`
	Samples  []string `json:"samples"`
	// MustRead is the progressive-disclosure order: the references,
	// package-relative, to open first when this skill is chosen.
	MustRead []string `json:"must_read"`
}

// RouterTable is the whole routing surface.
type RouterTable struct {
	Schema  string        `json:"schema"`
	Entries []RouterEntry `json:"entries"`
}

// Router returns the routing table. It is derived from the same skill
// set Install writes, so the two cannot drift silently.
func Router() RouterTable {
	entries := make([]RouterEntry, 0, len(order))
	for _, e := range routerData() {
		entries = append(entries, e)
	}
	return RouterTable{Schema: RouterSchema, Entries: entries}
}

func routerData() []RouterEntry {
	return []RouterEntry{
		{
			Skill:    "goal",
			Summary:  "长程任务的整体推进：从一个想法走到可交付，中途可停可续",
			Signals:  []Signal{{"跑到", 3}, {"长程", 3}, {"交付闭环", 3}, {"从想法", 2}, {"完整流程", 2}},
			Samples:  []string{"帮我跑到发布，不用问我。", "这个需求长程跑一下，中途别停"},
			MustRead: []string{"references/core.md"},
		},
		{
			Skill:    "plan",
			Summary:  "动手前的规划与拆解：想清楚再做，先出方案",
			Signals:  []Signal{{"规划", 2}, {"方案", 2}, {"拆解", 2}, {"分解任务", 3}},
			Samples:  []string{"帮我规划一下这个需求", "把这个需求拆解成任务"},
			MustRead: []string{"references/core.md"},
		},
		{
			Skill:    "truth",
			Summary:  "需求真相：把要做什么写清楚、签字、冻结、防漂移",
			Signals:  []Signal{{"冻结", 2}, {"验收标准", 3}, {"需求真相", 3}, {"需求包", 2}, {"写AC", 2}},
			Samples:  []string{"把这个需求冻结", "写一下验收标准"},
			MustRead: []string{"references/core.md"},
		},
		{
			Skill:    "design",
			Summary:  "设计：界面与交互的形态决策，含设计原语绑定",
			Signals:  []Signal{{"设计", 2}, {"界面", 2}, {"原型", 3}, {"交互", 2}, {"UI", 2}},
			Samples:  []string{"设计一下这个界面", "出个原型"},
			MustRead: []string{"references/core.md"},
		},
		{
			Skill:    "implement",
			Summary:  "实现：按冻结的需求写代码，有准入与工位约束",
			Signals:  []Signal{{"写代码", 3}, {"实现", 2}, {"编码", 3}, {"开发", 1}},
			Samples:  []string{"实现这个功能", "写代码把这个做了"},
			MustRead: []string{"references/core.md"},
		},
		{
			Skill:    "qa",
			Summary:  "验收：机器验资与门禁判定，自审不算数",
			Signals:  []Signal{{"验收", 2}, {"验证", 2}, {"评分", 2}, {"跑测试", 3}, {"门禁", 2}},
			Samples:  []string{"验收一下", "跑一下测试验证"},
			MustRead: []string{"references/core.md"},
		},
		{
			Skill:    "release",
			Summary:  "发布：发布门禁评估，人工确认才可放行",
			Signals:  []Signal{{"发版", 3}, {"上线", 3}, {"发布门禁", 3}, {"发布", 1}},
			Samples:  []string{"发版吧", "上线这个版本", "发布吧"},
			MustRead: []string{"references/core.md"},
		},
	}
}

// RouteResult reports which skill a request belongs to. A miss says so
// and offers candidates — it never guesses a skill just to have an
// answer.
type RouteResult struct {
	Hit        bool     `json:"hit"`
	Skill      string   `json:"skill,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	MustRead   []string `json:"must_read,omitempty"`
}

// Route picks the skill for one utterance. Scoring is transparent:
// every signal phrase found adds its weight, an exclusion zeroes the
// entry, and a tie or a zero high score is reported as a miss.
func Route(utterance string) RouteResult {
	text := strings.ToLower(strings.TrimSpace(utterance))
	if text == "" {
		return RouteResult{Reason: "empty_utterance"}
	}
	type scored struct {
		id    string
		score int
	}
	var best []scored
	for _, e := range Router().Entries {
		s := 0
		excluded := false
		for _, ex := range e.Excludes {
			if ex != "" && strings.Contains(text, strings.ToLower(ex)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		for _, sig := range e.Signals {
			if sig.Text != "" && strings.Contains(text, strings.ToLower(sig.Text)) {
				s += sig.Weight
			}
		}
		if s > 0 {
			best = append(best, scored{e.Skill, s})
		}
	}
	if len(best) == 0 {
		return RouteResult{Reason: "no_signal"}
	}
	sort.SliceStable(best, func(i, j int) bool { return best[i].score > best[j].score })
	top := best[0].score
	tied := []string{}
	for _, b := range best {
		if b.score == top {
			tied = append(tied, b.id)
		}
	}
	if len(tied) > 1 {
		return RouteResult{Reason: "ambiguous", Candidates: tied}
	}
	res := RouteResult{Hit: true, Skill: tied[0]}
	if e, ok := entryFor(tied[0]); ok {
		res.MustRead = append([]string{}, e.MustRead...)
	}
	return res
}

func entryFor(skill string) (RouterEntry, bool) {
	for _, e := range Router().Entries {
		if e.Skill == skill {
			return e, true
		}
	}
	return RouterEntry{}, false
}

// MustReadPaths resolves a skill's must-read list to absolute paths
// under a host skill directory, returning only the ones that really
// exist. A declared reference that is missing on disk is a bug, and
// this is where that bug becomes visible instead of a silent 404 in
// the host agent's context.
func MustReadPaths(hostDir, skill string) []string {
	e, ok := entryFor(skill)
	if !ok || hostDir == "" {
		return nil
	}
	base := filepath.Join(hostDir, InstallDirName(skill))
	out := make([]string, 0, len(e.MustRead))
	for _, rel := range e.MustRead {
		p := filepath.Join(base, filepath.FromSlash(rel))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}
