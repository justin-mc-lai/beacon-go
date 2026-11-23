# Issue 11 entry-point closure

Tracking: https://github.com/justin-mc-lai/beacon-go/issues/11#issuecomment-7103068807

## Working copy preservation

The original developer checkout was updated to main at 64a8ecd after preserving all tracked edits and untracked version/change documents in local branch `wip/preserve-pre-go16-20261123` at fa778db8bccd9cfc730f6dc5e28b11b4a7fc03d7. This branch is a preservation checkpoint, not reviewed product code and not intended for automatic merge.

A full non-git workspace archive, binary tracked patch, status inventory and SHA256 are under `~/.beacon/workspace-backups/beacon-go-20261123-pre-main/`. Existing runtime data remains in the original `.beacon/` directory. No reset, stash, clean or force checkout was used.

## Decomposition boundary

`truth decompose` is deterministic, non-LLM rule matching. Its candidate suggestions are not a semantic specification or executed test coverage. For SDD packages, suggestions live in their own section of truth.md; authored acceptance criteria, grounding, open questions and tasks.md are preserved. It does not create tests.md or overwrite truth.md with a tests document.

An external agent must turn the grounded intent and relevant sources into observable requirements, test assertions and an actual task graph. Open questions and scaffold materials still block freeze. A discovered `go test ./...` command is not automatically evidence for every suggested AC.

## Acceptance before closure

The GitHub checklist defines C11-01 through C11-07. Neither compiling this change nor passing an internal helper test closes the issue. The final evidence must cover the actual create/decompose, proposal/select/freeze and migrate CLI paths, including rejection paths, with exact source and binary hashes and independent review.

The runtime's existing local bounded repair is not a security sandbox. If issue closure requires container-level isolation, that requirement must be explicitly resolved and tested before closing; the implementation must not silently substitute workdir isolation.
