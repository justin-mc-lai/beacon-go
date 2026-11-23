package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

func validSDDProposal() LakeProposal {
	p := validProposal()
	p.PackageContract = SDDContract
	p.Version = "v2.0.0"
	for i := range p.Lakes {
		lake := &p.Lakes[i]
		lake.Slug = fmt.Sprintf("sdd-lake-%d", i+1)
		lake.DependsOn = nil
		lake.Truth = strings.Replace(sddTruthFixture(1, 1, false, ""), "slug: demo", "slug: "+lake.Slug, 1)
		lake.Truth = strings.Replace(lake.Truth, "version: v0.0.1", "version: v2.0.0", 1)
		lake.Truth = strings.Replace(lake.Truth, "package_contract: truth-tasks\n", "package_contract: truth-tasks\nstatus: draft\nintent_confirmed: false\ncanonical_refs:\n  truth: docs/features/"+lake.Slug+"/truth.md\n  tasks: docs/features/"+lake.Slug+"/tasks.md\n", 1)
		lake.Tasks = "# Tasks\n\n| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n| TASK-1 | Verify TC-011 | - | AC-001 | go test ./internal/orchestrator -run TestValidateLakeProposal -count=1 | go test ./internal/orchestrator -run TestValidateLakeProposal -count=1 | evidence/rounding.json | false |\n"
	}
	return p
}

func TestSDDProposalContentValidation(t *testing.T) {
	if err := ValidateLakeProposal(validSDDProposal()); err != nil {
		t.Fatalf("valid draft rejected without a signed semantic review: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*LakeProposal)
	}{
		{"no AC", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "| AC-001 | criterion 1 |", "", 1)
		}},
		{"missing model", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "fsm_required: false", "fsm_required: true", 1)
		}},
		{"missing FSM applicability", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "fsm_required: false\n", "", 1)
		}},
		{"missing negative applicability", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "negative_required: false\n", "", 1)
		}},
		{"unknown TC", func(p *LakeProposal) { p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "TC-011", "TC-999", 1) }},
		{"TC identity as Verify", func(p *LakeProposal) {
			p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "-count=1 | go test ./internal/orchestrator -run TestValidateLakeProposal -count=1 |", "-count=1 | TC-011 |", 1)
		}},
		{"shell Verify", func(p *LakeProposal) {
			p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "-count=1 | go test ./internal/orchestrator -run TestValidateLakeProposal -count=1 |", "-count=1 | go test ./x; true |", 1)
		}},
		{"trivial Verify", func(p *LakeProposal) {
			p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "-count=1 | go test ./internal/orchestrator -run TestValidateLakeProposal -count=1 |", "-count=1 | true |", 1)
		}},
		{"unknown matrix AC", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "| AC-001 | TC-011", "| AC-999 | TC-011", 1)
		}},
		{"unknown task AC", func(p *LakeProposal) { p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "AC-001", "AC-999", 1) }},
		{"unknown dependency", func(p *LakeProposal) {
			p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "| - | AC-001", "| TASK-999 | AC-001", 1)
		}},
		{"task cycle", func(p *LakeProposal) {
			p.Lakes[0].Tasks = strings.Replace(p.Lakes[0].Tasks, "| - | AC-001", "| TASK-1 | AC-001", 1)
		}},
		{"lake dependency", func(p *LakeProposal) { p.Lakes[0].DependsOn = []string{"unknown-lake"} }},
		{"missing DAG", func(p *LakeProposal) { p.Lakes[0].Tasks = "# Tasks\n" }},
		{"missing matrix", func(p *LakeProposal) { p.Lakes[0].Truth = strings.Split(p.Lakes[0].Truth, "## Coverage Matrix")[0] }},
		{"unknown contract", func(p *LakeProposal) { p.PackageContract = "invented-contract" }},
		{"slug mismatch", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "slug: sdd-lake-1", "slug: another-lake", 1)
		}},
		{"version mismatch", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "version: v2.0.0", "version: v9.0.0", 1)
		}},
		{"frozen proposal", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "status: draft", "status: frozen", 1)
		}},
		{"confirmed proposal", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "intent_confirmed: false", "intent_confirmed: true", 1)
		}},
		{"unsafe ref", func(p *LakeProposal) {
			p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "docs/features/sdd-lake-1/tasks.md", "../tasks.md", 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validSDDProposal()
			tc.mutate(&p)
			if err := ValidateLakeProposal(p); codeOfErr(err) != core.ReasonLakeProposalInvalid {
				t.Fatalf("invalid SDD accepted: %v", err)
			}
		})
	}
}

