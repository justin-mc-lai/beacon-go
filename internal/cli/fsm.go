package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/gate"
)

// TransitionResult represents the outcome of evaluating a state transition against the FSM.
type TransitionResult struct {
	Allowed           bool     `json:"allowed"`
	Verdict           string   `json:"verdict"`
	From              string   `json:"from"`
	To                string   `json:"to"`
	Aggregate         string   `json:"aggregate,omitempty"`
	Guard             string   `json:"guard,omitempty"`
	Event             string   `json:"event,omitempty"`
	GuardAttestations []string `json:"guard_attestations,omitempty"`
	GuardExplanation  string   `json:"guard_explanation"`
	Reason            string   `json:"reason"`
}

func newFSMCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "fsm",
		Short: "FSM runtime ability: state transition evaluation and inspection",
	}
	c.AddCommand(newFSMTransitionCmd())
	return c
}

func newFSMTransitionCmd() *cobra.Command {
	var project, feature, version, aggregate, from, to, event string
	var guards []string
	var asJSON bool

	c := &cobra.Command{
		Use:   "transition",
		Short: "Evaluate a state transition against the feature package's FSM specification",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "fsm transition", "no runtime DB yet, feature not found: "+feature)
			}
			defer st.Close()

			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}

			truthPath := filepath.Join(ctx.Project.RootPath, ctx.Feature.TruthPath)
			truthBytes, err := os.ReadFile(truthPath)
			if err != nil {
				return fail(ReasonFeatureNotFound, "fsm transition", fmt.Sprintf("cannot read truth.md at %s: %v", truthPath, err))
			}
			truthMD := string(truthBytes)

			res := evaluateFSMTransition(truthMD, aggregate, from, to, event, guards)

			if asJSON {
				if err := printJSON(cmd, res); err != nil {
					return err
				}
				return transitionResultError(res)
			}

			if res.Allowed {
				cmd.Printf("Verdict: PASS (%s)\nFrom: %s -> To: %s\n", res.Verdict, res.From, res.To)
				if res.Aggregate != "" {
					cmd.Printf("Aggregate: %s\n", res.Aggregate)
				}
				if res.Guard != "" {
					cmd.Printf("Guard: %s\n", res.Guard)
				}
				cmd.Printf("Reason: %s\n", res.Reason)
				cmd.Printf("Guards: %s\n", res.GuardExplanation)
				return nil
			}

			cmd.Printf("Verdict: REJECT (%s)\nFrom: %s -> To: %s\n", res.Verdict, res.From, res.To)
			if res.Aggregate != "" {
				cmd.Printf("Aggregate: %s\n", res.Aggregate)
			}
			cmd.Printf("Reason: %s\n", res.Reason)
			cmd.Printf("Guards: %s\n", res.GuardExplanation)
			return transitionResultError(res)
		},
	}

	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&aggregate, "aggregate", "", "optional aggregate root name")
	c.Flags().StringVar(&from, "from", "", "origin state (required)")
	c.Flags().StringVar(&to, "to", "", "target state (required)")
	c.Flags().StringVar(&event, "event", "", "optional event/trigger to disambiguate transitions")
	c.Flags().StringArrayVar(&guards, "guard", nil, "caller-attested satisfied guard (exact text; repeatable, not business state proof)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit JSON document")

	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("from")
	_ = c.MarkFlagRequired("to")

	return c
}

func evaluateFSMTransition(truthMD, aggregate, from, to, event string, guards []string) TransitionResult {
	res := TransitionResult{
		From: strings.TrimSpace(from), To: strings.TrimSpace(to), Aggregate: aggregate,
		Event: strings.TrimSpace(event), GuardAttestations: guards,
		GuardExplanation: "Supplied guards are caller attestations, not runtime business state proof; no guard expressions are evaluated.",
	}
	p := gate.ParseTruthFSM(truthMD)
	if !p.FiveTuplePresent || len(p.Transitions) == 0 {
		res.Verdict, res.Reason = "model_missing", "No FSM transition model found"
		return res
	}
	for _, ill := range p.Illegal {
		if aggregate != "" && ill.Aggregate != "" && !strings.EqualFold(aggregate, ill.Aggregate) {
			continue
		}
		if strings.EqualFold(ill.From, res.From) && strings.EqualFold(ill.To, res.To) {
			res.Verdict, res.Reason = "illegal_transition", "Matches explicit illegal transition: "+ill.Description
			return res
		}
	}
	if findings, blocked := gate.ValidateTruthFSM(p); blocked {
		res.Verdict = "invalid_model"
		for _, finding := range findings {
			if finding.Severity == "error" {
				res.Reason = finding.Text
				break
			}
		}
		return res
	}
	var match gate.FSMTransition
	matchCount := 0
	for _, tr := range p.Transitions {
		if aggregate != "" && tr.Aggregate != "" && !strings.EqualFold(tr.Aggregate, aggregate) {
			continue
		}
		if strings.EqualFold(tr.State, res.From) && strings.EqualFold(tr.To, res.To) &&
			(res.Event == "" || tr.Event == res.Event) {
			match, matchCount = tr, matchCount+1
		}
	}
	if matchCount == 0 {
		res.Verdict, res.Reason = "transition_not_found", fmt.Sprintf("No defined transition from '%s' to '%s' for the requested aggregate/event", res.From, res.To)
		return res
	}
	if matchCount > 1 {
		res.Verdict, res.Reason = "ambiguous_transition", "Multiple transitions match; supply --event and/or --aggregate to select one unambiguously"
		return res
	}
	tr := match
	res.Event, res.Guard, res.Aggregate = tr.Event, tr.Guard, tr.Aggregate
	satisfied := tr.Guard == "" || strings.EqualFold(tr.Guard, "none") || strings.EqualFold(tr.Guard, "true")
	for _, guard := range guards {
		if guard == tr.Guard {
			satisfied = true
		}
	}
	if !satisfied {
		res.Verdict, res.Reason = "guard_failed", "Guard is not satisfied; supply --guard with its exact text to attest satisfaction: "+tr.Guard
		return res
	}
	res.Allowed, res.Verdict, res.Reason = true, "allowed", "Transition is explicitly defined in the normalized FSM model"
	return res
}

func transitionResultError(res TransitionResult) error {
	if !res.Allowed {
		return fail("fsm_"+res.Verdict, "fsm transition", res.Reason)
	}
	return nil
}
