package benchgate

import (
	"encoding/json"
	"testing"
)

func doc(os, nproc string, items ...Item) []byte {
	b, _ := json.Marshal(Document{
		GeneratedAt: "2026-01-01T00:00:00Z",
		Environment: map[string]any{"os": os, "nproc": nproc, "binary_version": "v2"},
		Results:     items,
	})
	return b
}

func p95(v float64) *float64 { return &v }

func TestWithinTolerancePasses(t *testing.T) {
	base := doc("linux", "2", Item{AC: "AC-051", Name: "cold start", P95MS: p95(100), ThresholdMS: 200})
	cand := doc("linux", "2", Item{AC: "AC-051", Name: "cold start", P95MS: p95(109.9), ThresholdMS: 200})
	rep, err := Compare(base, cand, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Comparable || rep.Failed {
		t.Fatalf("109.9 vs 100 must pass: %+v", rep)
	}
}

func TestOverToleranceFails(t *testing.T) {
	base := doc("linux", "2", Item{AC: "AC-053", Name: "truth show", P95MS: p95(10), ThresholdMS: 200})
	cand := doc("linux", "2", Item{AC: "AC-053", Name: "truth show", P95MS: p95(11.1), ThresholdMS: 200})
	rep, err := Compare(base, cand, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Comparable || !rep.Failed || len(rep.Regressions) != 1 || rep.Regressions[0] != "AC-053" {
		t.Fatalf("11.1 vs 10 must fail at 10%%: %+v", rep)
	}
}

func TestCrossEnvironmentIsNotJudged(t *testing.T) {
	base := doc("linux", "2", Item{AC: "AC-051", P95MS: p95(1), ThresholdMS: 100})
	cand := doc("linux", "8", Item{AC: "AC-051", P95MS: p95(1000), ThresholdMS: 100})
	rep, err := Compare(base, cand, 10)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Comparable || rep.Failed {
		t.Fatalf("different CPU counts must not be judged: %+v", rep)
	}
	if rep.Note == "" {
		t.Error("non-comparable report must say why")
	}
}

func TestMissingEnvironmentFailsClosed(t *testing.T) {
	base := []byte(`{"environment":{"os":"linux"},"results":[]}`)
	cand := []byte(`{"environment":{"os":"linux"},"results":[]}`)
	rep, err := Compare(base, cand, 10)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Comparable {
		t.Fatal("a document without nproc must not be treated as comparable")
	}
}
