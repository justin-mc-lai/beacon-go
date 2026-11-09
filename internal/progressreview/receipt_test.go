package progressreview

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func testRevision(t *testing.T, contract string) string {
	t.Helper()
	rev := DeriveContractRevision(contract)
	if len(rev) != RevisionLength {
		t.Fatalf("derived revision length = %d, want %d", len(rev), RevisionLength)
	}
	return rev
}

func baseReceipt(eventID string, status Status, drift bool, revision string) Receipt {
	return Receipt{
		SchemaVersion:    SchemaVersion,
		EventID:          eventID,
		EvidenceID:       "evidence-" + eventID,
		Status:           status,
		Run:              Run{RunID: "run-1", Stage: "implement", GoalID: "goal-1"},
		Judgments:        []Judgment{{QuestionID: "q1", Verdict: "drift"}},
		DriftSignal:      drift,
		QuestionVersion:  "progress-questions-v1",
		ContractRevision: revision,
	}
}

func mustReviewer(t *testing.T, opts Options) *Reviewer {
	t.Helper()
	r, err := New(opts)
	if err != nil {
		t.Fatalf("New(%+v) error = %v", opts, err)
	}
	return r
}

func mustReview(t *testing.T, r *Reviewer, receipt Receipt) Decision {
	t.Helper()
	d, err := r.Review(receipt)
	if err != nil {
		t.Fatalf("Review(%s) error = %v", receipt.EventID, err)
	}
	return d
}

func TestProgressReviewReceiptSchemaAndDefaultMode(t *testing.T) {
	if SchemaVersion != "progress_review_receipt_v0" {
		t.Fatalf("SchemaVersion = %q", SchemaVersion)
	}
	if DefaultMode != ModeOff || DefaultDriftThreshold != 2 {
		t.Fatalf("defaults = mode %q threshold %d, want off/2", DefaultMode, DefaultDriftThreshold)
	}
	wantModes := []Mode{ModeOff, ModeShadow, ModeAssist}
	gotModes := Modes()
	if len(gotModes) != len(wantModes) {
		t.Fatalf("modes = %v, want %v", gotModes, wantModes)
	}
	for i := range wantModes {
		if gotModes[i] != wantModes[i] {
			t.Fatalf("modes = %v, want %v", gotModes, wantModes)
		}
	}
	if mode, err := ParseMode(""); err != nil || mode != ModeOff {
		t.Fatalf("ParseMode(empty) = %q, %v; want off, nil", mode, err)
	}
	if mode, err := ParseMode(" SHADOW "); err != nil || mode != ModeShadow {
		t.Fatalf("ParseMode(spaced shadow) = %q, %v; want shadow, nil", mode, err)
	}
	if _, err := ParseMode("turbo"); err == nil {
		t.Fatal("ParseMode must reject a word outside off/shadow/assist")
	}

	rev := testRevision(t, "goal contract v1")
	receipt := baseReceipt("evt-schema", StatusCompleted, true, rev)
	if err := receipt.Validate(); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("receipt JSON is not an object: %v", err)
	}
	for _, key := range []string{
		"schema_version", "event_id", "evidence_id", "status", "run",
		"judgments", "drift_signal", "question_version", "contract_revision",
	} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("receipt JSON missing %q: %s", key, raw)
		}
	}
	var roundTrip Receipt
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal receipt: %v", err)
	}
	if roundTrip.SchemaVersion != SchemaVersion || roundTrip.EventID != receipt.EventID {
		t.Fatalf("round trip = %+v", roundTrip)
	}

	for _, word := range []string{"completed", "abstained", "failed", "not_evaluated", "stale"} {
		if got := NormalizeStatus(word); string(got) != word {
			t.Fatalf("NormalizeStatus(%q) = %q", word, got)
		}
	}
	for _, word := range []string{"unknown", "", "made_up"} {
		if got := NormalizeStatus(word); got != StatusUnknown {
			t.Fatalf("NormalizeStatus(%q) = %q, want unknown", word, got)
		}
	}
	if !StatusCompleted.Handled() {
		t.Fatal("completed must be handled")
	}
	for _, s := range []Status{StatusAbstained, StatusFailed, StatusNotEvaluated, StatusStale, StatusUnknown} {
		if s.Handled() {
			t.Fatalf("%q must never be handled", s)
		}
	}

	broken := baseReceipt("evt-broken", StatusCompleted, true, "")
	broken.SchemaVersion = "progress_review_receipt_v9"
	if err := broken.Validate(); err == nil {
		t.Fatal("wrong schema_version must be rejected")
	}
	broken = baseReceipt("evt-broken", StatusCompleted, true, "")
	broken.EventID = " "
	if err := broken.Validate(); err == nil {
		t.Fatal("empty event_id must be rejected")
	}
	broken = baseReceipt("evt-broken", StatusCompleted, true, "")
	broken.QuestionVersion = ""
	if err := broken.Validate(); err == nil {
		t.Fatal("empty question_version must be rejected")
	}

	off := mustReviewer(t, Options{})
	if off.Mode() != ModeOff {
		t.Fatalf("default reviewer mode = %q, want off", off.Mode())
	}
	d := mustReview(t, off, baseReceipt("evt-off", StatusCompleted, true, rev))
	if d.Recorded || d.Drivable || len(off.Receipts()) != 0 {
		t.Fatalf("off mode produced a drivable receipt: %+v", d)
	}
}

