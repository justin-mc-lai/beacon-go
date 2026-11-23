package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSDDDecomposeArtifactHardlinkPreservesTasks(t *testing.T) {
	root := t.TempDir()
	tasks := filepath.Join(root, "tasks.md")
	artifact := filepath.Join(root, "draft.json")
	if err := os.WriteFile(tasks, []byte("authored DAG"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(tasks, artifact); err != nil {
		t.Fatal(err)
	}
	if err := writeDecompositionArtifact(artifact, map[string]string{"candidate": "suggestion"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != "authored DAG" {
		t.Fatal("artifact write truncated hard-linked task content")
	}
}
