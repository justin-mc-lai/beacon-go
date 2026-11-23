package orchestrator

import (
	"path/filepath"
	"regexp"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// SDDContract marks the two-file SDD package: truth.md is the Spec
// (including the AC/TC matrix) and tasks.md is the DAG. tests.md,
// evidence.md and restatement.md are optional.
const SDDContract = "truth-tasks"

func packageContract(truthText string) string {
	scalars, _, ok := gate.ParseFrontmatter(truthText)
	if !ok {
		return ""
	}
	return strings.TrimSpace(scalars["package_contract"])
}

var sddPlaceholders = []string{"（一句话说清", "待补齐", "见 truth.md", "TODO", "TBD", "写清谁在什么场景", "写清这个包", "写清本包", "写清必须", "主体流程在合法输入下到达完成状态"}

var sddTCID = regexp.MustCompile(`^TC-[0-9]{3,}$`)

// sddFreezeFindings checks applicability and cross references, not a fixed
// document size. Semantic alignment is attested by an independent review.
func sddFreezeFindings(pkg gate.PackageFiles) []gate.Finding {
	var out []gate.Finding
	add := func(code, detail string) {
		out = append(out, gate.Finding{Kind: code, ReasonCode: code, File: pkg.Truth.Path, Detail: detail, RepairRoute: gate.DefaultRepairRoute})
	}
	scalars, _, _ := gate.ParseFrontmatter(pkg.Truth.Text)
	if scalars["package_maturity"] == "scaffold" {
		add(core.ReasonSDDPlaceholder, "generated scaffold is not a completed requirement")
	}
	for _, phrase := range sddPlaceholders {
		if strings.Contains(pkg.Truth.Text, phrase) || strings.Contains(pkg.Tasks.Text, phrase) {
			add(core.ReasonSDDPlaceholder, "replace scaffold content: "+phrase)
			break
		}
	}
	if strings.EqualFold(scalars["traceability_required"], "false") {
		add("sdd_traceability_required", "SDD packages cannot disable evidence traceability")
	}
	for _, key := range []string{"fsm_required", "negative_required"} {
		if scalars[key] != "true" && scalars[key] != "false" {
			add("sdd_risk_policy_missing", "explicitly declare "+key+" based on the grounded requirement")
		}
	}
	defined := gate.DefinedACs(pkg.Truth.Text)
	if len(defined) == 0 {
		add(core.ReasonSDDACMin, "define at least one observable acceptance criterion")
	}
	rows, _, ok := gate.ParseCoverageMatrix(pkg.Truth.Text)
	if !ok {
		add(core.ReasonSDDMatrixMissing, "truth.md has no AC/TC coverage matrix")
		return out
	}
	seen := make(map[string]bool, len(rows))
	positive, negative := map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if !sddTCID.MatchString(row.TC) {
			add("sdd_tc_identity", "invalid test case identity: "+row.TC)
		}
		if seen[row.TC] {
			add("sdd_tc_duplicate", "repeated test case identity: "+row.TC)
		}
		seen[row.TC] = true
		command := strings.Trim(strings.TrimSpace(row.Command), "`")
		words, commandErr := SplitCommandWords(command)
		trivial := len(words) > 0 && (filepath.Base(words[0]) == "true" || words[0] == ":" || words[0] == "exit")
		if row.IsSoft() || gate.IsDocsOnlyCommand(command) || commandErr != nil || trivial || strings.Contains(command, "BLOCKED:") {
			add("sdd_tc_not_executable", row.TC+" must describe a real verification command and assertion")
		}
		if !defined[row.AC] {
			add("sdd_tc_unknown_ac", row.TC+" references undefined "+row.AC)
		}
		switch strings.ToLower(row.Layer) {
		case "behavior", "positive", "integration", "e2e", "unit":
			positive[row.AC] = true
		case "negative":
			negative[row.AC] = true
		default:
			add("sdd_tc_layer", row.TC+" requires a recognized test layer")
		}
	}
	for ac := range defined {
		if !positive[ac] {
			add(core.ReasonSDDTCPerAC, ac+" needs a positive observable behavior case")
		}
		if strings.EqualFold(scalars["negative_required"], "true") && !negative[ac] {
			add(core.ReasonSDDNegativeMin, ac+" requires a negative case under this package's risk policy")
		}
	}
	projection, graphFindings, _, _ := gate.CheckTruthFSM(pkg.Truth.Text)
	for _, finding := range graphFindings {
		if finding.Severity == "error" {
			add(core.ReasonSDDFSMRequired, finding.Text)
		}
	}
	if projection.FiveTuplePresent {
		out = append(out, sddTransitionCoverage(pkg.Truth.Text, projection, rows)...)
	}
	if pkg.Tasks.Present {
		if _, err := gate.ParseTaskDAG(pkg.Tasks.Text, defined); err != nil {
			add("sdd_task_dag_invalid", err.Error())
		}
	}
	return out
}
