package intenttree

import (
	"fmt"
	"html"
	"strings"
)

// renderFSMSVG draws a real flow diagram: states on layers, legal edges
// solid, illegal edges dashed red. No external renderer is involved.
func renderFSMSVG(f *DomainFSM, key string) string {
	if f == nil || len(f.States) == 0 {
		return ""
	}
	marker := "arrow-" + key
	markerRed := "arrow-red-" + key
	states := f.States
	index := map[string]int{}
	for i, s := range states {
		index[s] = i
	}
	level := map[string]int{}
	visited := map[string]bool{}
	for _, s := range states {
		level[s] = 0
	}
	// roots: no legal incoming edge
	in := map[string]int{}
	for _, tr := range f.Transitions {
		if _, ok := index[tr.From]; !ok {
			continue
		}
		if _, ok := index[tr.To]; !ok {
			continue
		}
		in[tr.To]++
	}
	var queue []string
	for _, s := range states {
		if in[s] == 0 {
			queue = append(queue, s)
			visited[s] = true
		}
	}
	if len(queue) == 0 {
		queue = append(queue, states[0])
		visited[states[0]] = true
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, tr := range f.Transitions {
			if tr.From != cur {
				continue
			}
			if _, ok := index[tr.To]; !ok || visited[tr.To] {
				continue
			}
			level[tr.To] = level[cur] + 1
			visited[tr.To] = true
			queue = append(queue, tr.To)
		}
	}
	maxLevel := 0
	byLevel := map[int][]string{}
	for _, s := range states {
		l := level[s]
		if l > maxLevel {
			maxLevel = l
		}
		byLevel[l] = append(byLevel[l], s)
	}
	maxCount := 1
	for _, list := range byLevel {
		if len(list) > maxCount {
			maxCount = len(list)
		}
	}
	nodeW, nodeH := 100.0, 40.0
	spacing := 118.0
	width := 300.0
	if float64(maxCount)*spacing+30 > width {
		width = float64(maxCount)*spacing + 30
	}
	height := 60 + float64(maxLevel+1)*105
	pos := map[string][2]float64{}
	for l := 0; l <= maxLevel; l++ {
		list := byLevel[l]
		for i, s := range list {
			x := (width - float64(len(list)-1)*spacing) / 2
			if len(list) > 1 {
				x += float64(i) * spacing
			}
			y := 48.0 + float64(l)*105
			pos[s] = [2]float64{x, y}
		}
	}
	var b strings.Builder
	// Default view: every forward transition (the normal lifecycle).
	// Back/cross edges and illegal transitions are the exception group.
	forward := map[string]bool{}
	for _, tr := range f.Transitions {
		if level[tr.To] > level[tr.From] {
			forward[tr.From+"->"+tr.To] = true
		}
	}
	drawEdge := func(tr FSMTransition, legal bool) string {
		s, ok1 := pos[tr.From]
		d, ok2 := pos[tr.To]
		if !ok1 || !ok2 || tr.From == tr.To {
			return ""
		}
		stroke, mk, dash := "#6b7280", marker, ""
		if !legal {
			stroke, mk, dash = "#b23b3b", markerRed, ` stroke-dasharray="5 4"`
		}
		x1, y1 := s[0], s[1]+nodeH/2
		x2, y2 := d[0], d[1]-nodeH/2
		path := ""
		if d[1] <= s[1] || !legal {
			side := width - 18
			path = fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f", x1, y1, side, y1+20, side, y2-20, x2, y2)
		} else {
			midY := (y1 + y2) / 2
			path = fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f", x1, y1, x1, midY, x2, midY, x2, y2)
		}
		var out strings.Builder
		fmt.Fprintf(&out, `<path d="%s" fill="none" stroke="%s" stroke-width="1.6"%s marker-end="url(#%s)"/>`, path, stroke, dash, mk)
		return out.String()
	}
	var defs, primary, exceptions strings.Builder
	fmt.Fprintf(&defs, `<defs><marker id="%s" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#6b7280"/></marker><marker id="%s" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#b23b3b"/></marker></defs>`, marker, markerRed)
	for _, tr := range f.Transitions {
		if forward[tr.From+"->"+tr.To] {
			primary.WriteString(drawEdge(tr, true))
		} else {
			exceptions.WriteString(drawEdge(tr, true))
		}
	}
	for _, tr := range f.Illegal {
		exceptions.WriteString(drawEdge(tr, false))
	}
	fmt.Fprintf(&b, `<div class="fsm-canvas" style="width:%.0fpx;height:%.0fpx">`, width, height)
	fmt.Fprintf(&b, `<svg class="edge-layer" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="业务状态机">`, width, height)
	b.WriteString(defs.String())
	b.WriteString(primary.String())
	b.WriteString(`<g class="exceptions">`)
	b.WriteString(exceptions.String())
	b.WriteString(`</g></svg>`)
	// State labels are HTML, not SVG text: readable in every browser,
	// selectable and searchable.
	for _, st := range states {
		p := pos[st]
		cls := "state"
		if in[st] == 0 {
			cls += " start"
		}
		fmt.Fprintf(&b, `<div class="%s" style="left:%.1fpx;top:%.1fpx">%s</div>`, cls, p[0]-nodeW/2, p[1]-nodeH/2, esc(st))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func statusLabel(s Status) string {
	switch s {
	case StatusDraft:
		return "草稿"
	case StatusFrozen:
		return "已冻结"
	case StatusImplemented:
		return "已交付/实现"
	case StatusVerified:
		return "已验证"
	case StatusChangePending:
		return "变更中"
	case StatusDeprecated:
		return "已废弃"
	case Status("blocked"):
		return "阻塞"
	}
	return string(s)
}

func testStatusLabel(s string) string {
	if s == "existing" {
		return "已绑定"
	}
	return "计划中"
}

func joinIllegal(in []FSMTransition) string {
	parts := make([]string, 0, len(in))
	for _, tr := range in {
		parts = append(parts, tr.From+"→"+tr.To)
	}
	return strings.Join(parts, "，")
}

func esc(s string) string { return html.EscapeString(s) }
