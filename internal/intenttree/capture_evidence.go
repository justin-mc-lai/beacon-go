package intenttree

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"beacon-go/internal/store"
)

// CaptureEvidence is the process-owned record shared by producer and consumers.
type CaptureEvidence struct {
	BaselineCommit     string            `json:"baseline_commit,omitempty"`
	ReplayCandidate    string            `json:"replay_candidate,omitempty"`
	ReplayTargetSHA256 string            `json:"replay_target_sha256,omitempty"`
	ExecutionRoot      string            `json:"execution_root,omitempty"`
	Schema             string            `json:"schema"`
	NodeID             string            `json:"node_id"`
	ACID               string            `json:"ac_id"`
	Phase              string            `json:"phase"`
	By                 string            `json:"by"`
	RedID              string            `json:"red_id,omitempty"`
	Fingerprint        string            `json:"requirement_fingerprint"`
	Command            []string          `json:"command"`
	TestSHA256         string            `json:"test_sha256"`
	SourceSHA256       string            `json:"source_sha256"`
	SourceCommit       string            `json:"source_commit,omitempty"`
	BinarySHA256       string            `json:"binary_sha256"`
	Workdir            string            `json:"workdir"`
	TestFiles          []string          `json:"test_files"`
	SourceAfter        string            `json:"source_after"`
	StartedAt          string            `json:"started_at"`
	FinishedAt         string            `json:"finished_at"`
	ExitCode           int               `json:"exit_code"`
	TimedOut           bool              `json:"timed_out"`
	Cancelled          bool              `json:"cancelled"`
	Status             string            `json:"status"`
	Reason             string            `json:"reason,omitempty"`
	ResultPath         string            `json:"result_path"`
	StdoutPath         string            `json:"stdout_path"`
	StderrPath         string            `json:"stderr_path"`
	StdoutSHA256       string            `json:"stdout_sha256"`
	StderrSHA256       string            `json:"stderr_sha256"`
	ArtifactID         string            `json:"artifact_id,omitempty"`
	Preflight          *CapturePreflight `json:"preflight,omitempty"`
}

