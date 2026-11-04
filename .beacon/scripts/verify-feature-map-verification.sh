#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
go test ./internal/orchestrator -run 'TestBuildFeatureMapBindsACs' -count=1
