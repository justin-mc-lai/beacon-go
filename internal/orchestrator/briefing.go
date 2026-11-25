package orchestrator

// Briefing is the structured intent restatement recorded in intent.json.
// The LLM client that used to produce it lives outside the kernel now;
// the host agent writes this shape through `truth intent --file`.
type Briefing struct {
	OriginalPrompt  string   `json:"original_prompt"`
	RestatedIntent  string   `json:"restated_intent"`
	IdentifiedRisks []string `json:"identified_risks"`
	Boundaries      []string `json:"boundaries"`
	ProposedSlugs   []string `json:"proposed_slugs"`
	Confirmed       bool     `json:"confirmed"`
	Degraded        bool     `json:"degraded,omitempty"`
}
