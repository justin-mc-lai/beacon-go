package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"beacon-go/internal/llm"
)

// ChiefOfStaff implements the high-level intent alignment agent (AC-AGENT-002).
// It follows the SpaceXAI "Indirect Prompting" principle:
// Never touch code immediately; restate user intent, surface boundaries, and generate a brief.
type ChiefOfStaff struct {
	client llm.Client
	model  string
}

// Briefing represents the structured alignment card generated before dispatch.
//
// IdentifiedRisks and Boundaries are empty unless the model actually
// returned them. The earlier build filled them with two constants, which
// made every plan card claim the same invariants — a fabricated review.
type Briefing struct {
	OriginalPrompt  string   `json:"original_prompt"`
	RestatedIntent  string   `json:"restated_intent"`
	IdentifiedRisks []string `json:"identified_risks"`
	Boundaries      []string `json:"boundaries"`
	ProposedSlugs   []string `json:"proposed_slugs"`
	Confirmed       bool     `json:"confirmed"`
	// Degraded is true when the model answer carried no parseable
	// structured tail; the slug was then derived from the utterance.
	Degraded bool `json:"degraded,omitempty"`
}

// NewChiefOfStaff creates a new Chief of Staff instance.
func NewChiefOfStaff(client llm.Client, model string) *ChiefOfStaff {
	if model == "" {
		model = "claude-3-7-sonnet-20250219"
	}
	return &ChiefOfStaff{
		client: client,
		model:  model,
	}
}

const chiefSystemPrompt = `You are the Chief of Staff in Beacon Go.
Your job is NOT to write code or modify files.
Your job is Indirect Prompting: restate the user's requirement, identify hidden risks/invariants,
and summarize the boundary into a strict confirmation brief.

Answer in two parts:
1. A short restatement of the requirement in the user's language.
2. A fenced JSON block as the LAST thing in your answer, exactly:
` + "```json" + `
{"slug":"kebab-case-ascii-slug","risks":["..."],"boundaries":["..."]}
` + "```" + `
Only list risks and boundaries you can actually justify from the requirement.
Never invent invariants. If there are none, return empty arrays.`

// AlignIntent takes a raw user goal prompt, restates it, and generates a structured Briefing.
func (c *ChiefOfStaff) AlignIntent(ctx context.Context, prompt string) (*Briefing, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty prompt provided to chief of staff")
	}
	resp, err := c.client.Complete(ctx, c.request(prompt))
	if err != nil {
		return nil, fmt.Errorf("chief of staff LLM alignment failed: %w", err)
	}
	return buildBriefing(prompt, resp.Content), nil
}

// AlignIntentStream aligns user prompt and calls chunkHandler as tokens stream in.
func (c *ChiefOfStaff) AlignIntentStream(ctx context.Context, prompt string, chunkHandler func(chunk string) error) (*Briefing, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty prompt provided to chief of staff")
	}
	resp, err := c.client.StreamComplete(ctx, c.request(prompt), chunkHandler)
	if err != nil {
		return nil, fmt.Errorf("chief of staff LLM alignment failed: %w", err)
	}
	return buildBriefing(prompt, resp.Content), nil
}

func (c *ChiefOfStaff) request(prompt string) llm.Request {
	return llm.Request{
		Role:  llm.RoleChiefOfStaff,
		Model: c.model,
		Messages: []llm.Message{
			{Role: "system", Content: chiefSystemPrompt},
			{Role: "user", Content: "Analyze this requirement and restate it:\n" + prompt},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
	}
}

type briefTail struct {
	Slug       string   `json:"slug"`
	Risks      []string `json:"risks"`
	Boundaries []string `json:"boundaries"`
}

// buildBriefing splits the model answer into prose + the JSON tail. A
// missing or malformed tail degrades to prose-only; it never invents
// risks, boundaries or a slug the model did not produce.
func buildBriefing(prompt, answer string) *Briefing {
	b := &Briefing{OriginalPrompt: prompt, RestatedIntent: strings.TrimSpace(answer)}
	if prose, tail, ok := splitBriefTail(answer); ok {
		if strings.TrimSpace(prose) != "" {
			b.RestatedIntent = strings.TrimSpace(prose)
		}
		b.IdentifiedRisks = cleanList(tail.Risks)
		b.Boundaries = cleanList(tail.Boundaries)
		if s := safeSlug(tail.Slug); s != "" {
			b.ProposedSlugs = []string{s}
		}
	}
	if b.RestatedIntent == "" {
		b.RestatedIntent = strings.TrimSpace(answer)
	}
	if len(b.ProposedSlugs) == 0 {
		b.ProposedSlugs = []string{derivedSlug(prompt)}
		b.Degraded = true
	}
	b.Confirmed = false
	return b
}

// splitBriefTail finds the last fenced json block and returns the prose
// before it plus the decoded tail.
func splitBriefTail(answer string) (string, briefTail, bool) {
	idx := strings.LastIndex(answer, "```json")
	if idx < 0 {
		return "", briefTail{}, false
	}
	rest := answer[idx+len("```json"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return "", briefTail{}, false
	}
	var tail briefTail
	if err := json.Unmarshal([]byte(strings.TrimSpace(rest[:end])), &tail); err != nil {
		return "", briefTail{}, false
	}
	return strings.TrimSpace(answer[:idx]), tail, true
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

var (
	nonSlugChar = regexp.MustCompile(`[^a-z0-9]+`)
	slugHead    = regexp.MustCompile(`^[a-z0-9]`)
)

// safeSlug normalises a model-proposed slug. It returns "" when the
// proposal cannot be made into a valid package slug (for example a
// Chinese-only name), so the caller can fall back to a derived one.
func safeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlugChar.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" || !slugHead.MatchString(s) {
		return ""
	}
	if len(s) > 48 {
		s = strings.Trim(s[:48], "-")
	}
	return s
}

// derivedSlug builds a deterministic ASCII slug from the utterance so a
// non-Latin prompt still gets a valid package name.
func derivedSlug(prompt string) string {
	if s := safeSlug(prompt); s != "" {
		return s
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(prompt)))
	return "feature-" + hex.EncodeToString(sum[:4])
}
