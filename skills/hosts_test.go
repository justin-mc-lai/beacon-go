package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostDirResolvesWorkBuddy(t *testing.T) {
	home := t.TempDir()
	dir, err := HostDir("workbuddy", home)
	if err != nil {
		t.Fatalf("HostDir: %v", err)
	}
	want := filepath.Join(home, ".workbuddy-ai", "skills")
	if dir != want {
		t.Fatalf("workbuddy dir = %q, want %q", dir, want)
	}
	// The previous default name must keep working.
	if alias, err := HostDir("generic", home); err != nil {
		t.Fatalf("generic alias: %v", err)
	} else if alias != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("generic alias = %q", alias)
	}
	if _, err := HostDir("nope", home); err == nil {
		t.Fatal("unknown host must fail")
	}
}

func TestDetectHostPrefersWorkBuddy(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".workbuddy-ai", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	h, err := DetectHost(home)
	if err != nil {
		t.Fatalf("DetectHost: %v", err)
	}
	if h.Name != "workbuddy" {
		t.Fatalf("detected %q, want workbuddy (table order)", h.Name)
	}
}

func TestDetectHostFailsClosed(t *testing.T) {
	_, err := DetectHost(t.TempDir())
	if err == nil {
		t.Fatal("no existing host dir must be an error, not a silent default")
	}
	if !strings.Contains(err.Error(), "Candidates") {
		t.Fatalf("error should list candidates: %v", err)
	}
}

func TestHostDirsReportsInstalled(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".workbuddy-ai", "skills")
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("install: %v", err)
	}
	hosts, err := HostDirs(home)
	if err != nil {
		t.Fatalf("HostDirs: %v", err)
	}
	if len(hosts) != len(HostNames()) {
		t.Fatalf("got %d hosts, want %d", len(hosts), len(HostNames()))
	}
	if hosts[0].Name != "workbuddy" || !hosts[0].Exists {
		t.Fatalf("workbuddy host = %+v", hosts[0])
	}
	if len(hosts[0].Installed) != len(IDs()) {
		t.Fatalf("installed = %v, want %d beacon-* dirs", hosts[0].Installed, len(IDs()))
	}
	if hosts[1].Exists {
		t.Fatalf("claude host should not exist in a temp home: %+v", hosts[1])
	}
}
