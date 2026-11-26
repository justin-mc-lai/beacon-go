package version

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime/debug"
	"strings"
)

// RuntimeInfo describes the binary that is rendering or mutating a tree.
// Requirement truth and runtime release are different axes; this struct
// keeps the runtime side explicit.
type RuntimeInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BinarySHA string `json:"binary_sha256,omitempty"`
}

// Runtime returns the build identity. Commit/BinarySHA are best-effort:
// release builds may omit VCS metadata, and tests may not have a stable
// executable path.
func Runtime() RuntimeInfo {
	out := RuntimeInfo{Version: Version}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				out.Commit = strings.TrimSpace(s.Value)
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		if raw, err := os.ReadFile(exe); err == nil {
			sum := sha256.Sum256(raw)
			out.BinarySHA = hex.EncodeToString(sum[:])
		}
	}
	return out
}
