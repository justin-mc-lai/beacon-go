# Brownfield intake (chain: full-delivery)

Input: an inherited repository and a second-development request.

For module/requirement-tree intake, also read `module-tree-workflow.md` completely;
its coverage_manifest and counting scope govern tree proposals; candidate HTML is optional in plan, mandatory canonical HTML belongs to truth.

1. `beacon truth code2truth --project . --target <repo> --feature <slug> --by <id>`
   writes an inventory and a draft package under a `反推草稿待人确认` banner.
2. Read the inventory: entry points, routes, models, tests, configuration. Build
   an optional current-state candidate tree; leave unknown behavior
   as open questions.
3. `truth ask` / `truth answer` / `truth intent` / `truth confirm`: the human
   resolves conflicts between code and stated intent. The reverse draft is not
   frozen as authority.
4. Hand the inherited code to `beacon-go-verify` mode `qa-code-review` for
   independent parallel review before changing it.
5. Hand off to truth for canonical formation via `plan init/expand/apply`, mandatory HTML review, approval and `truth node freeze`.

Refusal: do not claim "the code says so" as a requirement, and do not start
implementation before the inherited risks and unknown behavior are recorded.
