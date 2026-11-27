package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureReplayUsesRawBlobsAndOverlaysOnlyTests(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	for name, body := range map[string]string{".gitattributes": "config.txt export-ignore\nsource.txt export-subst\n", "config.txt": "needed", "source.txt": "$Format:%H$\n", "behavior_test.go": "old test"} {
		writeFile(t, filepath.Join(root, name), body)
	}
	git("init")
	git("add", ".")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "baseline")
	base := git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(root, "behavior_test.go"), "new test")
	writeFile(t, filepath.Join(root, "source.txt"), "current business source")
	git("add", ".")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "test update")
	dir, _, cleanup, err := prepareCaptureReplay(context.Background(), root, base, []string{"behavior_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for name, want := range map[string]string{"config.txt": "needed", "source.txt": "$Format:%H$\n", "behavior_test.go": "new test"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(b) != want {
			t.Fatalf("raw tree %s = %q err=%v", name, b, err)
		}
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("snapshot not cleaned")
	}
}
