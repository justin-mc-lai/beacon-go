package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestDeliveryReceiptCLIRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(*intenttree.DeliveryReceipt)
	}{
		{"missing_red", "delivery_red_green_missing", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Red = nil }},
		{"missing_green", "delivery_red_green_missing", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Green = nil }},
		{"duplicate_ac", "delivery_ledger_open", func(r *intenttree.DeliveryReceipt) { r.Tasks = append(r.Tasks, r.Tasks[0]) }},
		{"stale_log", "delivery_blob_stale", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Green.LogSHA256 = strings.Repeat("c", 64) }},
		{"reverse_review", "delivery_review_invalid", func(r *intenttree.DeliveryReceipt) { r.Review.Verdict = "fail" }},
		{"missing_review", "delivery_review_missing", func(r *intenttree.DeliveryReceipt) { r.Review = nil }},
		{"self_review", "delivery_review_invalid", func(r *intenttree.DeliveryReceipt) { r.Review.Reviewer = "dev" }},
		{"blank_review", "delivery_review_invalid", func(r *intenttree.DeliveryReceipt) { r.Review.Reviewer = "  " }},
		{"open_task", "delivery_ledger_open", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Status = "pending" }},
		{"empty_ledger", "delivery_ledger_open", func(r *intenttree.DeliveryReceipt) { r.Tasks = nil }},
		{"wrong_ac", "delivery_ledger_invalid", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].ACID = "unknown" }},
		{"green_first", "delivery_red_green_invalid", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Green.StartedAt = r.Tasks[0].Red.StartedAt }},
		{"changed_assertions", "delivery_red_green_invalid", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Green.TestSHA256 = strings.Repeat("c", 64) }},
		{"red_passed", "delivery_red_green_invalid", func(r *intenttree.DeliveryReceipt) { zero := 0; r.Tasks[0].Red.ExitCode = &zero }},
		{"green_failed", "delivery_red_green_invalid", func(r *intenttree.DeliveryReceipt) { one := 1; r.Tasks[0].Green.ExitCode = &one }},
		{"missing_exit", "delivery_red_green_invalid", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Green.ExitCode = nil }},
		{"outside_log", "delivery_blob_outside", func(r *intenttree.DeliveryReceipt) { r.Tasks[0].Red.LogPath = "../outside.log" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			if _, err := freeze(t, proj, "t1"); err != nil {
				t.Fatal(err)
			}
			p := writeEvidence(t, proj, "t1")
			mustRun(t, "implement", "report", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", p)
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.SplitN(string(raw), "delivery_receipt: ", 2)
			var r intenttree.DeliveryReceipt
			if err := json.Unmarshal([]byte(parts[1]), &r); err != nil {
				t.Fatal(err)
			}
			if tc.name == "outside_log" {
				if err := os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(p)), "outside.log"), []byte("outside"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			tc.mutate(&r)
			body, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(parts[0]+"delivery_receipt: "+string(body)+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err = runCLI(t, "verify", "leaf", "--project", proj, "--node", "t1", "--by", "qa", "--artifact", p)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("want %s, got %v", tc.code, err)
			}
			n, err := intenttree.ReadNode(filepath.Join(proj, "docs/beacon/v0.1.0/tree"), "t1")
			if err != nil {
				t.Fatal(err)
			}
			if n.Status != intenttree.StatusImplemented {
				t.Fatalf("refusal mutated status: %s", n.Status)
			}
		})
	}
}

func TestDeliveryReceiptStaleBlocksRollupAndDelivery(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	prepareCaptureTree(t, proj)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"t1", "t2"} {
		p := writeEvidence(t, proj, id)
		mustRun(t, "implement", "report", "--project", proj, "--node", id, "--by", "dev", "--artifact", p)
		mustRun(t, "verify", "leaf", "--project", proj, "--node", id, "--by", "qa", "--artifact", writeCapturedEvidence(t, proj, id))
	}
	mustRun(t, "verify", "rollup", "--project", proj, "--node", "root")
	path := filepath.Join(proj, "docs/beacon/v0.1.0/evidence/t1-green.log")
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"verify", "rollup", "--project", proj, "--node", "root"},
		{"truth", "node", "deliver", "--project", proj, "--node", "root", "--by", "owner"},
	} {
		_, err := runCLI(t, args...)
		if err == nil || !strings.Contains(err.Error(), "delivery_blob_stale") {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
