// Package changequality carries the premerge change-quality policy
// contract (kernel-change-quality): the off / safe_fix / strict_receipt
// mode vocabulary, the exact-diff-SHA receipt check, and the single
// bounded safe-fix gate.
//
// The package is a refusal surface, not a convenience layer: unknown
// modes fall back to off with an explicit configuration error, a stale
// or malformed receipt is a terminal refusal, and a repair that leaves
// the caller's authorization never applies a partial change.
package changequality

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Mode is the change-quality policy mode.
type Mode string

const (
	// ModeOff disables receipt checks and repairs.
	ModeOff Mode = "off"
	// ModeSafeFix allows at most one bounded repair per policy call.
	ModeSafeFix Mode = "safe_fix"
	// ModeStrictReceipt refuses a premerge whose receipt does not bind
	// the exact current diff.
	ModeStrictReceipt Mode = "strict_receipt"
)

// Reason codes surfaced by this package.
const (
	ReasonInvalidMode       = "change_quality_invalid_mode"
	ReasonReceiptMissing    = "receipt_missing"
	ReasonReceiptMalformed  = "receipt_malformed"
	ReasonStaleReceipt      = "stale_receipt"
	ReasonBaselineMismatch  = "receipt_baseline_mismatch"
	ReasonModeDoesNotAllow  = "safe_fix_mode_required"
	ReasonRepairBudgetSpent = "safe_fix_budget_exhausted"
	ReasonOutOfAuth         = "safe_fix_out_of_authorization"
	ReasonRepairNotApplied  = "safe_fix_not_applicable"
)

// ConfigError reports a mode value outside the vocabulary. The policy
// still resolves to off; callers are expected to surface this error.
type ConfigError struct {
	Value string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s: %q is not one of off, safe_fix, strict_receipt", ReasonInvalidMode, e.Value)
}

// Is implements errors.Is for the invalid-mode reason.
func (e *ConfigError) Is(target error) bool {
	return target == ErrInvalidMode
}

// ErrInvalidMode is the sentinel for an unrecognized policy value.
var ErrInvalidMode = errors.New(ReasonInvalidMode)

// ParseMode parses one raw policy value. An unrecognized value (or an
// empty one, which means unset) resolves to off; only an unrecognized
// non-empty value returns a ConfigError alongside off.
func ParseMode(raw string) (Mode, error) {
	switch strings.TrimSpace(raw) {
	case "", string(ModeOff):
		return ModeOff, nil
	case string(ModeSafeFix):
		return ModeSafeFix, nil
	case string(ModeStrictReceipt):
		return ModeStrictReceipt, nil
	default:
		return ModeOff, &ConfigError{Value: raw}
	}
}

// Policy is one resolved change-quality policy.
type Policy struct {
	mode   Mode
	raw    string
	err    error
	source string
}

// NewPolicy resolves a raw policy value. Unknown values stay off and
// carry their configuration error; they are never upgraded.
func NewPolicy(raw string) Policy {
	mode, err := ParseMode(raw)
	return Policy{mode: mode, raw: raw, err: err}
}

// NewPolicyFromGoal resolves a policy from a goal-level and a package-
// level value. The goal value wins when set; the package value is the
// fallback. Both go through ParseMode.
func NewPolicyFromGoal(goalValue, packageValue string) Policy {
	if strings.TrimSpace(goalValue) != "" {
		p := NewPolicy(goalValue)
		p.source = "goal"
		return p
	}
	p := NewPolicy(packageValue)
	p.source = "package"
	return p
}

// Mode returns the resolved mode. Unknown values resolve to off.
func (p Policy) Mode() Mode { return p.mode }

// ConfigError returns the configuration error, if the raw value was
// outside the vocabulary.
func (p Policy) ConfigError() error { return p.err }

// Source names where the policy value came from (goal or package).
func (p Policy) Source() string { return p.source }

// Hunk is one diff hunk. ID identifies the hunk for authorization.
type Hunk struct {
	ID  string
	Old string
	New string
}

// FileDiff is the changed content of one file.
type FileDiff struct {
	Path  string
	Hunks []Hunk
}

// CandidateDiff is the premerge candidate. Baseline is the commit the
// diff was taken from; the exact SHA binds baseline, file set and
// content.
type CandidateDiff struct {
	Baseline string
	Files    []FileDiff
}

