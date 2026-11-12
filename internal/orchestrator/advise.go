// Advise ranks a project's features by deterministic facts only. It never
// reads prose, never calls a model, and never invents a reason: every field
// on an item is a copy of a spine snapshot fact.
package orchestrator

import (
	"sort"
	"strings"

	"beacon-go/internal/store"
)

const (
	AdviseSchema  = "goal-advise.v1"
	AdviseVersion = "advise-v1"
)

// AdviseItem is one ranked feature. SuggestedNext is a stage pointer, not a
// sentence: the wording layer lives outside the kernel on purpose.
type AdviseItem struct {
	Feature        string   `json:"feature"`
	Project        string   `json:"project"`
	Score          int      `json:"score"`
	ReasonCodes    []string `json:"reason_codes"`
	SuggestedNext  string   `json:"suggested_next"`
	RunID          string   `json:"run_id,omitempty"`
	LifecycleStage string   `json:"lifecycle_stage"`
	LifecycleState string   `json:"lifecycle_status"`
	ProgressStatus string   `json:"progress_status"`
	RunStatus      string   `json:"run_status"`
	Blockers       []string `json:"blockers"`
	UpdatedAt      string   `json:"updated_at"`
}

type AdviseDoc struct {
	Schema  string       `json:"schema"`
	Version string       `json:"version"`
	Project string       `json:"project"`
	Items   []AdviseItem `json:"items"`
}

func hardBlockers(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if strings.HasPrefix(c, "migration_") || c == "" {
			continue
		}
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func suggestedNext(s store.SpineSnapshot) string {
	switch strings.ToLower(string(s.Stage)) {
	case "plan":
		return "plan"
	case "truth":
		return "truth"
	case "implement":
		return "implement"
	case "qa":
		return "qa"
	case "release":
		return "release"
	}
	if strings.EqualFold(s.ProgressStatus, "done") {
		return "archive"
	}
	return "review"
}

func adviseScore(s store.SpineSnapshot, hard []string) int {
	score := 0
	if len(hard) > 0 {
		score += 100
	}
	if strings.EqualFold(s.ProgressStatus, "in_progress") {
		score += 40
	}
	switch strings.ToLower(string(s.Stage)) {
	case "implement", "qa":
		score += 20
	}
	if strings.EqualFold(string(s.RunStatus), "Paused") {
		score += 10
	}
	return score
}

// AdviseProject ranks the snapshots of one project. Ties break on feature
// slug so two runs over the same facts produce the same order.
func AdviseProject(snaps []store.SpineSnapshot, projectKey string, top int) AdviseDoc {
	items := make([]AdviseItem, 0, len(snaps))
	for _, s := range snaps {
		if projectKey != "" && s.ProjectKey != projectKey {
			continue
		}
		hard := hardBlockers(s.ReasonCodes)
		blockers := s.ReasonCodes
		if blockers == nil {
			blockers = []string{}
		}
		items = append(items, AdviseItem{
			Feature:        s.FeatureSlug,
			Project:        s.ProjectKey,
			Score:          adviseScore(s, hard),
			ReasonCodes:    hard,
			SuggestedNext:  suggestedNext(s),
			RunID:          s.RunID,
			LifecycleStage: string(s.Stage),
			LifecycleState: string(s.Status),
			ProgressStatus: s.ProgressStatus,
			RunStatus:      string(s.RunStatus),
			Blockers:       blockers,
			UpdatedAt:      s.UpdatedAt,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].Feature < items[j].Feature
	})
	if top > 0 && len(items) > top {
		items = items[:top]
	}
	return AdviseDoc{Schema: AdviseSchema, Version: AdviseVersion, Project: projectKey, Items: items}
}
