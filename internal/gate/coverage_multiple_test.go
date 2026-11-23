package gate

import "testing"

func TestCoverageMatrixPreservesMultipleTestTables(t *testing.T) {
	doc := "| AC ID | TC ID | Layer | Command | Assertion | Evidence |\n|---|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | go test ./x | works | evidence/a |\n\n## More\n| AC ID | TC ID | Layer | Command | Assertion | Evidence |\n|---|---|---|---|---|---|\n| AC-002 | TC-002 | negative | go test ./y | refuses | evidence/b |\n\n## Index\n| AC ID | Criterion |\n|---|---|\n| AC-001 | behavior |\n"
	rows, _, ok := ParseCoverageMatrix(doc)
	if !ok || len(rows) != 2 || rows[0].AC != "AC-001" || rows[1].AC != "AC-002" {
		t.Fatalf("coverage lost across tables: %+v", rows)
	}
}
