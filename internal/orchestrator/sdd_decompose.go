package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// decomposeSDDDraft records rule suggestions separately from authored ACs and
// executable tests. A discovered project test command cannot establish coverage.
func (e *Engine) decomposeSDDDraft(fc store.FeatureContext, d core.Decomposition, by string) (DecomposeView, error) {
	if _, err := e.requireCanonicalBranch(fc.Project.RootPath, "truth.Decompose"); err != nil {
		return DecomposeView{}, err
	}
	text, err := readTruthText(fc)
	if err != nil {
		return DecomposeView{}, err
	}
	var b strings.Builder
	b.WriteString("Method: deterministic non-LLM rules. Semantic readiness: false.\n\nThese are suggestions, not acceptance criteria or verified coverage.\n\n")
	b.WriteString("| Candidate | Source | Suggested behavior |\n|---|---|---|\n")
	for _, in := range d.Intents {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", cellEscape(in.ID), cellEscape(in.SourceRef), cellEscape(in.Text))
	}
	for _, j := range d.Journeys {
		fmt.Fprintf(&b, "\n### Suggested journey %s: %s\n\nActors: %s\n", j.ID, cellEscape(j.Name), strings.Join(j.Actors, ", "))
		for _, step := range j.StepsSeed {
			fmt.Fprintf(&b, "\n- %s", step)
		}
		b.WriteString("\n")
	}
	text = replaceSection(text, "## Decomposition Suggestions", b.String())
	snap := gate.ParseIntentSnapshot(text)
	if strings.TrimSpace(snap.Utterance) == "" {
		// Only fill the actual verbatim input; do not invent goals or answered questions.
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) == `utterance: ""` {
				lines[i] = "utterance: " + quoteYAML(d.SurfaceAsk)
				break
			}
		}
		text = strings.Join(lines, "\n")
	}
	text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
	artifact := filepath.ToSlash(filepath.Join(".beacon", "state", "decompose", fc.Feature.Slug+".json"))
	d.Readiness.Ready = false
	d.Readiness.BlocksAutoTick = true
	d.Readiness.Missing = append(d.Readiness.Missing, "external_semantic_review", "authored_acceptance_and_test_contract")
	payload := map[string]any{"method": "deterministic_non_llm", "semantic_ready": false, "decomposition": d}
	artifactPath, err := migrationPath(fc.Project.RootPath, artifact)
	if err != nil {
		return DecomposeView{}, err
	}
	if err := writeDecompositionArtifact(artifactPath, payload); err != nil {
		return DecomposeView{}, err
	}
	if err := writeTruthText(fc, text); err != nil {
		return DecomposeView{}, err
	}
	if err := e.appendFeatureEvent(fc.Feature.ID, EventTruthDecomposed, map[string]any{"by": by, "method": "deterministic_non_llm", "semantic_ready": false, "artifact": artifact}); err != nil {
		return DecomposeView{}, err
	}
	return DecomposeView{Slug: fc.Feature.Slug, Version: fc.Version.Version, Decomposition: d, ArtifactPath: artifact, Method: "deterministic_non_llm", SemanticReady: false, Warnings: []string{"Rule suggestions do not modify authored ACs, coverage matrix or task DAG; complete grounding and open questions, then obtain independent semantic review."}}, nil
}

func writeDecompositionArtifact(path string, payload any) error {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".decompose-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