func writeField(b *strings.Builder, field string) {
	fmt.Fprintf(b, "%d:%s\n", len(field), field)
}

// Canonical returns the canonical serialization of the candidate diff:
// files sorted by path, hunks sorted by ID then content, each field
// length-prefixed so no value can forge a boundary.
func (d CandidateDiff) Canonical() string {
	var b strings.Builder
	writeField(&b, "beacon-change-quality-diff-v1")
	writeField(&b, d.Baseline)
	files := make([]FileDiff, len(d.Files))
	copy(files, d.Files)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	for _, f := range files {
		writeField(&b, f.Path)
		hunks := make([]Hunk, len(f.Hunks))
		copy(hunks, f.Hunks)
		sort.Slice(hunks, func(i, j int) bool {
			if hunks[i].ID != hunks[j].ID {
				return hunks[i].ID < hunks[j].ID
			}
			if hunks[i].Old != hunks[j].Old {
				return hunks[i].Old < hunks[j].Old
			}
			return hunks[i].New < hunks[j].New
		})
		for _, h := range hunks {
			writeField(&b, h.ID)
			writeField(&b, h.Old)
			writeField(&b, h.New)
		}
	}
	return b.String()
}

// SHA256 is the exact current diff SHA over the canonical form.
func (d CandidateDiff) SHA256() string {
	sum := sha256.Sum256([]byte(d.Canonical()))
	return hex.EncodeToString(sum[:])
}

// ErrReceiptMissing rejects an absent or empty receipt.
var ErrReceiptMissing = errors.New(ReasonReceiptMissing)

// ErrReceiptMalformed rejects an unparseable receipt.
var ErrReceiptMalformed = errors.New(ReasonReceiptMalformed)

// Receipt is the premerge receipt binding a diff SHA and its baseline.
type Receipt struct {
	DiffSHA  string `json:"diff_sha"`
	Baseline string `json:"baseline"`
}

// ParseReceipt parses a JSON receipt. An empty payload is a missing
// receipt; invalid JSON or an empty diff_sha is a malformed one.
func ParseReceipt(raw []byte) (Receipt, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Receipt{}, ErrReceiptMissing
	}
	var r Receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return Receipt{}, fmt.Errorf("%w: %v", ErrReceiptMalformed, err)
	}
	if strings.TrimSpace(r.DiffSHA) == "" {
		return Receipt{}, ErrReceiptMissing
	}
	return r, nil
}

// Decision is the premerge verdict. A refusal is terminal: the mode is
// still reported so callers cannot mistake it for a downgrade to off.
type Decision struct {
	Allowed bool
	Mode    Mode
	Reason  string
	Detail  string
}

// EvaluatePremerge decides whether the candidate may merge. off always
// allows. strict_receipt recomputes the exact current diff SHA and
// refuses on a missing, malformed, stale or baseline-mismatched
// receipt. safe_fix does not add a premerge receipt requirement.
func (p Policy) EvaluatePremerge(cand CandidateDiff, rawReceipt []byte) Decision {
	d := Decision{Allowed: true, Mode: p.mode}
	if p.mode != ModeStrictReceipt {
		return d
	}
	receipt, err := ParseReceipt(rawReceipt)
	if err != nil {
		d.Allowed = false
		d.Reason = ReasonReceiptMalformed
		if errors.Is(err, ErrReceiptMissing) {
			d.Reason = ReasonReceiptMissing
		}
		d.Detail = err.Error()
		return d
	}
	if strings.TrimSpace(receipt.Baseline) != strings.TrimSpace(cand.Baseline) {
		d.Allowed = false
		d.Reason = ReasonBaselineMismatch
		d.Detail = fmt.Sprintf("receipt baseline %q, candidate baseline %q", receipt.Baseline, cand.Baseline)
		return d
	}
	current := cand.SHA256()
	if !strings.EqualFold(strings.TrimSpace(receipt.DiffSHA), current) {
		d.Allowed = false
		d.Reason = ReasonStaleReceipt
		d.Detail = fmt.Sprintf("receipt diff_sha %q, current diff %q", receipt.DiffSHA, current)
		return d
	}
	return d
}

