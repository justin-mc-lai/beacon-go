// Discovery queue (P2b, G-09): the goal_queue.py handoff, re-based on
// the frozen nine tables. Storage decision (argued in the p2b report):
// queue items are rows in `runs` with run_kind="discovery_queue" —
// the frozen schema has no queue table and adding one would break the
// AC-067..075 contract, while runs already gives FIFO ordering
// (started_at, rowid), an append-only event trail, and the feature
// anchor runs.feature_id requires. The queue state proper
// (queued/consumed) lives in the item row's metadata_json; runs.status
// mirrors it coarsely (Created while queued, Closed once consumed).
//
// Anti-impersonation, both of the source's guards:
//  1. every item permanently carries product_complete=false and
//     may_mark_product_complete=false (assert_not_product_complete_
//     authority's projection);
//  2. a queue item can never be completed: Engine.Complete rejects
//     queue rows with loop_cannot_mark_product_complete, and the CLI
//     `goal queue mark-complete` is an explicit always-rejected surface
//     (the loop_goal.py mark-complete port).
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// RunKindDiscoveryQueue marks runs rows that are discovery queue
// items, not goal runs.
const RunKindDiscoveryQueue = "discovery_queue"

// Queue statuses (item metadata).
const (
	QueueQueued   = "queued"
	QueueConsumed = "consumed"
)

// queueItemMeta is the item snapshot in the queue row's metadata_json.
type queueItemMeta struct {
	Kind                   string `json:"kind"` // "discovery_queue_item"
	Utterance              string `json:"utterance"`
	FeatureSlug            string `json:"feature_slug,omitempty"`
	ProjectRoot            string `json:"project_root,omitempty"`
	Source                 string `json:"source"`
	Priority               string `json:"priority"`
	QueueStatus            string `json:"queue_status"`
	ConsumedAt             string `json:"consumed_at,omitempty"`
	ConsumedBy             string `json:"consumed_by,omitempty"`
	ConsumedRunID          string `json:"consumed_run_id,omitempty"`
	ProductComplete        bool   `json:"product_complete"`          // always false
	MayMarkProductComplete bool   `json:"may_mark_product_complete"` // always false
}

// QueueItemView is the stable JSON surface of one queue item.
type QueueItemView struct {
	ItemID                 string `json:"item_id"`
	Utterance              string `json:"utterance"`
	FeatureSlug            string `json:"feature_slug,omitempty"`
	FeatureID              string `json:"feature_id"`
	Source                 string `json:"source"`
	Priority               string `json:"priority"`
	Status                 string `json:"status"` // queued | consumed
	CreatedAt              string `json:"created_at"`
	ConsumedAt             string `json:"consumed_at,omitempty"`
	ConsumedBy             string `json:"consumed_by,omitempty"`
	ConsumedRunID          string `json:"consumed_run_id,omitempty"`
	ProductComplete        bool   `json:"product_complete"`
	MayMarkProductComplete bool   `json:"may_mark_product_complete"`
}

func queueItemViewOf(row store.Run, qm *queueItemMeta) QueueItemView {
	return QueueItemView{
		ItemID: row.ID, Utterance: qm.Utterance, FeatureSlug: qm.FeatureSlug,
		FeatureID: row.FeatureID, Source: qm.Source, Priority: qm.Priority,
		Status: qm.QueueStatus, CreatedAt: row.StartedAt,
		ConsumedAt: qm.ConsumedAt, ConsumedBy: qm.ConsumedBy, ConsumedRunID: qm.ConsumedRunID,
		ProductComplete: false, MayMarkProductComplete: false,
	}
}

func parseQueueMeta(row store.Run) (*queueItemMeta, bool) {
	qm := &queueItemMeta{}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, qm); err != nil {
			return nil, false
		}
	}
	return qm, qm.Kind == "discovery_queue_item"
}

// EnqueueOptions carries one discovery enqueue.
type EnqueueOptions struct {
	ProjectRoot string
	Version     string
	FeatureSlug string
	Utterance   string
	Source      string // default "beacon-loop-goal" (the discover surface)
	Priority    string // default "normal"
}

// EnqueueDiscovery writes one queued discovery item for later goal
// consumption (enqueue_discovery). The item anchors to a resolved
// feature because runs.feature_id is NOT NULL in the frozen schema —
// the resolution errors are the same as CreateRun's.
func (e *Engine) EnqueueDiscovery(opts EnqueueOptions) (QueueItemView, int, error) {
	root := store.ResolveRootPath(opts.ProjectRoot)
	ctx, err := e.store.ResolveFeature(root, opts.FeatureSlug, opts.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if opts.FeatureSlug != "" {
				return QueueItemView{}, 0, core.NewError(ReasonFeatureNotFound, "goal.EnqueueDiscovery",
					"feature "+opts.FeatureSlug+" not found under "+root)
			}
			return QueueItemView{}, 0, core.NewError(ReasonProjectNotRegistered, "goal.EnqueueDiscovery",
				"no features registered under project root "+root)
		}
		return QueueItemView{}, 0, err
	}
	source := opts.Source
	if source == "" {
		source = "beacon-loop-goal"
	}
	priority := opts.Priority
	if priority == "" {
		priority = "normal"
	}
	qm := &queueItemMeta{
		Kind: "discovery_queue_item", Utterance: opts.Utterance,
		FeatureSlug: ctx.Feature.Slug, ProjectRoot: root,
		Source: source, Priority: priority, QueueStatus: QueueQueued,
		ProductComplete: false, MayMarkProductComplete: false,
	}
	raw, err := json.Marshal(qm)
	if err != nil {
		return QueueItemView{}, 0, err
	}
	row, err := e.store.Runs().CreateRun(store.Run{
		FeatureID: ctx.Feature.ID, RunKind: RunKindDiscoveryQueue,
		Status: core.RunCreated, Metadata: raw,
	})
	if err != nil {
		return QueueItemView{}, 0, err
	}
	_ = e.appendEvent(row, "discovery_enqueued", map[string]any{
		"utterance": opts.Utterance, "source": source, "priority": priority,
		"feature_slug": ctx.Feature.Slug,
	})
	depth, err := e.QueueDepth(QueueQueued)
	if err != nil {
		return QueueItemView{}, 0, err
	}
	return queueItemViewOf(row, qm), depth, nil
}

