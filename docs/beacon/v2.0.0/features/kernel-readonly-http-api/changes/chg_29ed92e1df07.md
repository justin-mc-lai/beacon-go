---
change_id: chg_29ed92e1df07
slug: kernel-readonly-http-api
status: cancelled
created_by: apple(via codex)
created_at: 2026-11-11T13:42:54.095114Z
base_truth_sha256: 81a11d7b76cc9b25c825a4db59b6686ee1f30ed34acaec5eae7fd132c1d3f163
base_package_fingerprint: 8794ff36e0719c2b57726991e08e57e0cff49b5ba406f2a2ddde2e8070564952

---

# Change Request: AC-HTTP-004 adds started_at to each /spine/runs item so the frozen ordering assertion is observable; seed3 renames the zsh loop variable path->req_path (assertions unchanged) to stop clobbering PATH

AC-HTTP-004 adds started_at to each /spine/runs item so the frozen ordering assertion is observable; seed3 renames the zsh loop variable path->req_path (assertions unchanged) to stop clobbering PATH

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
