# Evidence Index: sea-to-lake-proposal

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| plan | support_advisory | tasks.md | present | truth |

## Verification Log

Every row is a real test run on 2026-11-05; nothing below is a plan.

| AC | Test selector | Result |
|----|---------------|--------|
| AC-001 | TestValidateLakeProposal | PASS — schema, unique slug and dependency DAG validated |
| AC-002 | TestValidateLakeProposalSize | PASS — fewer than 3 or more than 7 lakes refused |
| AC-003 | TestValidateLakeProposalCommercialFields | PASS — missing commercial field, empty not_proposed and empty utterance refused |
| AC-004 | TestProgramSelectRecordsDecision | PASS — selected/rejected/by/at recorded; unknown slug and empty selection refused |
| AC-005 | TestProgramSelectCreatesDraftsOnlyForSelected | PASS — selected lake gets a draft package, rejected lake does not |
| AC-006 | TestProgramPlanFromProposalRefusesUnselectedDependency | PASS — dependency on an unselected lake refused |
| AC-007 | TestProgramRouteShowsProposalCounts | PASS — route exposes proposal id and selected/rejected counts |
| AC-008 | TestPlanSkillMentionsProposalFlow | PASS — plan skill documents propose, select and freeze |
| AC-009 | TestProgramPlanRequiresProposal | PASS — first-cut direct plan refused with lake_proposal_required; re-plan of an existing program allowed |

## Whole-repo verification (2026-11-05)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS |
| vet | PASS |
| end-to-end chain | PASS |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