// ListQueue returns queue items, oldest first, optionally filtered by
// status ("" = all).
func (e *Engine) ListQueue(status string) ([]QueueItemView, error) {
	rows, err := e.store.Runs().ListByKind(RunKindDiscoveryQueue)
	if err != nil {
		return nil, err
	}
	out := []QueueItemView{}
	for _, row := range rows {
		qm, ok := parseQueueMeta(row)
		if !ok {
			continue
		}
		if status != "" && qm.QueueStatus != status {
			continue
		}
		out = append(out, queueItemViewOf(row, qm))
	}
	return out, nil
}

// QueueDepth counts queue items in one status ("" = all).
func (e *Engine) QueueDepth(status string) (int, error) {
	items, err := e.ListQueue(status)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// ConsumeResult is the outcome of one queue consumption.
type ConsumeResult struct {
	Status     string         `json:"status"` // consumed | empty
	ReasonCode string         `json:"reason_code,omitempty"`
	Item       *QueueItemView `json:"item,omitempty"`
	Run        *RunView       `json:"run,omitempty"`
	QueueDepth int            `json:"queue_depth"` // remaining queued
}

// ConsumeNext takes the oldest queued item (FIFO) and starts a formal
// goal run from it (consume_discovery_queue: consume + run facade).
// An empty queue is a normal result (status=empty,
// reason_code=goal_queue_empty), never an error.
func (e *Engine) ConsumeNext(consumer string) (ConsumeResult, error) {
	if consumer == "" {
		consumer = "beacon-goal"
	}
	rows, err := e.store.Runs().ListByKind(RunKindDiscoveryQueue)
	if err != nil {
		return ConsumeResult{}, err
	}
	var target *store.Run
	var qm *queueItemMeta
	for i := range rows {
		if parsed, ok := parseQueueMeta(rows[i]); ok && parsed.QueueStatus == QueueQueued {
			target = &rows[i]
			qm = parsed
			break
		}
	}
	depth, err := e.QueueDepth(QueueQueued)
	if err != nil {
		return ConsumeResult{}, err
	}
	if target == nil {
		return ConsumeResult{Status: "empty", ReasonCode: core.ReasonGoalQueueEmpty, QueueDepth: depth}, nil
	}
	// Recover the project root from the item's feature anchor:
	// feature -> version -> project.
	feat, err := e.store.Features().GetByID(target.FeatureID)
	if err != nil {
		return ConsumeResult{}, err
	}
	ver, err := e.store.Projects().GetVersionByID(feat.ProjectVersionID)
	if err != nil {
		return ConsumeResult{}, err
	}
	proj, err := e.store.Projects().GetByID(ver.ProjectID)
	if err != nil {
		return ConsumeResult{}, err
	}
	runView, err := e.CreateRun(CreateRunOptions{
		ProjectRoot: proj.RootPath, FeatureSlug: feat.Slug,
		Utterance: qm.Utterance, ExecutorID: consumer,
	})
	if err != nil {
		return ConsumeResult{}, err
	}
	// Record queue provenance on the new run.
	run, m, slug, err := e.loadRun(runView.RunID)
	if err != nil {
		return ConsumeResult{}, err
	}
	m.FromQueueItem = target.ID
	m.DiscoverySource = qm.Source
	if err := e.appendEvent(run, "goal_run_from_discovery", map[string]any{
		"queue_item_id": target.ID, "source": qm.Source,
	}); err != nil {
		return ConsumeResult{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ConsumeResult{}, err
	}
	runView = viewOf(run, m, slug)
	runView.FromQueueItem = target.ID
	// Mark the item consumed.
	qm.QueueStatus = QueueConsumed
	qm.ConsumedAt = nowISO()
	qm.ConsumedBy = consumer
	qm.ConsumedRunID = runView.RunID
	raw, err := json.Marshal(qm)
	if err != nil {
		return ConsumeResult{}, err
	}
	if err := e.store.Runs().UpdateRunState(target.ID, core.RunClosed, raw, nil); err != nil {
		return ConsumeResult{}, err
	}
	_ = e.appendEvent(*target, "discovery_consumed", map[string]any{
		"consumed_by": consumer, "consumed_run_id": runView.RunID,
	})
	item := queueItemViewOf(*target, qm)
	depth, err = e.QueueDepth(QueueQueued)
	if err != nil {
		return ConsumeResult{}, err
	}
	return ConsumeResult{Status: "consumed", Item: &item, Run: &runView, QueueDepth: depth}, nil
}

// RejectQueueComplete is the explicit anti-impersonation guard
// (reject_loop_product_complete): queue / loop surfaces can never mark
// product work complete; delivery completion belongs to Engine.Complete
// on a real goal run.
func RejectQueueComplete(attemptedBy string) error {
	if attemptedBy == "" {
		attemptedBy = "beacon-loop-goal"
	}
	return core.NewError(core.ReasonLoopCannotComplete, "goal.queue.mark-complete",
		"loop/queue surfaces are support discovery only and cannot mark product complete; use `beacon goal complete` on a goal run (attempted_by="+attemptedBy+")")
}
