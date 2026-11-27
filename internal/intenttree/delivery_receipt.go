package intenttree

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
)

// DeliveryReceipt is external execution evidence, not an authenticated agent identity.
// Each task closes one canonical AC; the receipt cannot invent its own required set.
type DeliveryReceipt struct {
	Schema                 string          `json:"schema"`
	NodeID                 string          `json:"node_id"`
	RequirementFingerprint string          `json:"requirement_fingerprint"`
	SourceCommit           string          `json:"source_commit"`
	BinarySHA256           string          `json:"binary_sha256"`
	Maker                  string          `json:"maker"`
	Tasks                  []DeliveryTask  `json:"tasks"`
	Review                 *DeliveryReview `json:"review"`
}

type DeliveryTask struct {
	ACID   string       `json:"ac_id"`
	Status string       `json:"status"`
	Red    *DeliveryRun `json:"red"`
	Green  *DeliveryRun `json:"green"`
}

type DeliveryRun struct {
	CaptureID  string   `json:"capture_id"`
	Command    []string `json:"command"`
	ExitCode   *int     `json:"exit_code"`
	StartedAt  string   `json:"started_at"`
	FinishedAt string   `json:"finished_at"`
	TestSHA256 string   `json:"test_sha256"`
	LogPath    string   `json:"log_path"`
	LogSHA256  string   `json:"log_sha256"`
}

type DeliveryReview struct {
	Reviewer      string `json:"reviewer"`
	Verdict       string `json:"verdict"`
	OpenFindings  int    `json:"open_findings"`
	ReceiptPath   string `json:"receipt_path"`
	ReceiptSHA256 string `json:"receipt_sha256"`
}

// IndependentReview is the separately supplied reviewer document. The envelope
// cannot certify a pass that its referenced reviewer document does not contain.
type IndependentReview struct {
	Captures               map[string]string `json:"captures"`
	SourceSHA256           string            `json:"source_sha256"`
	NodeID                 string            `json:"node_id"`
	RequirementFingerprint string            `json:"requirement_fingerprint"`
	SourceCommit           string            `json:"source_commit"`
	BinarySHA256           string            `json:"binary_sha256"`
	Reviewer               string            `json:"reviewer"`
	Verdict                string            `json:"verdict"`
	Findings               []string          `json:"findings"`
	ReviewedAt             string            `json:"reviewed_at"`
}

func deliveryError(code, detail string) error {
	return core.NewError(code, "intenttree.delivery", detail)
}

// deliveryBlob resolves aliases before containment checks. References are relative
// to the envelope directory and must remain there, including through symlinks.
func deliveryBlob(base, path, hash string) error {
	if strings.TrimSpace(path) == "" || !sha256Re.MatchString(hash) {
		return deliveryError("delivery_blob_invalid", "nonempty path and SHA256 required")
	}
	root, err := filepath.EvalSymlinks(base)
	if err != nil {
		return err
	}
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return deliveryError("delivery_blob_missing", err.Error())
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return deliveryError("delivery_blob_outside", path)
	}
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return deliveryError("delivery_blob_invalid", path)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != hash {
		return deliveryError("delivery_blob_stale", path)
	}
	return nil
}

