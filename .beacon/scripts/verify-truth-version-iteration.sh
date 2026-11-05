#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
go test ./internal/orchestrator -run 'TestDocsVersion|TestTruthCreateGreenfield|TestTruthCreateUsesCurrent' -count=1
go test ./internal/cli -run 'TestProjectsBumpPatch' -count=1
go test ./skills -run 'TestTruthSkillVersionFlow' -count=1
