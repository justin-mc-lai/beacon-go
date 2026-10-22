// Entry contract (G-01) + resume pack (G-08) — the two P2 debts
// closed in P4a. Both are read-only aggregations over state that
// already exists; neither creates new state of its own.
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// ---- Entry contract (G-01) ----

// EntryContractView is one run's public entry contract: the stable
// public surface of the goal ability (single entry, no dual
// orchestration — the shape of v1's public_entry_contract) joined
// with THIS run's stage plan and per-stage skill lists.
type EntryContractView struct {
	SchemaVersion           string           `json:"schema_version"`
	RunID                   string           `json:"run_id"`
	FeatureSlug             string           `json:"feature_slug"`
	PublicPrimary           string           `json:"public_primary"`
	PublicCLI               string           `json:"public_cli"`
	SingleEntry             bool             `json:"single_entry"`
	DualPublicOrchestration bool             `json:"dual_public_orchestration"`
	Status                  string           `json:"status"`
	LoopStage               string           `json:"loop_stage"`
	CurrentStage            string           `json:"current_stage,omitempty"`
	Utterance               string           `json:"utterance,omitempty"`
	Stages                  []StagePlanEntry `json:"stages"`
	NextSkill               string           `json:"next_skill,omitempty"`
	VerifierRequired        bool             `json:"verifier_required"`
	ReleaseRequiresHuman    bool             `json:"release_requires_human"`
}

// EntryContract returns the run's entry contract (read-only).
func (e *Engine) EntryContract(runID string) (EntryContractView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return EntryContractView{}, err
	}
	v := viewOf(run, m, slug)
	stages := m.StagePlan
	if stages == nil {
		stages = []StagePlanEntry{}
	}
	verifierRequired := true
	if m.Verifier != nil {
		verifierRequired = m.Verifier.RequiredForComplete
	}
	return EntryContractView{
		SchemaVersion:           "beacon-goal-entry-contract/v1",
		RunID:                   run.ID,
		FeatureSlug:             slug,
		PublicPrimary:           "goal",
		PublicCLI:               "beacon goal",
		SingleEntry:             true,
		DualPublicOrchestration: false,
		Status:                  v.Status,
		LoopStage:               m.LoopStage,
		CurrentStage:            m.currentStage(),
		Utterance:               m.Utterance,
		Stages:                  stages,
		NextSkill:               v.NextSkill,
		VerifierRequired:        verifierRequired,
		ReleaseRequiresHuman:    true,
	}, nil
}

// ---- Resume pack (G-08) ----

// CheckpointView is the latest checkpoint with its state decoded.
type CheckpointView struct {
	ID        string         `json:"id"`
	Iteration int            `json:"iteration"`
	CreatedAt string         `json:"created_at"`
	State     map[string]any `json:"state,omitempty"`
}

// ResumePackView is the full recovery pack (goal_resume_pack): the
// run snapshot, the resume decision (the same decideResume Resume
// itself applies — computed here WITHOUT consuming the interrupt
// or clearing the block), the latest checkpoint, and the open
// blocker / pending steering summaries a fresh session needs to
// pick the run up without re-reading the whole trail.
type ResumePackView struct {
	Run              RunView         `json:"run"`
	Resume           ResumeView      `json:"resume"`
	LatestCheckpoint *CheckpointView `json:"latest_checkpoint,omitempty"`
	OpenBlockers     []string        `json:"open_blockers"`
	BlockedStreak    int             `json:"blocked_streak"`
	PendingSteering  *SteeringRecord `json:"pending_steering,omitempty"`
}

// ResumePack assembles the recovery pack (read-only: unlike Resume
// it consumes nothing and clears nothing).
func (e *Engine) ResumePack(runID string) (ResumePackView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return ResumePackView{}, err
	}
	pack := ResumePackView{
		Run:             viewOf(run, m, slug),
		Resume:          decideResume(runID, m),
		OpenBlockers:    []string{},
		BlockedStreak:   m.BlockedStreak,
		PendingSteering: m.pendingSteering(),
	}
	seen := map[string]bool{}
	addBlocker := func(code string) {
		if code != "" && !seen[code] {
			seen[code] = true
			pack.OpenBlockers = append(pack.OpenBlockers, code)
		}
	}
	if m.Blocked {
		addBlocker(m.LastBlockReason)
	}
	ckpt, err := e.store.Runs().LatestCheckpoint(runID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ResumePackView{}, err
	}
	if err == nil {
		cv := &CheckpointView{ID: ckpt.ID, Iteration: ckpt.Iteration, CreatedAt: ckpt.CreatedAt}
		if len(ckpt.State) > 0 {
			var st map[string]any
			if json.Unmarshal(ckpt.State, &st) == nil {
				cv.State = st
				if obs, ok := st["open_blockers"].([]any); ok {
					for _, ob := range obs {
						if s, ok := ob.(string); ok {
							addBlocker(s)
						}
					}
				}
			}
		}
		pack.LatestCheckpoint = cv
	}
	return pack, nil
}