// ValidateDeliveryReceipt reads a single JSON header. Logs are hash checked;
// semantic Red quality and reviewer independence still require external review.
func ValidateDeliveryReceipt(n Node, meta EvidenceMeta, path string, raw []byte) error {
	var receipt DeliveryReceipt
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "delivery_receipt" {
			count++
			if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &receipt); err != nil {
				return deliveryError("delivery_receipt_invalid", err.Error())
			}
		}
	}
	if count == 0 {
		return deliveryError("delivery_receipt_missing", "delivery_receipt JSON header required")
	}
	if count != 1 {
		return deliveryError("delivery_receipt_invalid", "duplicate receipt headers")
	}
	if receipt.Schema != "beacon-delivery.v1" || receipt.NodeID != n.ID || receipt.RequirementFingerprint != meta.RequirementFingerprint || receipt.SourceCommit != meta.SourceCommit || receipt.BinarySHA256 != meta.BinarySHA256 {
		return deliveryError("delivery_receipt_binding", "schema/node/requirement/source/binary differs")
	}
	if strings.TrimSpace(receipt.Maker) == "" || n.Implementation == nil || receipt.Maker != strings.TrimSpace(n.Implementation.By) {
		return deliveryError("delivery_maker_mismatch", "receipt maker must match implementation")
	}
	expected := map[string]bool{}
	for _, ac := range n.ACs {
		if strings.TrimSpace(ac.ID) == "" || expected[ac.ID] {
			return deliveryError("delivery_ledger_invalid", "blank/duplicate canonical AC")
		}
		expected[ac.ID] = true
	}
	if len(expected) == 0 || len(receipt.Tasks) != len(expected) {
		return deliveryError("delivery_ledger_open", "every canonical AC requires one closed task")
	}
	base := filepath.Dir(path)
	seen := map[string]bool{}
	var lastGreen time.Time
	for _, task := range receipt.Tasks {
		if !expected[task.ACID] || seen[task.ACID] {
			return deliveryError("delivery_ledger_invalid", "unknown/duplicate AC: "+task.ACID)
		}
		seen[task.ACID] = true
		if task.Status != "done" {
			return deliveryError("delivery_ledger_open", task.ACID)
		}
		if task.Red == nil || task.Green == nil {
			return deliveryError("delivery_red_green_missing", task.ACID)
		}
		redEnd, err := validateDeliveryRun(base, task.Red, false)
		if err != nil {
			return err
		}
		greenEnd, err := validateDeliveryRun(base, task.Green, true)
		if err != nil {
			return err
		}
		greenStart, _ := time.Parse(time.RFC3339Nano, task.Green.StartedAt)
		redCommand, _ := json.Marshal(task.Red.Command)
		greenCommand, _ := json.Marshal(task.Green.Command)
		if !greenStart.After(redEnd) || string(redCommand) != string(greenCommand) || task.Red.TestSHA256 != task.Green.TestSHA256 {
			return deliveryError("delivery_red_green_invalid", "same test/command must run Red before Green")
		}
		if greenEnd.After(lastGreen) {
			lastGreen = greenEnd
		}
	}
	review := receipt.Review
	if review == nil {
		return deliveryError("delivery_review_missing", "independent review required")
	}
	if strings.TrimSpace(review.Reviewer) == "" || strings.EqualFold(strings.TrimSpace(review.Reviewer), strings.TrimSpace(receipt.Maker)) || review.Verdict != "pass" || review.OpenFindings != 0 {
		return deliveryError("delivery_review_invalid", "distinct reviewer, pass and zero open findings required")
	}
	if err := deliveryBlob(base, review.ReceiptPath, review.ReceiptSHA256); err != nil {
		return err
	}
	reviewPath := review.ReceiptPath
	if !filepath.IsAbs(reviewPath) {
		reviewPath = filepath.Join(base, reviewPath)
	}
	reviewRaw, err := os.ReadFile(reviewPath)
	if err != nil {
		return err
	}
	var independent IndependentReview
	if err := json.Unmarshal(reviewRaw, &independent); err != nil {
		return deliveryError("delivery_review_invalid", err.Error())
	}
	reviewedAt, err := time.Parse(time.RFC3339Nano, independent.ReviewedAt)
	if err != nil || !reviewedAt.After(lastGreen) || reviewedAt.After(time.Now().UTC()) || independent.NodeID != n.ID || independent.RequirementFingerprint != receipt.RequirementFingerprint || independent.SourceCommit != receipt.SourceCommit || independent.BinarySHA256 != receipt.BinarySHA256 || independent.Reviewer != review.Reviewer || independent.Verdict != "pass" || independent.Findings == nil || len(independent.Findings) != 0 {
		return deliveryError("delivery_review_invalid", "review content, candidate binding or timestamp differs")
	}
	return nil
}

func validateDeliveryRun(base string, run *DeliveryRun, green bool) (time.Time, error) {
	bad := func() (time.Time, error) {
		return time.Time{}, deliveryError("delivery_red_green_invalid", "command, exit code, test hash and ordered timestamps required")
	}
	if len(run.Command) == 0 || run.ExitCode == nil || !sha256Re.MatchString(run.TestSHA256) {
		return bad()
	}
	for _, arg := range run.Command {
		if strings.TrimSpace(arg) == "" {
			return bad()
		}
	}
	if (green && *run.ExitCode != 0) || (!green && (*run.ExitCode <= 0 || *run.ExitCode >= 126)) {
		return bad()
	}
	start, err := time.Parse(time.RFC3339Nano, run.StartedAt)
	if err != nil {
		return bad()
	}
	end, err := time.Parse(time.RFC3339Nano, run.FinishedAt)
	if err != nil || !end.After(start) || end.After(time.Now().UTC()) {
		return bad()
	}
	if err := deliveryBlob(base, run.LogPath, run.LogSHA256); err != nil {
		return time.Time{}, err
	}
	return end, nil
}
