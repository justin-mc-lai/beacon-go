package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These guards check contract distribution only, never claim agent compliance.
func TestStrictDeliveryReferencesSurviveInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range IDs() {
		s, ok := Get(id)
		if !ok {
			t.Fatalf("missing skill %s", id)
		}
		ref := "../beacon-go-verify/references/strict-delivery.md"
		if id == "verify" {
			ref = "references/strict-delivery.md"
		}
		if !strings.Contains(s.Body, "`"+ref+"`") {
			t.Errorf("%s lacks mandatory strict delivery pointer", id)
		}
		installed, err := os.ReadFile(filepath.Join(dir, InstallDirName(id), ref))
		if err != nil {
			t.Fatal(err)
		}
		embedded, err := files.ReadFile("beacon-go-verify/references/strict-delivery.md")
		if err != nil || string(installed) != string(embedded) {
			t.Errorf("%s installed contract differs from embedded source: %v", id, err)
		}
	}
}

func TestStrictDeliverySixControls(t *testing.T) {
	raw, err := files.ReadFile("beacon-go-verify/references/strict-delivery.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"## 1. Plan-First", "## 2. Red-Green", "## 3. Blind Review",
		"## 4. Evidence", "## 5. Ledger", "## 6. Deterministic Tooling",
		"环境故障、零测试", "保持已确认断言不变", "不传实现者推理或自评",
		"退出码非零", "不能产生第二账本", "不编造 flag", "文本检查通过不代表宿主已执行",
	} {
		if !strings.Contains(string(raw), required) {
			t.Errorf("strict delivery contract missing %q", required)
		}
	}
}