func TestSDDProposalFSMNormalizedCoverage(t *testing.T) {
	for _, table := range []string{
		"| From | To | Trigger | Guard | Side effect |\n|---|---|---|---|---|\n| pending | done | approve | reviewed | publish |\n",
		"| Action | To | Guard | Event | State |\n|---|---|---|---|---|\n| publish | done | reviewed | approve | pending |\n",
	} {
		p := validSDDProposal()
		p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "fsm_required: false", "fsm_required: true", 1)
		p.Lakes[0].Truth += "\n## FSM\n" + table + "\n### Illegal transitions\n- illegal: done -> pending\n\n## Transition Coverage\n| From | To | Trigger | TC ID |\n|---|---|---|---|\n| pending | done | approve | TC-011 |\n"
		if err := ValidateLakeProposal(p); err != nil {
			t.Fatalf("normalized FSM rejected: %v", err)
		}
		p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "| approve | TC-011 |", "| approve | TC-999 |", 1)
		if err := ValidateLakeProposal(p); err == nil {
			t.Fatal("unknown transition TC accepted")
		}
		p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "| pending | done | approve | TC-999 |", "", 1)
		if err := ValidateLakeProposal(p); err == nil {
			t.Fatal("missing transition coverage accepted")
		}
	}
}

func TestSDDProposalInvalidNotStoredAndLegacyLabeled(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	legacy, err := env.eng.ProgramPropose(run.RunID, validProposal())
	if err != nil || legacy.Readiness != "legacy_shape_only" || legacy.PackageContract != "" {
		t.Fatalf("legacy contract changed: %+v %v", legacy, err)
	}
	bad := validSDDProposal()
	bad.Lakes[0].Tasks = "# no DAG"
	if _, err := env.eng.ProgramPropose(run.RunID, bad); err == nil {
		t.Fatal("invalid proposal stored")
	}
	_, meta, _, err := env.eng.loadRun(run.RunID)
	if err != nil || meta.LakeProposal == nil || meta.LakeProposal.ProposalID != legacy.ProposalID {
		t.Fatalf("invalid proposal replaced existing proposal: %v", err)
	}
}

func TestSDDProposalSelectionMaterializesOnlyValidatedTwoFileDraft(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	p, err := env.eng.ProgramPropose(run.RunID, validSDDProposal())
	if err != nil {
		t.Fatal(err)
	}
	if p.Readiness != "sdd_content_validated_draft" {
		t.Fatalf("misleading readiness: %s", p.Readiness)
	}
	selected, err := env.eng.ProgramSelect(run.RunID, p.ProposalID, []string{p.Lakes[0].Slug}, []string{p.Lakes[1].Slug, p.Lakes[2].Slug}, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if selected.SelectedBy != "owner" {
		t.Fatal("human decision missing")
	}
	feat, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, p.Lakes[0].Slug)
	if err != nil {
		t.Fatal(err)
	}
	if feat.Status != "draft" || feat.TestsPath != feat.TruthPath {
		t.Fatalf("not an SDD draft registration: %+v", feat)
	}
	dir := filepath.Join(env.root, filepath.Dir(feat.TruthPath))
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 || files[0].Name() != "tasks.md" || files[1].Name() != "truth.md" {
		t.Fatalf("selection did not produce exactly two files: %v %v", files, err)
	}
	for name, want := range map[string]string{"truth.md": p.Lakes[0].Truth, "tasks.md": p.Lakes[0].Tasks} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("validated %s changed: %v", name, err)
		}
	}
	if _, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, p.Lakes[1].Slug); err == nil {
		t.Fatal("unselected lake registered")
	}
	// Identical draft selection is idempotent, not a silent overwrite.
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID, []string{p.Lakes[0].Slug}, nil, "owner"); err != nil {
		t.Fatalf("identical registered draft refused: %v", err)
	}
}

func TestSDDProposalSelectionRefusesExistingConflictsBeforeWrites(t *testing.T) {
	for _, kind := range []string{"legacy", "different SDD", "unregistered", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			env := newTestEnv(t, "draft")
			run := env.createRun(t)
			p, err := env.eng.ProgramPropose(run.RunID, validSDDProposal())
			if err != nil {
				t.Fatal(err)
			}
			conflict := p.Lakes[1].Slug
			switch kind {
			case "legacy":
				if _, err := env.eng.CreateTruthPackage(env.root, p.Version, conflict); err != nil {
					t.Fatal(err)
				}
			case "different SDD":
				if _, err := env.eng.CreateTruthPackageSDD(env.root, p.Version, conflict); err != nil {
					t.Fatal(err)
				}
			case "unregistered":
				if err := os.MkdirAll(filepath.Join(env.root, "docs/features", conflict), 0o755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.MkdirAll(filepath.Join(env.root, "docs/features"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Join(env.root, "docs/features", conflict)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID, []string{p.Lakes[0].Slug, conflict}, nil, "owner"); err == nil {
				t.Fatal("conflicting package reused")
			}
			if _, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, p.Lakes[0].Slug); err == nil {
				t.Fatal("wrote earlier lake before detecting later conflict")
			}
		})
	}
}

