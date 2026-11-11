---
change_id: chg_8cfeec4ce08f
slug: kernel-readonly-http-api
status: pending
created_by: apple(via codex)
created_at: 2026-11-11T13:46:04.541244Z
base_truth_sha256: 81a11d7b76cc9b25c825a4db59b6686ee1f30ed34acaec5eae7fd132c1d3f163
base_package_fingerprint: 8794ff36e0719c2b57726991e08e57e0cff49b5ba406f2a2ddde2e8070564952
base_requirement_fingerprints:
  AC-HTTP-001: 298dd30737c00185be98a116505541f6da98a846123efe9389cbedd7de81769e
  AC-HTTP-002: 9099993f9bffe76c4ff8d7b4b8e5ed49f14cb230b24d9bf302f35cbc830b5344
  AC-HTTP-003: 92d5f1d0df0b8b6c458a69918f7da742b37c5ce89248644dd47fd2c1708fbf08
  AC-HTTP-004: 06387069b65b5d4fa33ec25595b4f048d054313c1f57f8e17da99748b368c05a
  AC-HTTP-005: fce681aff5069dcb52b4d59e63cfaf1c2fdfddb1384f27d553aed8753fec51a1
  AC-HTTP-006: 4684305706d1a311c8638d7189c7a912ccf773ae31b95f236d51060173b6440c
  AC-HTTP-007: aa5aa823d81247cc164a122b3453dba7a6bd2634d8cc679a4c5252ed0f6d2775
  AC-HTTP-008: 4ef244e2aaa1b0dfd3f699174cca74a7e63b372aa5ea9f0a8ac65c81f011e9b1

---

# Change Request: AC-HTTP-004 adds started_at to each /spine/runs item so the frozen ordering assertion is observable; seed3 renames the zsh loop variable path->req_path (assertions unchanged) to stop clobbering PATH

AC-HTTP-004 adds started_at to each /spine/runs item so the frozen ordering assertion is observable; seed3 renames the zsh loop variable path->req_path (assertions unchanged) to stop clobbering PATH

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.

## AC Diff

added:
- (none)

changed:
- AC-HTTP-004 (before 06387069b65b → after 491e031a3d7c)

removed:
- (none)
