// Package orchestrator is the Go counterpart of the frozen v2 design's
// beacon-orchestrator crate: unified goal/run/team long-task orchestration
// with checkpoint/resume/stop semantics (AC-040..050, AC-105..108).
//
// P2 scope (complete at P2d): the goal engine and its whole
// workflow set — core engine (P2a: create / tick / continue / block
// / attach-verifier / complete / heartbeat / checkpoint / resume /
// trace), long-run care (P2b: driver / supervisor / queue /
// self-heal / guardrails), delegation & parallel (P2c: lanes /
// delegate-ralph / parallel-plan / design-stage / admission) and
// orchestration extensions (P2d: program / hotfix / multi-version
// / memory / axis). Stage execution stays behind the StageExecutor
// interface. P3a added formal gate evaluation write-back; P3c added
// the REAL integrations behind the interface (subprocess / profile
// executors, the ralph loop, git worktrees, planner review — see
// exec_*.go, worktree.go, planner_review.go).
//
// The orchestrator must never emit QA-pass / release-ready verdicts
// (AC-049); it only READS gate_results for exit decisions. Release
// completion requires human confirmation (core release gate).
package orchestrator
