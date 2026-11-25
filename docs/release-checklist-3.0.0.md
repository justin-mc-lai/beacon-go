# Release checklist — v3.0.0

## Preconditions

- [ ] `internal/version/version.go` = `3.0.0`
- [ ] README / README.zh-CN current release line = `v3.0.0`
- [ ] `CHANGELOG.md` has `[3.0.0]` with breaking/add/change sections
- [ ] `docs/beacon/v3.0.0/release/3.0.0.md` exists and is not a stub
- [ ] `docs/migration-3.0.0.md` covers ledger, packages, commands, rollback

## Verification

```bash
bash scripts/check_version_consistency.sh
bash scripts/test_check_version_consistency.sh
bash scripts/check_kernel_deps.sh
go build ./...
go vet ./...
go test ./... -count=1
bash e2e/run_e2e.sh
bash e2e/run_dogfood.sh
```

## Tag and release

- Tag: `v3.0.0` on the verified commit.
- GitHub Release: title `Beacon Go v3.0.0`, body = `docs/beacon/v3.0.0/release/3.0.0.md`.
- Assets: `beacon` binary for darwin/arm64 (sha256 recorded in the release body).

## Local install

Use the approved entry only:

```bash
python3 scripts/local_release.py prepare|approve|install --state-dir <dir> ...
```

Evidence must bind the source commit and the built binary SHA-256 and name an
independent checker (`e2e-suite`).

## Rollback

1. Stop running beacon processes.
2. Restore the backup recorded by `local_release.py install` (or the SQLite backup
   from `~/.beacon/`).
3. Reinstall the previous binary through `scripts/local_release.py`.
4. Verify `beacon --version` and `beacon plan tree --help`.
