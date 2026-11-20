// Continuous drive must stop on a deadline/cancel and return a stable
// reason code instead of spinning forever (program-integrated-delivery
// review: "连续驱动超时" — root cause program_drive.go unbounded loop).
package orchestrator

import (
	"context"
	"testing"
	"time"

	"beacon-go/internal/core"
)

func TestProgramDriveUntilDoneTimeoutBounded(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "p", Freeze: true,
		Lakes: []ProgramLakeInput{{Slug: "feat-a"}},
	}); err != nil {
		t.Fatal(err)
	}

	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	start := time.Now()
	v, err := env.eng.ProgramDrive(host.RunID, ProgramDriveOptions{
		Executor: "recording", UntilDone: true, Context: expired,
	})
	if err != nil {
		t.Fatalf("drive returned error, want a bounded view: %v", err)
	}
	if v.ReasonCode != core.ReasonProgramDriveTimeout {
		t.Fatalf("reason = %q, want %s", v.ReasonCode, core.ReasonProgramDriveTimeout)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("drive did not return promptly on an expired deadline: %s", elapsed)
	}

	canceled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	v, err = env.eng.ProgramDrive(host.RunID, ProgramDriveOptions{
		Executor: "recording", UntilDone: true, Context: canceled,
	})
	if err != nil {
		t.Fatalf("canceled drive returned error: %v", err)
	}
	if v.ReasonCode != core.ReasonHostInterrupted {
		t.Fatalf("cancel reason = %q, want %s", v.ReasonCode, core.ReasonHostInterrupted)
	}
}
