package orchestrator

import (
	"context"
	"fmt"
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
type Briefing struct {
	OriginalPrompt string   `json:"original_prompt"`
	RestatedIntent string   `json:"restated_intent"`
	IdentifiedRisks []string `json:"identified_risks"`
	Boundaries     []string `json:"boundaries"`
	ProposedSlugs  []string `json:"proposed_slugs"`
	Confirmed      bool     `json:"confirmed"`
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

// AlignIntent takes a raw user goal prompt, restates it, and generates a structured Briefing.
func (c *ChiefOfStaff) AlignIntent(ctx context.Context, prompt string) (*Briefing, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty prompt provided to chief of staff")
	}

	systemPrompt := `You are the Chief of Staff in Beacon Go.
Your job is NOT to write code or modify files.
Your job is Indirect Prompting: restate the user's requirement, identify hidden risks/invariants,
and summarize the boundary into a strict confirmation brief.`

	userPrompt := fmt.Sprintf("Analyze this requirement and restate it:\n%s", prompt)

	req := llm.Request{
		Role:  llm.RoleChiefOfStaff,
		Model: c.model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
	}

	resp, err := c.client.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("chief of staff LLM alignment failed: %w", err)
	}

	// Deterministic fallback/structured parse
	b := &Briefing{
		OriginalPrompt: prompt,
		RestatedIntent: resp.Content,
		IdentifiedRisks: []string{
			"Do not break existing database schema or FSM transitions",
			"Require full test coverage and machine evidence before QA gate",
		},
		Boundaries: []string{
			"Code modifications restricted to feature-specific worktree",
			"Must produce exit 0 evidence before release",
		},
		ProposedSlugs: []string{slugify(prompt)},
		Confirmed:     false,
	}
	return b, nil
}

// AlignIntentStream aligns user prompt and calls chunkHandler as tokens stream in.
func (c *ChiefOfStaff) AlignIntentStream(ctx context.Context, prompt string, chunkHandler func(chunk string) error) (*Briefing, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty prompt provided to chief of staff")
	}

	systemPrompt := `You are the Chief of Staff in Beacon Go.
Your job is NOT to write code or modify files.
Your job is Indirect Prompting: restate the user's requirement, identify hidden risks/invariants,
and summarize the boundary into a strict confirmation brief.`

	userPrompt := fmt.Sprintf("Analyze this requirement and restate it:\n%s", prompt)

	req := llm.Request{
		Role:  llm.RoleChiefOfStaff,
		Model: c.model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
	}

	resp, err := c.client.StreamComplete(ctx, req, chunkHandler)
	if err != nil {
		return nil, fmt.Errorf("chief of staff LLM alignment failed: %w", err)
	}

	// Deterministic fallback/structured parse
	b := &Briefing{
		OriginalPrompt: prompt,
		RestatedIntent: resp.Content,
		IdentifiedRisks: []string{
			"Do not break existing database schema or FSM transitions",
			"Require full test coverage and machine evidence before QA gate",
		},
		Boundaries: []string{
			"Code modifications restricted to feature-specific worktree",
			"Must produce exit 0 evidence before release",
		},
		ProposedSlugs: []string{slugify(prompt)},
		Confirmed:     false,
	}
	return b, nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}