type CapturePreflight struct {
	Command  []string `json:"command"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	TimedOut bool     `json:"timed_out"`
	Error    string   `json:"error,omitempty"`
}

func CaptureSourceHash(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if rel == ".git" || rel == ".beacon" || filepath.ToSlash(rel) == "docs/beacon" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return deliveryError("capture_source_invalid", rel)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		fmt.Fprintf(h, "%s\x00%x\n", filepath.ToSlash(rel), sum)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

func CaptureTestHash(root string, names []string) (string, error) {
	if len(names) == 0 {
		return "", deliveryError("capture_test_files_invalid", "empty tests")
	}
	paths := map[string]string{}
	for _, name := range names {
		p := name
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		p, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", deliveryError("capture_test_files_invalid", name)
		}
		if _, exists := paths[rel]; exists {
			return "", deliveryError("capture_test_files_invalid", "duplicate/alias test")
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(b)
		paths[rel] = hex.EncodeToString(sum[:])
	}
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, key := range keys {
		fmt.Fprintf(h, "%s\x00%s\n", key, paths[key])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ReadCaptureEvidence(st *store.Store, nodeID, id string) (CaptureEvidence, error) {
	var result CaptureEvidence
	arts, err := st.Artifacts().ListByScope("node", nodeID)
	if err != nil {
		return result, err
	}
	for _, art := range arts {
		if art.ID != id || art.Kind != "capture_result" {
			continue
		}
		raw, err := os.ReadFile(art.AbsPath)
		if err != nil {
			return result, err
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != art.ContentHash {
			return result, deliveryError("capture_stale", "result hash differs")
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return result, err
		}
		if result.Schema != "beacon-capture.v1" || result.NodeID != nodeID || filepath.Clean(result.ResultPath) != filepath.Clean(art.AbsPath) {
			return result, deliveryError("capture_stale", "scope/path differs")
		}
		for _, log := range []struct{ path, hash string }{{result.StdoutPath, result.StdoutSHA256}, {result.StderrPath, result.StderrSHA256}} {
			found := false
			for _, a := range arts {
				if a.Kind == "log" && a.AbsPath == log.path && a.ContentHash == log.hash {
					found = true
				}
			}
			if !found {
				return result, deliveryError("capture_not_found", "unregistered log")
			}
			// Empty stderr is valid, unlike a missing test transcript.
			b, err := os.ReadFile(log.path)
			if err != nil {
				return result, err
			}
			s := sha256.Sum256(b)
			if !sha256Re.MatchString(log.hash) || hex.EncodeToString(s[:]) != log.hash || filepath.Dir(log.path) != filepath.Dir(art.AbsPath) {
				return result, deliveryError("capture_stale", "log hash/path differs")
			}
		}
		result.ArtifactID = id
		return result, nil
	}
	return result, deliveryError("capture_not_found", "registered capture ID required")
}

// ValidateCaptureDelivery is the shared provenance guard for ALL verified writes
// and rechecks. External envelope fields can never substitute for reserved rows.
func ValidateCaptureDelivery(st *store.Store, root string, n Node, path string) error {
	if _, err := ValidateEvidenceFile(root, n.ID, n, path); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var receipt DeliveryReceipt
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "delivery_receipt" {
			if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &receipt); err != nil {
				return err
			}
		}
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	currentSource, err := CaptureSourceHash(canonicalRoot)
	if err != nil {
		return err
	}
	reviewPath := receipt.Review.ReceiptPath
	if !filepath.IsAbs(reviewPath) {
		reviewPath = filepath.Join(filepath.Dir(path), reviewPath)
	}
	b, err := os.ReadFile(reviewPath)
	if err != nil {
		return err
	}
	var review IndependentReview
	if err := json.Unmarshal(b, &review); err != nil {
		return err
	}
	if len(review.Captures) != len(receipt.Tasks) {
		return deliveryError("delivery_capture_required", "review must bind every Green capture")
	}
	fp, err := ContractFingerprint(n)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, task := range receipt.Tasks {
		if task.Red.CaptureID == "" || task.Green.CaptureID == "" {
			return deliveryError("delivery_capture_required", task.ACID)
		}
		red, err := ReadCaptureEvidence(st, n.ID, task.Red.CaptureID)
		if err != nil {
			return err
		}
		green, err := ReadCaptureEvidence(st, n.ID, task.Green.CaptureID)
		if err != nil {
			return err
		}
		if seen[red.ArtifactID] || seen[green.ArtifactID] {
			return deliveryError("delivery_capture_invalid", "reused capture")
		}
		seen[red.ArtifactID] = true
		seen[green.ArtifactID] = true
		for _, pair := range []struct {
			capture CaptureEvidence
			run     *DeliveryRun
			phase   string
		}{{red, task.Red, "red"}, {green, task.Green, "green"}} {
			c, r := pair.capture, pair.run
			workdir, err := filepath.EvalSymlinks(c.Workdir)
			if err != nil {
				return err
			}
			if c.Status != "accepted" || c.Phase != pair.phase || c.ACID != task.ACID || c.By != receipt.Maker || c.Fingerprint != fp || workdir != canonicalRoot || c.TimedOut || c.Cancelled || c.SourceSHA256 == "" || c.SourceAfter != c.SourceSHA256 || r.ExitCode == nil || c.ExitCode != *r.ExitCode || c.StartedAt != r.StartedAt || c.FinishedAt != r.FinishedAt || c.TestSHA256 != r.TestSHA256 || !reflect.DeepEqual(c.Command, r.Command) {
				return deliveryError("delivery_capture_invalid", "capture/envelope differs")
			}
			if r.LogSHA256 != c.StdoutSHA256 {
				return deliveryError("delivery_capture_invalid", "transcript differs")
			}
		}
		testHash, err := CaptureTestHash(canonicalRoot, green.TestFiles)
		if err != nil {
			return err
		}
		if green.RedID != red.ArtifactID || red.TestSHA256 != green.TestSHA256 || testHash != green.TestSHA256 || !reflect.DeepEqual(red.Command, green.Command) || green.SourceSHA256 != currentSource || review.SourceSHA256 != currentSource || green.BinarySHA256 != receipt.BinarySHA256 || review.Captures[task.ACID] != green.ArtifactID || (green.SourceCommit != "" && green.SourceCommit != receipt.SourceCommit) {
			return deliveryError("delivery_capture_stale", "pair, candidate, tests or review differs")
		}
		if red.BaselineCommit != "" && (red.SourceCommit != red.BaselineCommit || red.ReplayCandidate != green.SourceCommit || red.ReplayTargetSHA256 != green.SourceSHA256) {
			return deliveryError("delivery_capture_stale", "historical Red targets a different Green candidate")
		}
		if red.BaselineCommit != "" {
			head, e := exec.Command("git", "-C", canonicalRoot, "rev-parse", "HEAD").Output()
			if e != nil || strings.TrimSpace(string(head)) != red.ReplayCandidate {
				return deliveryError("delivery_capture_stale", "current HEAD differs from replay target")
			}
		}
	}
	return nil
}
