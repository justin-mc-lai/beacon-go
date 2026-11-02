# skills (go:embed)

The Scheme A 1+6 public skill surface, embedded in the binary:
the `goal` facade plus `plan`, `truth`, `design`, `implement`,
`qa`, `release` — each a directory with a `SKILL.md` (the
always-loaded surface) and `references/` (loaded on intent match).

The contents are slim operating manuals for the Go binary (the
full Python v1.6.19 skill tree under `skills/beacon/` in the
Python repo remains the deeper reference; each references/core.md
here is distilled from it and says so).

Hosts discover skills from files, not from binaries: run
`beacon skills install` to materialize this set into a host skill
directory (`~/.claude/skills`, `~/.codex/skills`,
`~/.config/opencode/skills`, `~/.agents/skills`, or `--dir`).
Installed names are namespaced (`beacon-go-goal`, `beacon-go-truth`, …)
so they never collide with the Python v1 `beacon-<id>` set, plus one
generated `beacon-go` router skill whose version comes from the binary.
