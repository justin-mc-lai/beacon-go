// Scope mode (R2a, parity audit M6): Python never lets a human
// free-type scope_mode — it is a DERIVED value on two axes:
//
//   - planner axis (planner_review_runtime._scope_mode, line 324):
//     full_parity / mvp / patch / brownfield_takeover / research,
//     derived from prompt signals, in that priority order;
//   - decomposition axis (intent_decomposition.decompose_product_intent,
//     line 285): lake / ocean, ocean when an ocean signal hits or
//     the utterance spans >= 3 lakes.
//
// The Go intent snapshot carries one scope_mode field, so the
// controlled vocabulary is the union of both axes plus "focused" —
// the Go kernel's pre-R2a default for a single focused lake
// (retained as a value; Python's decomposition axis calls the same
// shape "lake"). "explore" (a former CLI help string, never a
// stored value) is dropped. An explicit value outside the
// vocabulary fails closed (core.ReasonScopeModeInvalid); an empty
// value is derived from the utterance.
package gate

import "strings"

// ScopeModeVocabulary is the controlled set for the intent
// snapshot's scope_mode field.
var ScopeModeVocabulary = []string{
	"full_parity", "mvp", "patch", "brownfield_takeover", "research", // planner axis
	"lake", "ocean", // decomposition axis
	"focused", // Go-retained single-lake default (see file comment)
}

// ValidScopeMode reports whether s names a controlled scope mode
// (case-insensitive; the stored form is lowercase).
func ValidScopeMode(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	for _, v := range ScopeModeVocabulary {
		if s == v {
			return true
		}
	}
	return false
}

// NormalizeScopeMode returns the canonical stored form.
func NormalizeScopeMode(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Signal lists copied from the Python sources:
// planner_review_runtime._scope_mode (lines 326-332).
var (
	scopeParitySignals = []string{"完整复刻", "同等能力", "same capability", "full parity", "做全", "parity", "不要 mvp"}
	scopeMVPSignals    = []string{"mvp", "最小", "minimal"}
	scopePatchSignals  = []string{"patch", "hotfix", "修复"}
	// intent_decomposition._OCEAN (lines 56-77).
	scopeOceanSignals = []string{
		"拆海", "海级", "整包", "完整复刻", "parity", "底座", "foundation",
		"开源底座", "多模块", "多湖", "蒸干",
		"l0", "l1", "l2", "l3", "l4", "l5", "l6",
		"oss parity", "全功能",
	}
)

// DeriveScopeMode derives the snapshot's scope mode from the
// recorded utterance, following the Python priority: the planner
// axis signals first (parity > mvp > patch), then the decomposition
// axis's ocean signals, else the focused single-lake default.
// brownfield_takeover and research are not derivable here: Python
// derives them from source_refs, which the intent snapshot does
// not carry (see the R2a report's difference list).
func DeriveScopeMode(utterance string) string {
	blob := strings.ToLower(utterance)
	switch {
	case containsAnySignal(blob, scopeParitySignals):
		return "full_parity"
	case containsAnySignal(blob, scopeMVPSignals):
		return "mvp"
	case containsAnySignal(blob, scopePatchSignals):
		return "patch"
	case containsAnySignal(blob, scopeOceanSignals):
		return "ocean"
	}
	return "focused"
}