func TestSDDProgramFreezeCannotBypassPackageGates(t *testing.T) {
	for _, entry := range []string{"proposal plan", "direct plan", "freeze"} {
		for _, markerOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/marker=%v", entry, markerOnly), func(t *testing.T) {
				env := newTestEnv(t, "draft")
				run := env.createRun(t)
				p, err := env.eng.ProgramPropose(run.RunID, validSDDProposal())
				if err != nil {
					t.Fatal(err)
				}
				slug := p.Lakes[0].Slug
				if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID, []string{slug}, nil, "owner"); err != nil {
					t.Fatal(err)
				}
				if markerOnly {
					feat, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, slug)
					if err != nil {
						t.Fatal(err)
					}
					text := strings.Replace(p.Lakes[0].Truth, "status: draft", "status: frozen", 1)
					if err := os.WriteFile(filepath.Join(env.root, feat.TruthPath), []byte(text), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				switch entry {
				case "proposal plan":
					_, err = env.eng.ProgramPlanFromProposal(run.RunID, p.ProposalID, true)
				case "direct plan":
					// Even replacing the proposal cannot hide an actual SDD package.
					if _, err := env.eng.ProgramPropose(run.RunID, validProposal()); err != nil {
						t.Fatal(err)
					}
					_, err = env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "p", Lakes: []ProgramLakeInput{{Slug: slug}}, Freeze: true})
				case "freeze":
					if _, err := env.eng.ProgramPlanFromProposal(run.RunID, p.ProposalID, false); err != nil {
						t.Fatal(err)
					}
					_, err = env.eng.ProgramFreeze(run.RunID)
				}
				if codeOfErr(err) != ReasonTruthFreezeBlocked {
					t.Fatalf("SDD package gate bypassed: %v", err)
				}
				_, meta, _, loadErr := env.eng.loadRun(run.RunID)
				if loadErr != nil || (meta.Program != nil && meta.Program.Frozen) {
					t.Fatalf("blocked freeze changed plan: %v", loadErr)
				}
			})
		}
	}
}

func TestSDDFrozenProgramChangeRefusesUnfrozenReplacement(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{
		Slug: "legacy-plan", Lakes: []ProgramLakeInput{{Slug: "legacy-lake"}}, Freeze: true,
	}); err != nil {
		t.Fatal(err)
	}
	p, err := env.eng.ProgramPropose(run.RunID, validSDDProposal())
	if err != nil {
		t.Fatal(err)
	}
	slug := p.Lakes[0].Slug
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID, []string{slug}, nil, "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramChange(run.RunID, "replace with SDD lake", []ProgramLakeInput{{Slug: slug}}, true); codeOfErr(err) != ReasonTruthFreezeBlocked {
		t.Fatalf("frozen replacement bypassed package gate: %v", err)
	}
	_, meta, _, err := env.eng.loadRun(run.RunID)
	if err != nil || meta.Program == nil || !meta.Program.Frozen || len(meta.Program.Lakes) != 1 || meta.Program.Lakes[0].Slug != "legacy-lake" || len(meta.Program.Changes) != 0 {
		t.Fatalf("refused replacement changed frozen plan: %+v %v", meta.Program, err)
	}
}

func TestSDDProposalMissingIdentityDefaultsPreserveBody(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	p := validSDDProposal()
	p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "slug: "+p.Lakes[0].Slug+"\n", "", 1)
	p.Lakes[0].Truth = strings.Replace(p.Lakes[0].Truth, "status: draft\n", "", 1)
	p, err := env.eng.ProgramPropose(run.RunID, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID, []string{p.Lakes[0].Slug}, nil, "owner"); err != nil {
		t.Fatal(err)
	}
	feat, _ := env.st.Features().GetBySlug(env.feature.ProjectVersionID, p.Lakes[0].Slug)
	got, err := os.ReadFile(filepath.Join(env.root, feat.TruthPath))
	if err != nil {
		t.Fatal(err)
	}
	scalars, _, _ := gate.ParseFrontmatter(string(got))
	if scalars["slug"] != p.Lakes[0].Slug || scalars["status"] != "draft" || scalars["intent_confirmed"] != "false" {
		t.Fatalf("missing safe identity defaults: %v", scalars)
	}
	beforeBody := strings.SplitN(p.Lakes[0].Truth[4:], "\n---", 2)[1]
	afterBody := strings.SplitN(string(got)[4:], "\n---", 2)[1]
	if beforeBody != afterBody {
		t.Fatal("selection rewrote validated requirement body")
	}
}
