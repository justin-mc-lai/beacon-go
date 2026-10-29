// The machine verification lever (AC-AGENT-004 / AC-AGENT-006):
// `beacon qa verify-harness` answers, for one feature package, whether
// the recorded QA evidence still exists and still matches its recorded
// hash. The answer is computed from the ledger and the bytes on disk —
// there is no constant, no placeholder and no unconditional pass.
//
// This is a READ-ONLY verification: it never writes a gate result.
// Formal verdicts belong to `gate evaluate qa`, which persists them.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// HarnessVerifySchema is the version stamp of the verification document.
const HarnessVerifySchema = "beacon-harness-verify.v1"

// HarnessVerification is one machine-oriented verification answer.
type HarnessVerification struct {
	Schema        string           `json:"schema"`
	Slug          string           `json:"slug"`
	Status        string           `json:"status"` // passed | blocked
	ExitCode      int              `json:"exit_code"`
	Verified      bool             `json:"verified"`
	EvidenceHash  string           `json:"evidence_hash,omitempty"`
	EvidenceCount int              `json:"evidence_count"`
	Stale         []string         `json:"stale,omitempty"`
	ReasonCodes   []string         `json:"reason_codes,omitempty"`
	Checks        []gate.CheckItem `json:"checks"`
	Evaluator     string           `json:"evaluator"`
}

// VerifyHarness runs the read-only verification for one feature.
//
// "passed" requires BOTH of the QA gate's factual halves (QA artifacts
// recorded and the qa stage executed) and that every QA artifact blob
// still matches its recorded content hash. The evidence hash is the
// SHA-256 over the sorted (kind, path, content-hash) triples: it
// changes if any artifact is added, removed, moved or edited, and it is
// the digest a host agent can compare across turns.
func (e *Engine) VerifyHarness(ctx store.FeatureContext, evaluator string) (HarnessVerification, error) {
	if strings.TrimSpace(evaluator) == "" {
		evaluator = "harness"
	}
	out := HarnessVerification{
		Schema: HarnessVerifySchema, Slug: ctx.Feature.Slug, Evaluator: evaluator,
		Status: "blocked", ExitCode: 1,
	}
	ev, err := e.evalQA(ctx, evaluator)
	if err != nil {
		return HarnessVerification{}, err
	}
	out.Checks = ev.Checks
	out.ReasonCodes = ev.ReasonCodes

	runs, err := e.store.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return HarnessVerification{}, err
	}
	type triple struct{ kind, path, hash string }
	var triples []triple
	for _, run := range runs {
		arts, err := e.store.Artifacts().ListByRun(run.ID)
		if err != nil {
			return HarnessVerification{}, err
		}
		for _, a := range arts {
			if !qaKind(a.Kind) {
				continue
			}
			out.EvidenceCount++
			if stale, reason := store.ArtifactStaleness(a); stale {
				out.Stale = append(out.Stale, a.Kind+":"+a.AbsPath+" ("+reason+")")
			}
			triples = append(triples, triple{kind: a.Kind, path: a.AbsPath, hash: a.ContentHash})
		}
	}
	if len(triples) > 0 {
		sort.Slice(triples, func(i, j int) bool {
			if triples[i].path != triples[j].path {
				return triples[i].path < triples[j].path
			}
			return triples[i].kind < triples[j].kind
		})
		h := sha256.New()
		for _, t := range triples {
			h.Write([]byte(t.kind))
			h.Write([]byte{0})
			h.Write([]byte(t.path))
			h.Write([]byte{0})
			h.Write([]byte(t.hash))
			h.Write([]byte{'\n'})
		}
		out.EvidenceHash = hex.EncodeToString(h.Sum(nil))
	}
	if ev.Verdict == gate.VerdictPass && len(out.Stale) == 0 && out.EvidenceHash != "" {
		out.Status = "passed"
		out.ExitCode = 0
		out.Verified = true
	}
	if out.ReasonCodes == nil {
		out.ReasonCodes = []string{}
	}
	return out, nil
}
