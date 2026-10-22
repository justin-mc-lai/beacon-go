package orchestrator

import (
	"testing"

	"beacon-go/internal/core"
)

// FIFO: two discoveries are consumed oldest-first; each consumption
// starts a formal goal run carrying the queue provenance; a third
// consumption is a normal empty result, not an error.
func TestQueueFIFOConsumeCreatesRuns(t *testing.T) {
	env := newTestEnv(t, "active")
	first, depth, err := env.eng.EnqueueDiscovery(EnqueueOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", Utterance: "first discovery",
	})
	if err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if depth != 1 || first.Status != QueueQueued || first.ProductComplete || first.MayMarkProductComplete {
		t.Errorf("first item = %+v depth %d", first, depth)
	}
	second, depth, err := env.eng.EnqueueDiscovery(EnqueueOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", Utterance: "second discovery",
	})
	if err != nil || depth != 2 {
		t.Fatalf("enqueue 2: %+v depth %d", second, depth)
	}
	// FIFO order in the list surface.
	items, err := env.eng.ListQueue(QueueQueued)
	if err != nil || len(items) != 2 || items[0].ItemID != first.ItemID || items[1].ItemID != second.ItemID {
		t.Fatalf("queue list = %+v %v", items, err)
	}
	res, err := env.eng.ConsumeNext("")
	if err != nil {
		t.Fatalf("consume 1: %v", err)
	}
	if res.Status != "consumed" || res.Item == nil || res.Item.ItemID != first.ItemID {
		t.Fatalf("consume 1 = %+v", res)
	}
	if res.Run == nil || res.Run.FromQueueItem != first.ItemID {
		t.Errorf("consumed run = %+v", res.Run)
	}
	if res.Item.Status != QueueConsumed || res.Item.ConsumedRunID != res.Run.RunID || res.QueueDepth != 1 {
		t.Errorf("consumed item = %+v depth %d", res.Item, res.QueueDepth)
	}
	res, err = env.eng.ConsumeNext("")
	if err != nil || res.Item == nil || res.Item.ItemID != second.ItemID {
		t.Fatalf("consume 2 = %+v %v", res, err)
	}
	res, err = env.eng.ConsumeNext("")
	if err != nil {
		t.Fatalf("consume empty must not error: %v", err)
	}
	if res.Status != "empty" || res.ReasonCode != core.ReasonGoalQueueEmpty || res.Item != nil {
		t.Errorf("empty consume = %+v", res)
	}
}

// Anti-impersonation: a queue item can never be completed (the
// Complete path rejects it with the source's hard-guard code), never
// driven, and the explicit reject helper carries the same code.
func TestQueueItemCannotComplete(t *testing.T) {
	env := newTestEnv(t, "active")
	item, _, err := env.eng.EnqueueDiscovery(EnqueueOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", Utterance: "sneaky discovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Complete(item.ItemID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "lai"}); err == nil {
		t.Fatal("completing a queue item must be rejected")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonLoopCannotComplete {
		t.Errorf("reason = %q want %q", rc, core.ReasonLoopCannotComplete)
	}
	if _, err := env.eng.DriverStart(item.ItemID, DriverStartOptions{}); err == nil {
		t.Error("driving a queue item must be rejected")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonNotAGoalRun {
		t.Errorf("driver-on-queue reason = %q", rc)
	}
	if err := RejectQueueComplete("beacon-loop-goal"); err == nil {
		t.Error("RejectQueueComplete must always error")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonLoopCannotComplete {
		t.Errorf("reject reason = %q", rc)
	}
	// The item is still queued afterwards (rejection consumed nothing).
	depth, err := env.eng.QueueDepth(QueueQueued)
	if err != nil || depth != 1 {
		t.Errorf("depth after rejected complete = %d %v", depth, err)
	}
}
