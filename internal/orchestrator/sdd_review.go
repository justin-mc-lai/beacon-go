package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

type SDDReviewReceipt struct {
	Fingerprint string               `json:"fingerprint"`
	Reviewer    string               `json:"reviewer"`
	Verdict     string               `json:"verdict"`
	Findings    []SDDSemanticFinding `json:"findings"`
	Summary     string               `json:"summary"`
}

type SDDSemanticFinding struct {
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

// RecordSDDReview consumes a real externally supplied review; the kernel never
// produces a semantic passing verdict on its own.
func (e *Engine) RecordSDDReview(fc store.FeatureContext, receiptPath string) (string, error) {
	pkg := e.packageFiles(fc)
	if pkg.Contract != SDDContract {
		return "", fmt.Errorf("semantic review receipt requires an SDD package")
	}
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		return "", err
	}
	var receipt SDDReviewReceipt
	if err = json.Unmarshal(raw, &receipt); err != nil {
		return "", err
	}
	scalars, _, _ := gate.ParseFrontmatter(pkg.Truth.Text)
	if strings.TrimSpace(scalars["package_author"]) == "" || strings.TrimSpace(receipt.Reviewer) == "" || strings.EqualFold(strings.TrimSpace(receipt.Reviewer), strings.TrimSpace(scalars["package_author"])) {
		return "", fmt.Errorf("sdd_review_self: reviewer must differ from declared package_author")
	}
	fingerprint := gate.SDDReviewFingerprint(pkg)
	if receipt.Fingerprint != fingerprint {
		return "", fmt.Errorf("sdd_review_stale: receipt fingerprint does not match current content")
	}
	if receipt.Verdict != "pass" || strings.TrimSpace(receipt.Summary) == "" {
		return "", fmt.Errorf("sdd_review_required: a passing review with a substantive summary is required")
	}
	for _, f := range receipt.Findings {
		switch strings.ToLower(f.Severity) {
		case "critical", "high", "error", "blocker":
			return "", fmt.Errorf("sdd_review_blocked: %s", f.Detail)
		case "warning", "info", "low":
		default:
			return "", fmt.Errorf("sdd_review_invalid: unknown severity %q", f.Severity)
		}
	}
	// Record the full immutable input on the event trail before exposing signing metadata.
	if err = e.appendFeatureEvent(fc.Feature.ID, "sdd_semantic_review_recorded", map[string]any{"receipt": receipt, "recorded_at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		return "", err
	}
	text := pkg.Truth.Text
	for _, pair := range [][2]string{{"sdd_review_by", receipt.Reviewer}, {"sdd_review_verdict", "pass"}, {"sdd_review_at", time.Now().UTC().Format(time.RFC3339Nano)}, {"sdd_review_fingerprint", fingerprint}, {"intent_confirmed", "false"}} {
		text, _ = setFrontmatterScalar(text, pair[0], pair[1])
	}
	if err = writeTruthText(fc, text); err != nil {
		return "", err
	}
	return fingerprint, nil
}

func (e *Engine) SDDReviewFingerprint(fc store.FeatureContext) string {
	return gate.SDDReviewFingerprint(e.packageFiles(fc))
}

func (e *Engine) sddRecordedReviewFindings(fc store.FeatureContext, pkg gate.PackageFiles, confirmed bool) []gate.Finding {
	findings := gate.SDDReviewFindings(pkg, confirmed)
	if pkg.Contract != SDDContract {
		return findings
	}
	missing := func(detail string) []gate.Finding {
		return append(findings, gate.Finding{Kind: "sdd_review_unrecorded", ReasonCode: "sdd_review_unrecorded", File: pkg.Truth.Path, Detail: detail, RepairRoute: gate.DefaultRepairRoute})
	}
	events, err := e.store.ListEvents(fc.Feature.ID)
	if err != nil {
		return missing("cannot read semantic review trail: " + err.Error())
	}
	scalars, _, _ := gate.ParseFrontmatter(pkg.Truth.Text)
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != "sdd_semantic_review_recorded" {
			continue
		}
		var payload struct {
			Receipt SDDReviewReceipt `json:"receipt"`
		}
		if json.Unmarshal(events[i].Payload, &payload) != nil {
			return missing("semantic review trail is unreadable")
		}
		if payload.Receipt.Fingerprint == gate.SDDReviewFingerprint(pkg) && payload.Receipt.Reviewer == scalars["sdd_review_by"] && payload.Receipt.Verdict == "pass" {
			return findings
		}
		return missing("latest recorded review does not bind this content and reviewer")
	}
	return missing("no recorded external semantic review receipt")
}
