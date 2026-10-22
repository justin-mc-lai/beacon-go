// Goal memory (P2d, G-15): goal_memory_reindex / goal_memory_search
// of the durable supervisor, re-based from the v1 per-run sidecar
// FTS database onto the global store.
//
// Source shape: sqlite FTS5 over the run's execution log, explicitly
// non-authoritative ("FTS results must not complete a goal or
// replace verifier", authority=support_advisory, can_complete=false).
// The Go port keeps that contract exactly and changes only the
// storage: the index source is the run's own store trail
// (lifecycle_events + artifact_refs + checkpoints — no external
// files, no second data source), and the index itself lives in the
// run's metadata_json snapshot.
//
// Why not FTS5: modernc.org/sqlite v1.60.1 DOES support FTS5
// (measured: CREATE VIRTUAL TABLE ... USING fts5 + MATCH work on
// SQLite 3.53.4). But an FTS5 virtual table is a new schema object
// (plus shadow tables) in the frozen nine-table database, and the
// v1 alternative — a per-run sidecar DB file — violates charter
// rule 7 (state lives in the store only). The inverted index below
// is the minimal substitute; it is a derived, rebuildable cache of
// authoritative rows, so its loss would cost nothing.
//
// Structural non-authority: Engine.Complete and the admission
// checker never read MemoryIndex — there is no code path from a
// search hit to a completion decision. Every hit is stamped
// reference_only=true.
package orchestrator

import (
	"strings"
	"unicode"

	"beacon-go/internal/core"
)

// tokenize splits text into lowercase word tokens (letters/digits),
// dropping tokens shorter than 2 runes.
func tokenize(text string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) >= 2 {
			out = append(out, string(cur))
		}
		cur = cur[:0]
	}
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func capText(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// MemoryReindex rebuilds the run's inverted index from its store
// trail and stores it in the run snapshot (+ goal_memory_reindexed
// event).
func (e *Engine) MemoryReindex(runID string) (MemoryReindexView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return MemoryReindexView{}, err
	}
	var entries []MemoryEntry
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return MemoryReindexView{}, err
	}
	for _, ev := range events {
		entries = append(entries, MemoryEntry{
			SourceKind: "event", SourceID: ev.ID, RunID: runID, Label: ev.EventType,
			Text: capText(ev.EventType+" "+string(ev.Payload), 1000), CreatedAt: ev.CreatedAt,
		})
	}
	arts, err := e.store.Artifacts().ListByRun(runID)
	if err != nil {
		return MemoryReindexView{}, err
	}
	for _, a := range arts {
		entries = append(entries, MemoryEntry{
			SourceKind: "artifact", SourceID: a.ID, RunID: runID, Label: a.Kind,
			Text: capText(a.Kind+" "+a.AbsPath+" "+a.ContentHash, 1000), CreatedAt: a.CreatedAt,
		})
	}
	ckpts, err := e.store.Runs().ListCheckpoints(runID)
	if err != nil {
		return MemoryReindexView{}, err
	}
	for _, c := range ckpts {
		entries = append(entries, MemoryEntry{
			SourceKind: "checkpoint", SourceID: c.ID, RunID: runID, Label: "checkpoint",
			Text: capText("checkpoint "+string(c.State), 1000), CreatedAt: c.CreatedAt,
		})
	}
	terms := map[string][]int{}
	for i, en := range entries {
		entryTerms := map[string]bool{}
		for _, tok := range tokenize(en.Text) {
			entryTerms[tok] = true
		}
		// Marshal the payload keys in too: payload JSON keys are
		// already part of Text, so nothing extra is needed.
		for tok := range entryTerms {
			terms[tok] = append(terms[tok], i)
		}
	}
	m.Memory = &MemoryIndex{IndexedAt: nowRFC(), Entries: entries, Terms: terms}
	if err := e.appendEvent(run, "goal_memory_reindexed", map[string]any{
		"entries": len(entries), "authority": "support_advisory",
	}); err != nil {
		return MemoryReindexView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return MemoryReindexView{}, err
	}
	return MemoryReindexView{
		RunID: runID, Status: "ok", Indexed: len(entries),
		Authority: "support_advisory", ReferenceOnly: true,
		Note: "FTS-style results are reference only and must not complete a goal or replace the verifier",
	}, nil
}

// MemorySearch answers a term query from the run's stored index:
// an entry hits when it contains EVERY query term (posting-list
// intersection). Without a prior reindex the result is the source's
// "empty" shape, not an error. Hits are reference_only by
// construction.
func (e *Engine) MemorySearch(runID, query string, limit int) (MemorySearchView, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return MemorySearchView{}, err
	}
	view := MemorySearchView{
		RunID: runID, Query: query, Hits: []MemoryHit{},
		Authority: "support_advisory", ReferenceOnly: true, CanComplete: false,
	}
	if strings.TrimSpace(query) == "" {
		return view, core.NewError(core.ReasonMemoryQueryEmpty, "goal.MemorySearch", "empty query")
	}
	if m.Memory == nil || len(m.Memory.Entries) == 0 {
		view.Status = "empty"
		view.Note = "run memory reindex first; non-authoritative"
		return view, nil
	}
	if limit <= 0 {
		limit = 10
	}
	qterms := tokenize(query)
	if len(qterms) == 0 {
		view.Status = "empty"
		return view, nil
	}
	var match []int
	for i, tok := range qterms {
		postings := m.Memory.Terms[tok]
		if i == 0 {
			match = append(match, postings...)
			continue
		}
		in := map[int]bool{}
		for _, p := range postings {
			in[p] = true
		}
		next := match[:0]
		for _, idx := range match {
			if in[idx] {
				next = append(next, idx)
			}
		}
		match = next
	}
	for _, idx := range match {
		if len(view.Hits) >= limit {
			break
		}
		en := m.Memory.Entries[idx]
		view.Hits = append(view.Hits, MemoryHit{
			SourceKind: en.SourceKind, SourceID: en.SourceID, RunID: en.RunID,
			Label: en.Label, Snippet: capText(en.Text, 240), CreatedAt: en.CreatedAt,
			ReferenceOnly: true,
		})
	}
	if len(view.Hits) > 0 {
		view.Status = "ok"
	} else {
		view.Status = "empty"
	}
	return view, nil
}
