---
change_id: chg_ebbeadee16f3
slug: intent-contract-bridge
status: pending
created_by: beacon-go-kernel
created_at: 2026-11-11T04:26:13.298572Z
base_truth_sha256: 0bb6dba67c70422213bd8778071083d677b41646c4c3406db4fb47a29f3dda7d
base_package_fingerprint: 62eb8d320113c0356280451235894546ec50e1343d22300a50c9b1b0eb718061
base_requirement_fingerprints:
  AC-001: e764d4329cdb3a659f7ee64185354c35768ff81cb36a64836e2ff716cbb4a03a
  AC-002: 430bcb3de3446348e4ded16c4c2a7f87c7430149f94ba86459b6cc17fa7b9014
  AC-003: d0668a78fd34120da636c61d1206d5b9fa6dc221092e9afcdc026e28f57b9426

---

# Change Request: close AC-001 seed gaps found by acceptance recheck: run selection (--run-id wins, else latest run), confirmed=false mirroring; drop out-of-contract intent_path output assertions; add malformed intent.json negative and non-replay resume assertion; record actual change-file status instead of claiming closed

close AC-001 seed gaps found by acceptance recheck: run selection (--run-id wins, else latest run), confirmed=false mirroring; drop out-of-contract intent_path output assertions; add malformed intent.json negative and non-replay resume assertion; record actual change-file status instead of claiming closed

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.

## AC Diff

added:
- (none)

changed:
- (none)

removed:
- (none)
