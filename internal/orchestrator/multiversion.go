// Multi-version plan (P2d, G-14): plan_multi_version /
// multi_version_intent.build_multi_version_execution_plan, recast
// from "parse an utterance into dual-track lakes" to the explicit
// segment form the task fixes: one goal cut into per-version
// segments, each carrying its lake/run items and its dependencies
// on earlier segments.
//
// The check the source family leaves implicit is explicit here:
// an item that a segment leaves unfinished must be carried into
// the next segment's items or recorded as dropped — work may not
// silently vanish between versions (multi_version_carry_missing).
// Version order must be strictly increasing (numeric dotted
// compare, optional leading "v"); the last segment may not leave
// unfinished items behind either — they have nowhere to carry to.
package orchestrator

import (
	"fmt"
	"strconv"
	"strings"

	"beacon-go/internal/core"
)

// parseVersion splits a dotted numeric version (optional leading v)
// into comparable parts. ok=false for anything else.
func parseVersion(v string) ([]int, bool) {
	s := strings.TrimPrefix(strings.TrimSpace(v), "v")
	if s == "" {
		return nil, false
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// compareVersions orders two parsed versions (-1/0/+1).
func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func strSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range items {
		out[s] = true
	}
	return out
}

// PlanMultiVersion validates and records the segmentation on the
// run (metadata + multi_version_planned event). Structural defects
// (no segments, unparsable or non-monotonic versions, dependencies
// on unknown/later segments) refuse the plan outright; carry gaps
// are recorded IN the plan as problems (the plan exists, marked
// invalid, so the gap is visible state — the same treatment
// parallel plans give touch conflicts).
func (e *Engine) PlanMultiVersion(runID string, in MultiVersionInput) (MultiVersionPlan, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return MultiVersionPlan{}, err
	}
	if len(in.Segments) == 0 {
		return MultiVersionPlan{}, core.NewError(core.ReasonMultiVersionSegmentsEmpty,
			"goal.PlanMultiVersion", "a multi-version plan needs at least one segment")
	}
	parsed := make([][]int, len(in.Segments))
	versions := map[string]bool{}
	for i, seg := range in.Segments {
		p, ok := parseVersion(seg.Version)
		if !ok {
			return MultiVersionPlan{}, core.NewError(core.ReasonMultiVersionInvalid,
				"goal.PlanMultiVersion", "segment version is not a dotted numeric version: "+seg.Version)
		}
		parsed[i] = p
		if versions[seg.Version] {
			return MultiVersionPlan{}, core.NewError(core.ReasonMultiVersionNotMonotonic,
				"goal.PlanMultiVersion", "duplicate segment version "+seg.Version)
		}
		versions[seg.Version] = true
		if i > 0 && compareVersions(parsed[i-1], p) >= 0 {
			return MultiVersionPlan{}, core.NewError(core.ReasonMultiVersionNotMonotonic,
				"goal.PlanMultiVersion", "segment versions must strictly increase: "+
					in.Segments[i-1].Version+" then "+seg.Version)
		}
	}
	indexOf := map[string]int{}
	for i, seg := range in.Segments {
		indexOf[seg.Version] = i
	}
	for i, seg := range in.Segments {
		for _, dep := range seg.DependsOn {
			j, known := indexOf[dep]
			if !known || j >= i {
				return MultiVersionPlan{}, core.NewError(core.ReasonMultiVersionDepInvalid,
					"goal.PlanMultiVersion", fmt.Sprintf(
						"segment %s depends on %s, which is not an earlier segment", seg.Version, dep))
			}
		}
	}
	plan := MultiVersionPlan{Schema: "multi-version-plan.v1", RunID: runID, CreatedAt: nowRFC(), Valid: true}
	for i, seg := range in.Segments {
		out := VersionSegment{
			Version: seg.Version, Items: seg.Items, DependsOn: seg.DependsOn,
			Completed: in.Completed[seg.Version], Dropped: in.Dropped[seg.Version],
		}
		if i > 0 {
			prev := in.Segments[i-1]
			prevDone := strSet(in.Completed[prev.Version])
			prevDrop := strSet(in.Dropped[prev.Version])
			mine := strSet(seg.Items)
			for _, item := range prev.Items {
				if mine[item] && !prevDone[item] && !prevDrop[item] {
					out.CarriedIn = append(out.CarriedIn, item)
				}
			}
		}
		plan.Segments = append(plan.Segments, out)
	}
	for i, seg := range in.Segments {
		done := strSet(in.Completed[seg.Version])
		drop := strSet(in.Dropped[seg.Version])
		var next map[string]bool
		if i+1 < len(in.Segments) {
			next = strSet(in.Segments[i+1].Items)
		}
		for _, item := range seg.Items {
			if done[item] || drop[item] {
				continue
			}
			if next == nil || !next[item] {
				plan.Problems = append(plan.Problems, MVProblem{
					ReasonCode: core.ReasonMultiVersionCarryMissing,
					Detail: fmt.Sprintf("segment %s leaves %q unfinished: not completed, not dropped, not carried into the next segment",
						seg.Version, item),
				})
			}
		}
	}
	plan.Valid = len(plan.Problems) == 0
	m.MultiVersion = &plan
	codes := []string{}
	for _, p := range plan.Problems {
		codes = append(codes, p.ReasonCode)
	}
	if err := e.appendEvent(run, "multi_version_planned", map[string]any{
		"segments": versionsInOrder(in.Segments), "valid": plan.Valid, "problem_codes": codes,
	}); err != nil {
		return MultiVersionPlan{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return MultiVersionPlan{}, err
	}
	return plan, nil
}

func versionsInOrder(segs []VersionSegmentInput) []string {
	out := []string{}
	for _, s := range segs {
		out = append(out, s.Version)
	}
	return out
}
