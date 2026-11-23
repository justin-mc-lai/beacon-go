package gate

import "testing"

func TestParseCoverageMatrixLastTableWins(t *testing.T) {
	doc := `## Acceptance Criteria

| AC ID | Criterion |
|---|---|
| AC-001 | criterion |

## Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|---|---|---|---|---|
| AC-001 | TC-001 | behavior | go test ./x -count=1 | ok |
| AC-001 | TC-002 | negative | go test ./x -count=1 | refused |
`
	rows, _, ok := ParseCoverageMatrix(doc)
	if !ok {
		t.Fatal("matrix not found")
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (the AC table must not leak in): %+v", len(rows), rows)
	}
	if rows[0].Layer != "behavior" || rows[1].Layer != "negative" {
		t.Fatalf("layer not parsed: %+v", rows)
	}
}
