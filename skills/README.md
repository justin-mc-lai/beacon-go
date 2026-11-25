# skills (go:embed)

The seven public skill packages are embedded in the binary under their
installed names: `beacon-go-goal`, `beacon-go-plan`, `beacon-go-truth`,
`beacon-go-design`, `beacon-go-implement`, `beacon-go-verify`, and
`beacon-go-release`. Each directory carries a `SKILL.md` (the always-loaded
surface), `agents/openai.yaml`, `references/`, `examples/`, and `evals/`.

The contents are slim operating manuals for the Go binary. The Python v1.6.19
skill tree remains the deeper historical reference; each `references/core.md`
here is distilled from it and says so.

Hosts discover skills from files, not from binaries: run
`beacon skills install` to materialize this set into a host skill directory
(`~/.claude/skills`, `~/.codex/skills`, `~/.config/opencode/skills`,
`~/.agents/skills`, or `--dir`). Installed names are namespaced
(`beacon-go-goal`, `beacon-go-truth`, …) so they never collide with the Python
v1 `beacon-<id>` set, plus one generated `beacon-go` router skill whose
version comes from the binary.

Source directory names match the frontmatter `name` and the installed
directory name, so the same package passes `quick_validate.py` before and
after installation. The router is generated at install time because it must
name the current skill set without carrying a stale version literal.

`beacon-go-plan` includes `pln-brownfield` for inherited codebases;
`beacon-go-verify` includes `qa-code-review`, a host-side parallel reviewer
protocol that registers an aggregate artifact without pretending the kernel
ran the reviewers.