// Authorization is the caller's explicit repair permission: file path
// to the hunk IDs the caller authorized.
type Authorization struct {
	Files map[string][]string
}

func (a Authorization) allows(path, hunkID string) bool {
	ids, ok := a.Files[path]
	if !ok {
		return false
	}
	for _, id := range ids {
		if id == hunkID {
			return true
		}
	}
	return false
}

// RepairHunk is one bounded replacement proposed by a repair.
type RepairHunk struct {
	ID  string
	Old string
	New string
}

// FileRepair is the repair for one file.
type FileRepair struct {
	Path  string
	Hunks []RepairHunk
}

// Repair is one proposed bounded fix.
type Repair struct {
	Files []FileRepair
}

// RepairResult reports one applied repair.
type RepairResult struct {
	Applied      bool
	FilesChanged []string
	Attempts     int
}

var (
	// ErrModeDoesNotAllow rejects a repair request under a non-safe-fix mode.
	ErrModeDoesNotAllow = errors.New(ReasonModeDoesNotAllow)
	// ErrRepairBudgetExhausted rejects a second repair attempt.
	ErrRepairBudgetExhausted = errors.New(ReasonRepairBudgetSpent)
	// ErrOutOfAuthorization rejects a repair touching anything outside the
	// caller's authorization; nothing is applied.
	ErrOutOfAuthorization = errors.New(ReasonOutOfAuth)
	// ErrRepairNotApplicable rejects a repair whose hunks do not match the tree.
	ErrRepairNotApplicable = errors.New(ReasonRepairNotApplied)
)

// SafeFixGate applies at most one authorized repair per policy call.
type SafeFixGate struct {
	policy      Policy
	auth        Authorization
	tree        map[string]string
	attempts    int
	maxAttempts int
}

// NewSafeFixGate builds a gate for one policy call. The tree is copied,
// so the caller's map is never mutated.
func NewSafeFixGate(p Policy, auth Authorization, tree map[string]string) (*SafeFixGate, error) {
	if p.Mode() != ModeSafeFix {
		return nil, ErrModeDoesNotAllow
	}
	copyTree := make(map[string]string, len(tree))
	for path, content := range tree {
		copyTree[path] = content
	}
	return &SafeFixGate{policy: p, auth: auth, tree: copyTree, maxAttempts: 1}, nil
}

// Attempts reports how many repairs have been applied.
func (g *SafeFixGate) Attempts() int { return g.attempts }

// Tree returns a copy of the current tree.
func (g *SafeFixGate) Tree() map[string]string {
	out := make(map[string]string, len(g.tree))
	for path, content := range g.tree {
		out[path] = content
	}
	return out
}

// Request applies one repair or refuses. Authorization is checked for
// every file and hunk before any change; a refusal leaves the tree
// exactly as it was.
func (g *SafeFixGate) Request(r Repair) (RepairResult, error) {
	result := RepairResult{Attempts: g.attempts}
	if g.attempts >= g.maxAttempts {
		return result, ErrRepairBudgetExhausted
	}
	if len(r.Files) == 0 {
		return result, ErrRepairNotApplicable
	}
	for _, f := range r.Files {
		for _, h := range f.Hunks {
			if !g.auth.allows(f.Path, h.ID) {
				return result, fmt.Errorf("%w: %s hunk %s", ErrOutOfAuthorization, f.Path, h.ID)
			}
		}
	}
	next := g.Tree()
	changed := make([]string, 0, len(r.Files))
	seen := make(map[string]bool, len(r.Files))
	for _, f := range r.Files {
		content, ok := next[f.Path]
		if !ok {
			return result, fmt.Errorf("%w: %s not in tree", ErrRepairNotApplicable, f.Path)
		}
		for _, h := range f.Hunks {
			if h.Old == "" || !strings.Contains(content, h.Old) {
				return result, fmt.Errorf("%w: %s hunk %s", ErrRepairNotApplicable, f.Path, h.ID)
			}
			content = strings.Replace(content, h.Old, h.New, 1)
		}
		next[f.Path] = content
		if !seen[f.Path] {
			seen[f.Path] = true
			changed = append(changed, f.Path)
		}
	}
	sort.Strings(changed)
	g.tree = next
	g.attempts++
	return RepairResult{Applied: true, FilesChanged: changed, Attempts: g.attempts}, nil
}