func TestShadowModeRecordsReceiptWithoutReplanObligation(t *testing.T) {
	rev := testRevision(t, "goal contract v1")
	shadow := mustReviewer(t, Options{Mode: ModeShadow, Threshold: 2, PinnedRevision: rev})
	for i := 1; i <= 2; i++ {
		d := mustReview(t, shadow, baseReceipt(fmt.Sprintf("evt-shadow-%d", i), StatusCompleted, true, rev))
		if !d.Recorded {
			t.Fatalf("shadow receipt %d not recorded: %+v", i, d)
		}
		if d.Drivable || d.Counted || d.Obligation != nil {
			t.Fatalf("shadow receipt %d drove a decision: %+v", i, d)
		}
		if d.Consecutive != 0 {
			t.Fatalf("shadow receipt %d counted as drift: %+v", i, d)
		}
	}
	if len(shadow.Receipts()) != 2 {
		t.Fatalf("shadow recorded %d receipts, want 2", len(shadow.Receipts()))
	}
	if shadow.Obligation() != nil || shadow.ConsecutiveDrift() != 0 {
		t.Fatalf("shadow raised obligation/streak: %+v / %d", shadow.Obligation(), shadow.ConsecutiveDrift())
	}

	off := mustReviewer(t, Options{Mode: ModeOff, PinnedRevision: rev})
	d := mustReview(t, off, baseReceipt("evt-off-1", StatusCompleted, true, rev))
	if d.Recorded || d.Drivable || len(off.Receipts()) != 0 {
		t.Fatalf("off mode must not record a drivable receipt: %+v", d)
	}

	assist := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: rev})
	mustReview(t, assist, baseReceipt("evt-assist-1", StatusCompleted, true, rev))
	d = mustReview(t, assist, baseReceipt("evt-assist-2", StatusCompleted, true, rev))
	if !d.Recorded || !d.Drivable || d.Obligation == nil {
		t.Fatalf("assist must raise the obligation at threshold: %+v", d)
	}
}

func TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation(t *testing.T) {
	rev := testRevision(t, "goal contract v1")
	r := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: rev})

	first := mustReview(t, r, baseReceipt("evt-c1", StatusCompleted, true, rev))
	if !first.Counted || first.Consecutive != 1 || first.Obligation != nil {
		t.Fatalf("receipt N-1 must count without raising: %+v", first)
	}
	second := mustReview(t, r, baseReceipt("evt-c2", StatusCompleted, true, rev))
	if !second.Counted || second.Consecutive != 2 || second.Obligation == nil {
		t.Fatalf("receipt N must raise: %+v", second)
	}
	if second.Obligation.Kind != ObligationKind || second.Obligation.Threshold != 2 {
		t.Fatalf("obligation = %+v", second.Obligation)
	}
	if len(second.Obligation.EvidenceIDs) != 2 {
		t.Fatalf("obligation evidence = %v, want both receipts", second.Obligation.EvidenceIDs)
	}
	third := mustReview(t, r, baseReceipt("evt-c3", StatusCompleted, true, rev))
	if third.Obligation == nil || third.Obligation.Consecutive != 3 {
		t.Fatalf("streak beyond threshold must keep the obligation: %+v", third)
	}

	fresh := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: rev})
	quiet := mustReview(t, fresh, baseReceipt("evt-q1", StatusCompleted, false, rev))
	if !quiet.Handled || quiet.Counted || quiet.Consecutive != 0 || quiet.Obligation != nil {
		t.Fatalf("completed without drift must not count: %+v", quiet)
	}
	afterQuiet := mustReview(t, fresh, baseReceipt("evt-q2", StatusCompleted, true, rev))
	if afterQuiet.Consecutive != 1 || afterQuiet.Obligation != nil {
		t.Fatalf("non-drift receipt must break the streak: %+v", afterQuiet)
	}

	three := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 3, PinnedRevision: rev})
	for i, wantConsecutive := range []int{1, 2} {
		d := mustReview(t, three, baseReceipt(fmt.Sprintf("evt-t%d", i+1), StatusCompleted, true, rev))
		if d.Consecutive != wantConsecutive || d.Obligation != nil {
			t.Fatalf("threshold 3 receipt %d = %+v", i+1, d)
		}
	}
	if d := mustReview(t, three, baseReceipt("evt-t3", StatusCompleted, true, rev)); d.Obligation == nil {
		t.Fatal("threshold 3 must raise on the third receipt")
	}
}

func TestNonCompletedStatusesNeverCountAsHandled(t *testing.T) {
	rev := testRevision(t, "goal contract v1")
	statuses := []Status{
		StatusAbstained, StatusFailed, StatusNotEvaluated, StatusStale,
		StatusUnknown, Status("bogus"), Status(""),
	}
	for _, status := range statuses {
		label := string(status)
		if label == "" {
			label = "empty"
		}
		r := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: rev})
		mustReview(t, r, baseReceipt("evt-seed-"+label, StatusCompleted, true, rev))
		d := mustReview(t, r, baseReceipt("evt-non-"+label, status, true, rev))
		if d.Handled || d.Counted {
			t.Fatalf("status %q counted as handled: %+v", status, d)
		}
		if d.Consecutive != 0 || r.ConsecutiveDrift() != 0 || d.Obligation != nil {
			t.Fatalf("status %q did not break the streak: decision %+v streak %d", status, d, r.ConsecutiveDrift())
		}
		next := mustReview(t, r, baseReceipt("evt-after-"+label, StatusCompleted, true, rev))
		if next.Consecutive != 1 || next.Obligation != nil {
			t.Fatalf("status %q interruption leaked into the next streak: %+v", status, next)
		}
	}

	r := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 1, PinnedRevision: rev})
	dup := baseReceipt("evt-dup", StatusCompleted, true, rev)
	mustReview(t, r, dup)
	second := mustReview(t, r, dup)
	if second.Counted || second.Consecutive != 0 {
		t.Fatalf("duplicate event counted twice: %+v", second)
	}
	if second.AssistBlockedReason != ReasonDuplicateEvent {
		t.Fatalf("duplicate reason = %q", second.AssistBlockedReason)
	}
}

