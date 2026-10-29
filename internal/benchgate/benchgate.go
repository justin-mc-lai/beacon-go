// Package benchgate is the P6 performance regression gate (AC-095):
// compare a benchmark result document with its baseline and fail when any
// shared acceptance criterion regressed by more than the allowed
// percentage.
//
// Comparison is only meaningful inside one environment. The gate
// therefore refuses to judge across a different OS or CPU count instead
// of producing a confident number from two different machines; the
// caller sees `comparable=false` and the reasons.
package benchgate

import (
	"encoding/json"
	"fmt"
	"sort"
)

// DefaultTolerancePct is the frozen AC-095 threshold.
const DefaultTolerancePct = 10.0

// Document is the shape run_bench.sh writes (only the fields the gate
// needs; unknown fields are ignored).
type Document struct {
	GeneratedAt string         `json:"generated_at"`
	Environment map[string]any `json:"environment"`
	Results     []Item         `json:"results"`
}

// Item is one benchmark line.
type Item struct {
	AC          string   `json:"ac"`
	Name        string   `json:"name"`
	P95MS       *float64 `json:"p95_ms"`
	ThresholdMS float64  `json:"threshold_ms"`
}

// Row is one comparison.
type Row struct {
	AC             string  `json:"ac"`
	Name           string  `json:"name"`
	BaselineP95MS  float64 `json:"baseline_p95_ms"`
	CandidateP95MS float64 `json:"candidate_p95_ms"`
	DeltaPct       float64 `json:"delta_pct"`
	TolerancePct   float64 `json:"tolerance_pct"`
	Regressed      bool    `json:"regressed"`
}

// Report is the gate's answer.
type Report struct {
	Schema       string   `json:"schema"`
	Comparable   bool     `json:"comparable"`
	Note         string   `json:"note,omitempty"`
	TolerancePct float64  `json:"tolerance_pct"`
	Rows         []Row    `json:"rows"`
	Regressions  []string `json:"regressions,omitempty"`
	Failed       bool     `json:"failed"`
}

// Compare computes the regression report.
func Compare(baselineRaw, candidateRaw []byte, tolerancePct float64) (Report, error) {
	if tolerancePct <= 0 {
		tolerancePct = DefaultTolerancePct
	}
	rep := Report{Schema: "beacon-bench-gate.v1", TolerancePct: tolerancePct, Rows: []Row{}}
	var base, cand Document
	if err := json.Unmarshal(baselineRaw, &base); err != nil {
		return rep, fmt.Errorf("baseline is not a benchmark document: %w", err)
	}
	if err := json.Unmarshal(candidateRaw, &cand); err != nil {
		return rep, fmt.Errorf("candidate is not a benchmark document: %w", err)
	}
	// Environment comparability: the same OS and the same CPU count.
	for _, key := range []string{"os", "nproc"} {
		bv := envString(base.Environment, key)
		cv := envString(cand.Environment, key)
		if bv == "" || cv == "" {
			rep.Note = "baseline and candidate must both declare environment." + key
			return rep, nil
		}
		if bv != cv {
			rep.Note = fmt.Sprintf("not comparable: environment.%s baseline=%s candidate=%s", key, bv, cv)
			return rep, nil
		}
	}
	rep.Comparable = true
	baseByAC := map[string]Item{}
	for _, it := range base.Results {
		baseByAC[it.AC] = it
	}
	type pair struct{ b, c Item }
	var pairs []pair
	for _, c := range cand.Results {
		b, ok := baseByAC[c.AC]
		if !ok || b.P95MS == nil || c.P95MS == nil || *b.P95MS <= 0 {
			continue
		}
		pairs = append(pairs, pair{b: b, c: c})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].c.AC < pairs[j].c.AC })
	for _, p := range pairs {
		delta := (*p.c.P95MS - *p.b.P95MS) / *p.b.P95MS * 100
		row := Row{
			AC: p.c.AC, Name: p.c.Name,
			BaselineP95MS: round2(*p.b.P95MS), CandidateP95MS: round2(*p.c.P95MS),
			DeltaPct: round2(delta), TolerancePct: tolerancePct,
			Regressed: delta > tolerancePct,
		}
		rep.Rows = append(rep.Rows, row)
		if row.Regressed {
			rep.Regressions = append(rep.Regressions, row.AC)
		}
	}
	if len(rep.Rows) == 0 {
		rep.Note = "no shared acceptance criteria between baseline and candidate"
		return rep, nil
	}
	rep.Failed = len(rep.Regressions) > 0
	return rep, nil
}

func envString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
