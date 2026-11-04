#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
go test ./skills ./internal/cli -run 'TestPluginManifest|TestVerify|TestSkillsVerify' -count=1
