---
change_id: chg_a15c2c14f3e0
slug: restatement-gate
status: pending
created_by: BeaconGo 内核
created_at: 2026-11-13T04:42:19.809987Z
base_truth_sha256: 9716d5180bc1caa2ae8a03002c79df6d445bbbb7b01ac2721e0efc56ec0db0bb
base_package_fingerprint: bba9c560b71561deb547140df405616bc9a0bbb4ecae336140565aca3b3e6878
base_requirement_fingerprints:
  AC-001: 48fc2de3b327afe695b5a981636dc66aa6b692828cfa008b8f2c5d6645bca2b6
  AC-002: d12dac0d297a5c33bb0646d4b23b46a5eda13ab3b8a4566991210b995a5525bc
  AC-003: b7aa780fcc314a1db585771889690e30d4c904409f596eb92ad538cff699c610
  AC-004: 2006be6337f808065d32d8acf781dd63900e5503beeb6083b05bef2da7fc5e7e
  AC-005: 143b0d778ffb22eb9805c3c9671b32fd7a5ec03f0d3260dbd8c9d389eb264f55
  AC-006: cce7f4e53e64cc48b92f1ead566435d743bc06a6df7394882c539ab4f44a80ea
  AC-007: aaa201c62310bbc43cca8532af32b410f0b2ce557280d7ec7e2d2fef07a4e4c8
  AC-008: 9cec7b422f2919d846cbae64fe6da4f5dcb85e469031812838a090216c6a7b50

---

# Change Request: AC-004 去掉会被解析器误认的示例 token，并明确追溯只作用于 Acceptance Criteria 表

AC-004 去掉会被解析器误认的示例 token，并明确追溯只作用于 Acceptance Criteria 表

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.

## AC Diff

added:
- (none)

changed:
- AC-004 (before 2006be6337f8 → after 496099d353a8)

removed:
- (none)
