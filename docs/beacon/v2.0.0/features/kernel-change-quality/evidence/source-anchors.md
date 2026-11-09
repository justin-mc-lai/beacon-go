# P3 源锚点：kernel-change-quality

- 上游仓库：https://github.com/loopx-project/loopx（镜像/前身上游 https://github.com/huangruiteng/loopx）
- 基线 commit：c9c0644be428bbb0ab10726637f5b36602972912（2.3.0 发布窗口 HEAD）
- 取舍依据说明：上游历史快照 91d8ed06 缺失 change_quality/README.md（+247行规范）及完整 premerge 校验约定，因此统一以实际核验过的 2.3.0 发布 HEAD c9c0644 作为权威基线。
- 锚点清单来源：BeaconGo 研究 2026-11-09 落盘的 loopx-p1-p2-p3-p5-source-anchors.md，本文件只摘 P3 部分
- 主锚点文件：`loopx/capabilities/change_quality/policy.py`、`loopx/capabilities/change_quality/goal_configuration.py`、`loopx/capabilities/change_quality/receipt.py`、`loopx/capabilities/change_quality/scope.py`、`loopx/capabilities/change_quality/result.py`、`loopx/capabilities/change_quality/README.md`、`loopx/control_plane/capabilities/change_quality_validation.ts`

## AC-001（上游 AC-Q1）

- 判据：策略默认 off，可选 `safe_fix` 或 `strict_receipt`。
- 文件与行号：`loopx/capabilities/change_quality/policy.py` L10-L26；`loopx/capabilities/change_quality/goal_configuration.py` L43-L64；`loopx/capabilities/change_quality/README.md` L3-L15
- 支撑点摘录：
  - `policy.py` L10-L26：策略 schema 固定为 `change_quality_qualification_policy_v0`；默认 `enabled = raw.get("enabled") is True`（缺省即 off），支持独立布尔配置 `safe_fix = raw.get("safe_fix") is True` 与 `strict_receipt = raw.get("strict_receipt") is True`。
    ```python
    CHANGE_QUALITY_POLICY_SCHEMA_VERSION = "change_quality_qualification_policy_v0"


    def change_quality_goal_policy(goal: dict[str, Any]) -> dict[str, Any]:
        control_plane = (
            cast(dict[str, Any], goal.get("control_plane"))
            if isinstance(goal.get("control_plane"), dict)
            else {}
        )
        raw = (
            cast(dict[str, Any], control_plane.get("change_quality_qualification"))
            if isinstance(control_plane.get("change_quality_qualification"), dict)
            else {}
        )
        return {
            "schema_version": CHANGE_QUALITY_POLICY_SCHEMA_VERSION,
            "enabled": raw.get("enabled") is True,
            "safe_fix": raw.get("safe_fix") is True,
            "strict_receipt": raw.get("strict_receipt") is True,
        }
    ```
  - `goal_configuration.py` L43-L64：Goal 级控制面支持独立设置 `safe_fix` 与 `strict_receipt` 开关。
  - `README.md` L3-L15：官方规范明确 `safe_fix` 与 `strict_receipt` 是两个正交但可组合的策略选项，默认完全处于非激活（off）状态。

## AC-002（上游 AC-Q2）

- 判据：`strict_receipt` 在 premerge 校验回执与**精确当前 diff SHA** 一致，否则拒绝。
- 文件与行号：`loopx/capabilities/change_quality/scope.py` L190-L215；`loopx/capabilities/change_quality/receipt.py` L42-L73、L505-L520；`loopx/capabilities/change_quality/README.md` L87-L91
- 支撑点摘录：
  - `scope.py` L190-L215：计算 `scope_fingerprint`，严格捕获 base commit 与当前 working tree 的精确 diff SHA（committed、staged、unstaged 与 untracked 全部纳入 SHA-256 哈希计算）。
  - `receipt.py` L42-L73 与 L505-L520：回执状态状态机显式定义 `stale_receipt` 与 `receipt_missing`；启用 `strict_receipt` 时，`ok = receipt_valid or not strict`；若当前指纹与回执不一致，premerge 校验拒绝通过。
    ```python
    _PR_EVIDENCE_BY_STATUS: dict[str, str] = {
        "valid": "The receipt matches the exact current base and diff.",
        "stale_receipt": "A prior receipt exists, but the exact base or diff changed; requalification is required.",
        "receipt_missing": "No receipt is recorded for the exact current base and diff.",
        "failed_qualification": "The qualification receipt recorded a failure on the current base and diff.",
    }
    ```
  - `README.md` L87-L91：`canary premerge --goal-id <goal-id>` 要求合并前回执指纹必须与真实当前 diff SHA 严格吻合，任何篡改或偏离立即 fail-closed。

## AC-003（上游 AC-Q3）

- 判据：`safe_fix` 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。
- 文件与行号：`loopx/capabilities/change_quality/receipt.py` L202-L205、L225-L229；`loopx/capabilities/change_quality/result.py` L355-L368；`loopx/capabilities/change_quality/README.md` L13-L15、L195-L207；`loopx/control_plane/capabilities/change_quality_validation.ts` L15-L85
- 支撑点摘录：
  - `receipt.py` L202-L205：常量强制限定 `max_safe_fix_passes: 1`。仅允许单次有界修复轮次。
  - `result.py` L355-L368：未授权 safe_fix 或状态不符直接抛出 `ValueError`。
  - `README.md` L13-L15 与 L195-L207：官方明确 safe_fix 不赋予写权限，严格受限于调用方已有工作区授权，且仅允许最多 1 pass；超出授权直接终止（“This version deliberately qualifies one final diff with at most one safe-fix pass... only one coherent safe-fix pass can be reported”）。
  - `change_quality_validation.ts` L15-L85：在控制面阶段执行 `fail(...)` 校验不变式，若修复范围越出原授权清单或违背预检契约，立即执行 fail-closed 阻断。
