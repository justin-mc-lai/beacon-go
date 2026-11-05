// Lifecycle writes are monotonic over completion. Release/Completed
// is a terminal fact: creating a new run, freezing/refreezing the
// package, or ticking a new run may upgrade a feature TO it, but must
// never move a released feature back to Prd/InProgress, Freeze, or
// Blocked. Explicit `truth reopen` deliberately starts a new revision
// and keeps its own snapshot write (a new revision is a new intent).
package orchestrator

import (
	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// updateLifecycleMonotonic writes one lifecycle snapshot unless it
// would downgrade a feature already at Release/Completed. It works
// inside or outside a transaction because it only needs the
// FeatureDao handle.
func updateLifecycleMonotonic(f *store.FeatureDao, l store.Lifecycle) error {
	if cur, err := f.GetLifecycle(l.FeatureID); err == nil {
		released := cur.Stage == core.StageRelease && cur.Status == core.StatusCompleted
		writingRelease := l.Stage == core.StageRelease && l.Status == core.StatusCompleted
		if released && !writingRelease {
			return nil
		}
	}
	return f.UpdateLifecycle(l)
}
