package intenttree

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// TreeDir is the repo-side canonical location for one version line.
func TreeDir(projectRoot, docsRoot string) string {
	return filepath.Join(projectRoot, docsRoot, "tree")
}

func IndexPath(dir string) string { return filepath.Join(dir, "index.json") }
func NodePath(dir, id string) string {
	return filepath.Join(dir, "nodes", id+".json")
}

// atomicWrite writes a temp file in the same directory and renames it.
// A failed render never leaves a half-written artifact.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tree-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func prettyJSON(v any) ([]byte, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// WriteNode writes one canonical node body.
func WriteNode(dir string, n Node) error {
	raw, err := prettyJSON(n)
	if err != nil {
		return err
	}
	return atomicWrite(NodePath(dir, n.ID), raw)
}

// ReadNode reads one node body and verifies its fingerprint.
func ReadNode(dir, id string) (Node, error) {
	raw, err := os.ReadFile(NodePath(dir, id))
	if err != nil {
		return Node{}, err
	}
	var n Node
	if err := json.Unmarshal(raw, &n); err != nil {
		return n, err
	}
	if n.ID != id {
		return n, core.NewError(ReasonInvalidIndex, "intenttree.ReadNode", "node id mismatch: "+id)
	}
	ok, err := VerifyFingerprint(n)
	if err != nil {
		return n, err
	}
	if !ok {
		return n, core.NewError(ReasonInvalidIndex, "intenttree.ReadNode", "fingerprint drift on "+id)
	}
	return n, nil
}

// WriteIndex writes the aggregate review index last, so readers never
// see an index referring to node files that are not written yet.
func WriteIndex(dir string, idx Index) error {
	raw, err := prettyJSON(SortedIndex(idx))
	if err != nil {
		return err
	}
	return atomicWrite(IndexPath(dir), raw)
}

// ReadIndex parses the aggregate view without reading node bodies.
func ReadIndex(dir string) (Index, error) {
	raw, err := os.ReadFile(IndexPath(dir))
	if err != nil {
		return Index{}, err
	}
	var idx Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return idx, err
	}
	if idx.Schema != Schema {
		return idx, core.NewError(ReasonSchemaUnknown, "intenttree.ReadIndex", "unknown tree schema "+idx.Schema)
	}
	return idx, nil
}

// HasIndex reports whether a canonical tree index exists on disk.
func HasIndex(dir string) bool {
	_, err := os.Stat(IndexPath(dir))
	return err == nil
}

// SourceHash binds a rendered view to the index it came from.
func SourceHash(idx Index) string {
	raw, err := json.Marshal(SortedIndex(idx))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum)
}

// RenderMarkdown renders a deterministic, reviewable outline from the
// index only. It never reads node bodies.
func RenderMarkdown(idx Index) []byte {
	idx = SortedIndex(idx)
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Requirement tree: %s\n\n", idx.Version)
	fmt.Fprintf(&b, "Schema: `%s` · root: `%s` · nodes: %d\n", idx.Schema, idx.RootID, len(idx.Nodes))
	fmt.Fprintf(&b, "Source hash: `%s`\n\n", SourceHash(idx))
	b.WriteString("| id | kind | status | title | fingerprint |\n|---|---|---|---|---|\n")
	for _, n := range idx.Nodes {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | `%s` |\n",
			n.ID, n.Kind, n.Status, strings.ReplaceAll(n.Title, "|", "\\|"), n.Fingerprint[:min(12, len(n.Fingerprint))])
	}
	b.WriteString("\n## Outline\n\n")
	var walk func(string, int)
	walk = func(parent string, depth int) {
		levels := children[parent]
		sort.Slice(levels, func(i, j int) bool { return levels[i].Path < levels[j].Path })
		for _, n := range levels {
			fmt.Fprintf(&b, "%s- [%s] %s (%s/%s)\n", strings.Repeat("  ", depth), n.ID, n.Title, n.Kind, n.Status)
			walk(n.ID, depth+1)
		}
	}
	walk("", 0)
	return []byte(b.String())
}

// RenderHTML renders a self-contained static HTML view from the index.
// It is a view, never a truth source, and is written outside the repo.
func RenderHTML(idx Index) []byte {
	idx = SortedIndex(idx)
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<!-- source: %s -->\n", SourceHash(idx))
	b.WriteString("<title>Requirement tree</title><style>\n")
	b.WriteString("body{font:14px/1.6 sans-serif;margin:32px;max-width:1100px}details{margin-left:18px}")
	b.WriteString("summary{cursor:pointer;font-weight:600}code{font-size:12px;color:#555}\n")
	b.WriteString("</style></head><body>\n")
	fmt.Fprintf(&b, "<h1>Requirement tree · %s</h1><p><code>%s</code> · %d nodes</p>\n",
		html.EscapeString(idx.Version), html.EscapeString(idx.Schema), len(idx.Nodes))
	var walk func(string)
	walk = func(parent string) {
		levels := children[parent]
		sort.Slice(levels, func(i, j int) bool { return levels[i].Path < levels[j].Path })
		for _, n := range levels {
			fmt.Fprintf(&b, "<details open><summary>%s <code>%s</code> <code>%s/%s</code></summary>\n",
				html.EscapeString(n.Title), html.EscapeString(n.ID), n.Kind, n.Status)
			if len(children[n.ID]) > 0 {
				walk(n.ID)
			}
			b.WriteString("</details>\n")
		}
	}
	walk("")
	b.WriteString("</body></html>\n")
	return []byte(b.String())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