func TestReceiptBindsPinnedGoalRevision(t *testing.T) {
	contract := "goal contract v1"
	rev := DeriveContractRevision(contract)
	if rev != DeriveContractRevision(contract) {
		t.Fatal("contract revision must be reproducible")
	}
	upper := strings.ToUpper(rev)
	normalized, err := NormalizeContractRevision("  " + upper + "  ")
	if err != nil || normalized != rev {
		t.Fatalf("NormalizeContractRevision(upper) = %q, %v; want %q", normalized, err, rev)
	}
	if _, err := NormalizeContractRevision("abc"); err == nil {
		t.Fatal("short revision must be rejected")
	}
	if _, err := NormalizeContractRevision(strings.Repeat("z", RevisionLength)); err == nil {
		t.Fatal("non-hex revision must be rejected")
	}

	r := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: upper})
	if r.PinnedRevision() != rev {
		t.Fatalf("pinned revision = %q, want %q", r.PinnedRevision(), rev)
	}
	for i := 1; i <= 2; i++ {
		receipt := baseReceipt(fmt.Sprintf("evt-pin-%d", i), StatusCompleted, true, upper)
		d := mustReview(t, r, receipt)
		if d.AssistBlockedReason != "" || d.RebindHint != "" {
			t.Fatalf("pinned receipt blocked: %+v", d)
		}
		if d.ContractRevision != rev {
			t.Fatalf("decision revision = %q, want %q", d.ContractRevision, rev)
		}
	}
	obligation := r.Obligation()
	if obligation == nil || obligation.ContractRevision != rev {
		t.Fatalf("obligation revision = %+v, want %q", obligation, rev)
	}
	if len(r.Receipts()) != 2 || r.Receipts()[0].ContractRevision != rev {
		t.Fatalf("recorded revisions not normalized: %+v", r.Receipts())
	}

	again := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: rev})
	for i := 1; i <= 2; i++ {
		d := mustReview(t, again, baseReceipt(fmt.Sprintf("evt-again-%d", i), StatusCompleted, true, rev))
		if !d.Counted {
			t.Fatalf("reproduced revision did not count: %+v", d)
		}
		if raise := i >= 2; raise != (d.Obligation != nil) {
			t.Fatalf("obligation at receipt %d = %+v, want raised=%v", i, d.Obligation, raise)
		}
	}
}

func TestForeignRevisionIsNotCountedAndReportsRebindHint(t *testing.T) {
	pinned := testRevision(t, "goal contract v1")
	foreign := testRevision(t, "goal contract v2")

	unpinned := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2})
	d := mustReview(t, unpinned, baseReceipt("evt-unpinned", StatusCompleted, true, pinned))
	if d.Counted || d.Consecutive != 0 || d.Obligation != nil {
		t.Fatalf("unpinned receipt counted: %+v", d)
	}
	if d.AssistBlockedReason != ReasonRevisionUnpinned {
		t.Fatalf("unpinned reason = %q, want %q", d.AssistBlockedReason, ReasonRevisionUnpinned)
	}
	if _, ok := unpinned.Summary()[SummaryKeyRebindHint]; ok {
		t.Fatalf("unpinned summary reported rebind hint: %v", unpinned.Summary())
	}

	r := mustReviewer(t, Options{Mode: ModeAssist, Threshold: 2, PinnedRevision: pinned})
	foreignDecision := mustReview(t, r, baseReceipt("evt-foreign", StatusCompleted, true, foreign))
	if foreignDecision.Counted || foreignDecision.Consecutive != 0 || foreignDecision.Obligation != nil {
		t.Fatalf("foreign receipt counted: %+v", foreignDecision)
	}
	if foreignDecision.RebindHint != RebindHintNewerUnderUnpinned {
		t.Fatalf("decision rebind hint = %q", foreignDecision.RebindHint)
	}
	if got := r.Summary()[SummaryKeyRebindHint]; got != RebindHintNewerUnderUnpinned {
		t.Fatalf("summary rebind hint = %v", got)
	}
	if foreignDecision.AssistBlockedReason != ReasonRevisionMismatch {
		t.Fatalf("foreign reason = %q", foreignDecision.AssistBlockedReason)
	}

	matching := mustReview(t, r, baseReceipt("evt-matching", StatusCompleted, true, pinned))
	if matching.Consecutive != 1 || matching.Obligation != nil {
		t.Fatalf("foreign receipt leaked into the streak: %+v", matching)
	}

	blank := mustReview(t, r, baseReceipt("evt-blank", StatusCompleted, true, ""))
	if blank.AssistBlockedReason != ReasonRevisionUnpinned || blank.Counted {
		t.Fatalf("unbound receipt = %+v", blank)
	}

	shadow := mustReviewer(t, Options{Mode: ModeShadow, PinnedRevision: pinned})
	sd := mustReview(t, shadow, baseReceipt("evt-shadow-foreign", StatusCompleted, true, foreign))
	if sd.RebindHint != RebindHintNewerUnderUnpinned || sd.Drivable || sd.Counted || sd.Obligation != nil {
		t.Fatalf("shadow foreign receipt = %+v", sd)
	}
}
