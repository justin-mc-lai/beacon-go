package gate

import "testing"

// A go test -run alternation carries pipes inside the code span. They
// are command content, not table separators: the Evidence column must
// stay aligned so the conformance coverage check reads the real path.
func TestParseCoverageMatrixKeepsPipesInCodeSpan(t *testing.T) {
	tests := "---\nslug: x\n---\n" +
		"| AC ID | TC ID | Layer | Command | Assertion | Evidence |\n" +
		"|-------|-------|-------|---------|-----------|----------|\n" +
		"| AC-005 | TC-010 | regression | `go test ./internal/cli -run 'A|B|C' -count=1` | holds | evidence/out.txt |\n"
	rows, hasEvidence, ok := ParseCoverageMatrix(tests)
	if !ok || !hasEvidence || len(rows) != 1 {
		t.Fatalf("matrix = %+v hasEvidence=%v ok=%v", rows, hasEvidence, ok)
	}
	row := rows[0]
	if row.AC != "AC-005" || row.TC != "TC-010" {
		t.Fatalf("row ids = %q/%q", row.AC, row.TC)
	}
	if row.Command == "" || IsDocsOnlyCommand(row.Command) {
		t.Fatalf("executable command = %q", row.Command)
	}
	if row.Evidence != "evidence/out.txt" {
		t.Fatalf("evidence = %q, want evidence/out.txt", row.Evidence)
	}
	if row.Assertion != "holds" {
		t.Fatalf("assertion = %q, want holds", row.Assertion)
	}
}
