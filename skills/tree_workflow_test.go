package skills

import (
	"strings"
	"testing"
)

// Text contracts only: these tests prove neither CLI behavior nor agent compliance.
func TestTreeWorkflowDeliverables(t *testing.T) {
	cases := map[string][]string{
		"beacon-go-plan/SKILL.md":                          {"## Tree delivery contract", "无需已有树或 HTML", "候选树可选", "module_count", "coverage_manifest", "examples/module-tree-workflow.md"},
		"beacon-go-plan/references/core.md":                {"调查、头脑风暴、分析", "plan apply", "工具命名", "truth"},
		"beacon-go-truth/SKILL.md":                         {"本轮范围", "唯一 canonical 树", "HTML 必交", "html_path", "source_hash", "truth node freeze", "change"},
		"beacon-go-truth/references/core.md":               {"不可变 baseline", "本轮 delta", "future draft", "不拖本轮发布", "SQLite", "canonical JSON"},
		"beacon-go-change/SKILL.md":                        {"后续所有节点变更", "change 入口", "draft", "change request", "change legacy"},
		"beacon-go-change/references/core.md":              {"持续成长", "baseline", "delta", "future draft", "truth node update", "truth node freeze"},
		"beacon-go-goal/SKILL.md":                          {"run_id", "current_stage", "existing_artifacts", "next_action", "owner", "next/missing", "用户只决定目标、取舍、批准", "重复绑定", "内核不调 LLM、不跑 agent"},
		"beacon-go-goal/references/core.md":                {"plan -> truth -> implement -> verify -> release", "next/missing", "HTML", "当前 run", "future draft"},
		"beacon-go-plan/examples/module-tree-workflow.md":  {"intent -> module -> feature -> touchpoint", "worked sample", "来源", "11", "28", "候选树可选", "truth", "重复"},
		"beacon-go-truth/examples/freeze-readiness.md":     {"beacon plan apply", "beacon truth tree", "--format html", "beacon truth node freeze", "拒绝", "source_hash"},
		"beacon-go-change/examples/change-and-refreeze.md": {"beacon change request", "beacon truth node update", "beacon truth node freeze", "--version", "拒绝", "future draft"},
		"beacon-go-goal/examples/full-run.md":              {"beacon goal workflow check --run-id", "beacon goal tick --run-id", "--by", "owner", "next/missing", "拒绝"},
	}
	for path, required := range cases {
		t.Run(path, func(t *testing.T) {
			raw, err := files.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range required {
				if !strings.Contains(string(raw), want) {
					t.Errorf("missing required contract %q", want)
				}
			}
		})
	}
}

func TestTreeWorkflowRejectsHistoricalRouting(t *testing.T) {
	for _, id := range []string{"plan", "truth", "change", "goal"} {
		paths := packageFiles(id)
		for _, path := range paths {
			if !strings.HasSuffix(path, ".md") {
				continue
			}
			raw, err := files.ReadFile(packageDir(id) + "/" + path)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"beacon truth change", "truth -> design", "implement -> qa -> release", "Lake vs sea", "A sea is split", "boil lakes", "HTML output are mandatory"} {
				if strings.Contains(string(raw), forbidden) {
					t.Errorf("%s contains obsolete routing %q", path, forbidden)
				}
			}
		}
	}
}
