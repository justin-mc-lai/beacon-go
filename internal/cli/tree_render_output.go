package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	buildversion "beacon-go/internal/version"
)

func writeTreeRender(outPath, treeDir, defaultName string, data []byte) (string, error) {
	p := outPath
	if p == "" {
		if treeDir != "" {
			p = filepath.Join(treeDir, defaultName)
		} else {
			p = defaultName
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return p, nil
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

// printTreeSummary is the human default: what the project is, how much is
// covered, and where the readable view is. Machine data stays behind --json.
func printTreeSummary(cmd *cobra.Command, idx intenttree.Index, bodies map[string]intenttree.Node, dir, projRoot string) error {
	var modules, features, leaves, acs, fsm, planned, existing, linked, mismatch int
	var draft, frozen, implemented, verified, delivered, changePending, deprecated int
	var unlinked []string
	for _, e := range idx.Nodes {
		switch e.Kind {
		case intenttree.KindModule:
			modules++
		case intenttree.KindFeature:
			features++
		case intenttree.KindTouchpoint:
			leaves++
		}
		switch e.Status {
		case intenttree.StatusDraft:
			draft++
		case intenttree.StatusFrozen:
			frozen++
		case intenttree.StatusImplemented:
			implemented++
		case intenttree.StatusVerified:
			verified++
		case intenttree.StatusDelivered:
			delivered++
		case intenttree.StatusChangePending:
			changePending++
		case intenttree.StatusDeprecated:
			deprecated++
		}
		n, ok := bodies[e.ID]
		if !ok {
			continue
		}
		acs += len(n.ACs)
		if n.DomainFSM != nil {
			fsm++
		}
		for _, tc := range n.TestCases {
			if tc.Status == "existing" {
				existing++
			} else {
				planned++
			}
		}
		if n.EffectiveTruthPath() != "" {
			linked++
			st := readTruthStatus(projRoot, n)
			if !st.Match || st.Error != "" {
				mismatch++
			}
		} else if e.Kind == intenttree.KindFeature {
			unlinked = append(unlinked, filepath.Base(e.Path))
		}
	}
	root := bodies[idx.RootID]
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "需求简报 · %s\n\n", idx.Version)
	if root.Intent != "" {
		fmt.Fprintf(out, "%s\n\n", root.Intent)
	}
	rt := buildversion.Runtime()
	fmt.Fprintf(out, "版本：runtime %s · requirement %s\n", rt.Version, idx.Version)
	fmt.Fprintf(out, "%d 模块 · %d 功能 · %d 触达点 · %d 条 AC · %d 个状态机\n", modules, features, leaves, acs, fsm)
	fmt.Fprintf(out, "状态：草稿 %d · 冻结 %d · 已交付/实现 %d · 已验证 %d · 已交付 %d · 变更中 %d · 废弃 %d\n", draft, frozen, implemented, verified, delivered, changePending, deprecated)
	fmt.Fprintf(out, "测试：%d 已绑定 · %d 计划中；已验证节点 %d/%d\n", existing, planned, verified, len(idx.Nodes))
	fmt.Fprintf(out, "legacy 包链（可选）：%d/%d 个功能节点已 link，状态不一致 %d\n", linked, features, mismatch)
	if len(unlinked) > 0 {
		fmt.Fprintf(out, "tree-native: %d 个功能无 legacy 包（可选）\n", len(unlinked))
	}
	if planned > 0 {
		fmt.Fprintf(out, "\n还不能宣称已验证：%d 条测试仍是计划状态。\n", planned)
	}
	fmt.Fprintf(out, "\n阅读版：beacon truth tree --version %s --format html\n", idx.Version)
	return nil
}
