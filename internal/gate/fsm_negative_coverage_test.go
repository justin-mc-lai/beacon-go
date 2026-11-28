package gate

import (
	"strings"
	"testing"
)

const negativeCoverageTruth = `## Domain Model
| Entity | Key | Precedence |
|---|---|---|
| Order | id | first |
## Domain FSM - Order
| State | From | Guard |
|---|---|---|
| pending | - | none |
| paid | pending | confirmed |
## Illegal Transitions
| To | Aggregate | From |
|---|---|---|
| pending | Order | paid |
| paid | Order | cancelled |
## Acceptance Criteria
| AC ID | Criterion |
|---|---|
| AC-001 | Order rejects illegal moves |
`

func negativeCoverageTests(rows string) string {
	return "| AC ID | TC ID | Layer | Command | Assertion | Aggregate | From | To |\n|---|---|---|---|---|---|---|---|\n" + rows
}

func TestFSMNegativeCoveragePerSlot(t *testing.T) {
	tests := negativeCoverageTests("| AC-001 | TC-001 | negative | go test ./x | rejected | Order | paid | pending |\n")
	_, findings := EvaluateDomainFSM(negativeCoverageTruth, tests)
	var uncovered []DomainFSMFinding
	for _, f := range findings {
		if f.ReasonCode == "uncovered" {
			uncovered = append(uncovered, f)
		}
	}
	if len(uncovered) != 1 || !strings.Contains(uncovered[0].Text, "cancelled") || !strings.Contains(uncovered[0].Text, "paid") || uncovered[0].Aggregate != "Order" {
		t.Fatalf("want exactly Order cancelled -> paid uncovered, got %+v", findings)
	}
	tests += "| AC-001 | TC-002 | negative | go test ./x | rejected | Order | cancelled | paid |\n"
	_, findings = EvaluateDomainFSM(negativeCoverageTruth, tests)
	if len(findings) != 0 {
		t.Fatalf("all slots covered must pass: %+v", findings)
	}
}

func TestFSMNegativeCoverageRejectsNonExecutableSideMatrix(t *testing.T) {
	main := negativeCoverageTests("| AC-001 | TC-001 | behavior | go test ./x | works | Order | paid | pending |\n")
	side := strings.ReplaceAll(negativeCoverageTests("| AC-001 | TC-002 | negative | go test ./x | rejected | Order | paid | pending |\n"), "AC ID", "AC_ID")
	side = strings.ReplaceAll(side, "TC ID", "TC_ID")
	doc := main + "\n## Side matrix\n" + side
	rows := ParseCoverageRows(doc)
	if len(rows) != 1 || rows[0].TC != "TC-001" {
		t.Fatalf("formal parser accepted side matrix: %+v", rows)
	}
	truth := strings.Replace(negativeCoverageTruth, "| paid | Order | cancelled |\n", "", 1)
	_, fs := EvaluateDomainFSM(truth, doc)
	for _, f := range fs {
		if f.ReasonCode == "uncovered" {
			return
		}
	}
	t.Fatalf("non-executable side matrix cleared uncovered: %+v", fs)
}

func TestFSMNegativeCoverageMalformedDataDoesNotTruncateRequiredTCs(t *testing.T) {
	first := "| AC-001 | TC-001 | behavior | go test ./x | works | Order | paid | pending |\n"
	last := "| AC-001 | TC-002 | negative | go test ./x | rejected | Order | paid | pending |\n"
	for _, bad := range []string{
		"| | TC-099 | negative | go test ./x | malformed | Order | paid | pending |\n",
		"| invalid | TC-099 | negative | go test ./x | malformed | Order | paid | pending |\n",
	} {
		t.Run(strings.Fields(bad)[1], func(t *testing.T) {
			doc := negativeCoverageTests(first + bad + last)
			rows := ParseCoverageRows(doc)
			if len(rows) != 2 || rows[0].TC != "TC-001" || rows[1].TC != "TC-002" || rows[1].AC != "AC-001" {
				t.Fatalf("malformed data shrank required TC set: %+v", rows)
			}
			truth := strings.Replace(negativeCoverageTruth, "| paid | Order | cancelled |\n", "", 1)
			_, fs := EvaluateDomainFSM(truth, doc)
			for _, f := range fs {
				if f.ReasonCode == "uncovered" {
					t.Fatalf("valid later negative TC lost: %+v", fs)
				}
			}
		})
	}
}

func TestFSMNegativeCoverageAdjacentSideHeaderEndsMatrix(t *testing.T) {
	main := negativeCoverageTests("| AC-001 | TC-001 | behavior | go test ./x | works | Order | paid | pending |\n")
	side := strings.ReplaceAll(negativeCoverageTests("| AC-001 | TC-002 | negative | go test ./x | rejected | Order | paid | pending |\n"), "AC ID", "AC_ID")
	side = strings.ReplaceAll(side, "TC ID", "TC_ID")
	rows := ParseCoverageRows(main + side)
	if len(rows) != 1 || rows[0].TC != "TC-001" {
		t.Fatalf("adjacent side header inherited formal matrix: %+v", rows)
	}
	truth := strings.Replace(negativeCoverageTruth, "| paid | Order | cancelled |\n", "", 1)
	_, fs := EvaluateDomainFSM(truth, main+side)
	for _, f := range fs {
		if f.ReasonCode == "uncovered" {
			return
		}
	}
	t.Fatalf("side matrix cleared uncovered: %+v", fs)
}

