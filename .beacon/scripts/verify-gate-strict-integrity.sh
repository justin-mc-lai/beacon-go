#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
go test ./internal/cli -run 'TestGateIntegrityStrictBlocksFrozenPackage|TestGateIntegrityFrozenDefaultStaysAudit' -count=1
go test ./internal/orchestrator -run 'TestReleaseCandidateIntegrityBlocks|TestHistoricalFrozenIntegrityAudits' -count=1
go test ./internal/gate -run 'TestEvaluateReleaseHasNoIntegrityPrereq' -count=1
