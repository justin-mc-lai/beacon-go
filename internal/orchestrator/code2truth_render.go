package orchestrator

import (
	"fmt"
	"strings"
)

func renderCode2TruthSection(target string, inv Code2TruthInventory, claims []Code2TruthClaim, ambiguity []Code2TruthAmbiguity) string {
	var b strings.Builder
	b.WriteString("**反推草稿待人确认**：本节由 `beacon truth code2truth` 从存量代码静态提取生成，权威级别 candidate_only（只是候选，不是正式真相）。\n")
	b.WriteString("tree_mapping: `" + code2TruthTreeMapping + "`（claim 到 intent/module/feature 的推理与 `plan expand/apply` proposal 由宿主完成，内核只产出可核对事实）。\n")
	b.WriteString("正式真相仍以人确认后的本文为准；冻结照常走意图门（签字、问题清零、覆盖矩阵），反推内容不豁免任何一道门。\n\n")
	fmt.Fprintf(&b, "- target: `%s`\n", target)
	fmt.Fprintf(&b, "- files_scanned: `%d`（source %d / test %d / doc %d / skill %d）\n",
		inv.FileCount, inv.Counts["source"], inv.Counts["test"], inv.Counts["doc"], inv.Counts["skill"])
	fmt.Fprintf(&b, "- exported_symbols: `%d`，routes: `%d`，models: `%d`\n",
		inv.Symbols.ExportedSymbols, len(inv.Symbols.Routes), len(inv.Symbols.Models))
	b.WriteString("\n| Claim | Confidence | Source Refs |\n|-------|------------|-------------|\n")
	for _, c := range claims {
		fmt.Fprintf(&b, "| %s %s | %s | %s |\n", c.ID, cellEscape(c.Claim), c.Confidence,
			cellEscape(strings.Join(capStrings(c.SourceRefs, 5), ", ")))
	}
	if len(ambiguity) > 0 {
		b.WriteString("\n歧义（人确认时必须处理）：\n\n")
		for _, a := range ambiguity {
			fmt.Fprintf(&b, "- `%s` %s route=`%s`\n", a.ID, a.Reason, a.Route)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderCode2TruthMarkdown(slug, target string, inv Code2TruthInventory, claims []Code2TruthClaim, ambiguity []Code2TruthAmbiguity, providerAvailable bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Code2Truth Intake: %s\n\n", slug)
	fmt.Fprintf(&b, "- target_project: `%s`\n", target)
	b.WriteString("- authority: `support_evidence_only`\n")
	b.WriteString("- tree_mapping: `" + code2TruthTreeMapping + "` (claims -> intent/module/feature is host reasoning; kernel emits evidence)\n\n")
	b.WriteString("## Boundary\n\nCode and provider graphs can propose candidate truth. User-confirmed Beacon package Markdown is formal truth.\n\n")
	b.WriteString("## Source Inventory\n\n")
	fmt.Fprintf(&b, "- files_scanned: `%d`\n- source_files: `%d`\n- skill_files: `%d`\n- test_files: `%d`\n- doc_files: `%d`\n\n",
		inv.FileCount, inv.Counts["source"], inv.Counts["skill"], inv.Counts["test"], inv.Counts["doc"])
	b.WriteString("### Entry Points\n\n")
	if len(inv.EntryPoints) == 0 {
		b.WriteString("- none detected\n")
	}
	for _, ref := range capStrings(inv.EntryPoints, 20) {
		fmt.Fprintf(&b, "- `%s`\n", ref)
	}
	b.WriteString("\n## Provider Evidence\n\n")
	fmt.Fprintf(&b, "- understand_anything_available: `%v`\n- provider_authority: `support_advisory`\n- truth_authority: `false`\n\n", providerAvailable)
	b.WriteString("## Candidate Truth\n\n")
	if len(claims) == 0 {
		b.WriteString("- none\n")
	}
	for _, c := range claims {
		fmt.Fprintf(&b, "- `%s` confidence=`%s` authority=`candidate_only`: %s\n", c.ID, c.Confidence, c.Claim)
		if len(c.SourceRefs) > 0 {
			fmt.Fprintf(&b, "  - refs: %s\n", strings.Join(capStrings(c.SourceRefs, 5), ", "))
		}
	}
	b.WriteString("\n## Ambiguity\n\n")
	if len(ambiguity) == 0 {
		b.WriteString("- none\n")
	}
	for _, a := range ambiguity {
		fmt.Fprintf(&b, "- `%s` %s route=`%s`\n", a.ID, a.Reason, a.Route)
	}
	b.WriteString("\n## Promotion Route\n\nCandidate truth can be promoted only through user confirmation and Beacon truth/change/refreeze routing.\n")
	return b.String()
}
