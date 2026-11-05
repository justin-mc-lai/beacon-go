#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
go test ./internal/orchestrator -run 'TestProgramDriveUntilDone|TestProgramComplete' -count=1
