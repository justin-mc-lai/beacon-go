#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
go test ./internal/orchestrator -run 'TestUpdateLifecycleMonotonic|TestCreateRunDoesNotDowngradeCompletedFeature|TestFreezeDoesNotDowngradeCompletedFeature|TestLifecycleGuardIsShared' -count=1