func TestFSMNegativeCoverageSharesFormalRows(t *testing.T) {
	row := "| AC-001 | TC-001 | negative | go test ./x | rejected | Order | `paid` | `pending` |\n"
	for _, tc := range []struct {
		name, doc string
		count     int
		duplicate bool
	}{
		{"normalized binding", negativeCoverageTests(row), 1, false},
		{"fenced", "```md\n" + negativeCoverageTests(row) + "```\n", 0, false},
		{"duplicate header", strings.Replace(negativeCoverageTests(row), "| Assertion |", "| From |", 1), 0, false},
		{"duplicate identity", negativeCoverageTests(row + row), 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := ParseCoverageRows(tc.doc)
			if len(rows) != tc.count {
				t.Fatalf("rows=%+v", rows)
			}
			for _, r := range rows {
				if r.From != "paid" || r.To != "pending" || r.Aggregate != "Order" || r.DuplicateTC != tc.duplicate {
					t.Fatalf("not normalized: %+v", r)
				}
			}
		})
	}
}

func TestFSMNegativeCoverageDegenerate(t *testing.T) {
	row := "| AC-001 | TC-001 | negative | go test ./x | rejected | Order | paid | pending |\n"
	truth := strings.Replace(negativeCoverageTruth, "| paid | Order | cancelled |\n", "", 1)
	for _, tc := range []struct {
		name, tests string
		want        int
	}{
		{"empty", "", 1}, {"whitespace", " \n\t", 1},
		{"keyword", "illegal transitions covered by TC-001", 1},
		{"fenced example", "```md\n" + negativeCoverageTests(row) + "```\n", 1},
		{"missing ac", negativeCoverageTests(strings.Replace(row, "AC-001", "", 1)), 1},
		{"positive", negativeCoverageTests(strings.Replace(row, "negative", "behavior", 1)), 1},
		{"reverse", negativeCoverageTests(strings.Replace(row, "| paid | pending |", "| pending | paid |", 1)), 1},
		{"blank from", negativeCoverageTests(strings.Replace(row, "| paid | pending |", "|  | pending |", 1)), 1},
		{"wrong aggregate", negativeCoverageTests(strings.Replace(row, "| Order |", "| Invoice |", 1)), 1},
		{"duplicate tc", negativeCoverageTests(row + row), 1},
		{"duplicate column", strings.Replace(negativeCoverageTests(row), "| Assertion |", "| From |", 1), 1},
		{"normalized", negativeCoverageTests(strings.Replace(row, "| paid | pending |", "| `paid` | `pending` |", 1)), 0},
		{"no execution needed", negativeCoverageTests(row), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, fs := EvaluateDomainFSM(truth, tc.tests)
			n := 0
			for _, f := range fs {
				if f.ReasonCode == "uncovered" {
					n++
				}
			}
			if n != tc.want {
				t.Fatalf("uncovered=%d want=%d findings=%+v", n, tc.want, fs)
			}
		})
	}
	_, fs := EvaluateDomainFSM("## Acceptance Criteria\n- AC-001 stateless", "")
	if len(fs) != 0 {
		t.Fatalf("absent domain must remain inapplicable: %+v", fs)
	}
}

func TestFSMNegativeCoverageProseAndMultipleAggregates(t *testing.T) {
	truth := negativeCoverageTruth + "\n## Domain FSM - Invoice\n| State | From | Guard |\n|---|---|---|\n| pending | paid | none |\n### Illegal Transitions\n- illegal: paid -> pending\n- Never bypass approval, even when paid -> pending appears in an explanation\n"
	refs := negativeCoverageTests("| AC-001 | TC-001 | negative | go test ./x | rejected | Order | paid | pending |\n| AC-001 | TC-002 | negative | go test ./x | rejected | Order | cancelled | paid |\n")
	_, fs := EvaluateDomainFSM(truth, refs)
	n := 0
	for _, f := range fs {
		if f.ReasonCode == "uncovered" {
			n++
			if f.Aggregate != "Invoice" {
				t.Fatalf("wrong scope: %+v", f)
			}
		}
	}
	if n != 2 {
		t.Fatalf("want two Invoice gaps, got %+v", fs)
	}
	refs += "| AC-001 | TC-003 | negative | go test ./x | rejected | Invoice | paid | pending |\n"
	refs += "\n| AC ID | TC ID | Layer | Illegal Slot | Aggregate |\n|---|---|---|---|---|\n| AC-001 | TC-004 | negative | Never bypass approval, even when paid -> pending appears in an explanation | Invoice |\n"
	_, fs = EvaluateDomainFSM(truth, refs)
	for _, f := range fs {
		if f.ReasonCode == "uncovered" {
			t.Fatalf("precise references must cover prose: %+v", fs)
		}
	}
}
