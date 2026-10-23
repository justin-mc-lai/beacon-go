// QA strategy surface (R2d, parity audit M2): persists the
// qa9 strategy (internal/gate/qastrategy.go) into the project
// so the rest of the kernel can consume it. The file is
// project configuration, not requirement truth: qa9.py's
// consumers (auto_test / gate / qa / test_case) read the
// strategy in-process in Python; here the autotest command
// discovery chain reads the persisted default_command when
// neither tests.md nor .beacon/config names a command.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"

	"beacon-go/internal/gate"
)

// QAStrategyRelPath is the persisted strategy location,
// project-relative.
const QAStrategyRelPath = ".beacon/qa-strategy.json"

// GenerateQAStrategy builds the strategy for one project root
// and (unless write=false) persists it to QAStrategyRelPath.
func GenerateQAStrategy(projectRoot string, qaTypes []string, uiRunner string, write bool) (gate.QAStrategy, string, error) {
	strategy := gate.BuildQAStrategy(projectRoot, qaTypes, uiRunner, nowISO())
	if !write {
		return strategy, "", nil
	}
	raw, err := json.MarshalIndent(strategy, "", "  ")
	if err != nil {
		return gate.QAStrategy{}, "", err
	}
	abs := filepath.Join(projectRoot, QAStrategyRelPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return gate.QAStrategy{}, "", err
	}
	if err := os.WriteFile(abs, append(raw, '\n'), 0o644); err != nil {
		return gate.QAStrategy{}, "", err
	}
	return strategy, QAStrategyRelPath, nil
}

// qaStrategyDefaultCommand reads the persisted strategy's
// default_command ("" when no strategy was generated). It is
// the last resort of the autotest discovery chain — a strategy
// the project generated for itself outranks guessing, and
// nothing outranks an explicit tests.md row or .beacon/config.
func qaStrategyDefaultCommand(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, QAStrategyRelPath))
	if err != nil {
		return ""
	}
	var strategy gate.QAStrategy
	if err := json.Unmarshal(raw, &strategy); err != nil {
		return ""
	}
	return strategy.DefaultCommand
}
