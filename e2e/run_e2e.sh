#!/usr/bin/env bash
# Beacon Go (v2.0.0-go.8) — end-to-end full-chain script.
#
# Chain (one step must not be skipped; no manual back-filling anywhere):
#   1. git-init a temp project + `projects register`
#   2. `truth create` a feature package (slug: e2e-login-2fa)
#   3. fill the four-piece package with minimal real content
#  3b. `truth ask` -> `truth intent` -> `truth answer` -> `truth confirm`
#      (R1 ask-before-sign: the intent gate refuses an unsigned package)
#   4. `truth freeze`
#  4b. commit the frozen package on main + `truth baseline`
#      (S1, AC-139: a frozen body becomes a consumable baseline
#      only once committed on the canonical branch; step 7's
#      admission enforces it — this step was added with S1 and is
#      the one declared script change of that slice)
#   5. `gate evaluate truth_freeze` (formal verdict)
#  5b. `goal topology project` a single-repo GitScopeGraph
#      (S3, AC-171/173: since go.7 a git project's implement
#      admission consumes the topology gate; step 7 enforces it —
#      this step was added with S3 and is the one declared script
#      change of that slice)
#   6. `goal run` (--executor-id maker; subprocess profile executor via
#      --exec-cmd pointing at a real stage-agent script; no --ux, so the
#      design stage must NOT be inserted)
#   7. `goal admit` (shared implement admission — must fully pass)
#   8. `goal driver start` + `goal driver run-loop`
#      (--executor subprocess:codex-style) pushing the run to release_gate;
#      the agent script does real per-stage work and produces a QA report
#      file at the qa stage
#  8b. `goal artifacts register` the QA report (P6 fix, v2.0.0-go.5):
#      the on-disk report becomes a registered qa_report pointer with
#      its real sha256 — the only kind of evidence the qa gate trusts
#   9. `gate evaluate` design / implement / qa / release (qa must now
#      pass on the registered evidence; release is evaluated BEFORE
#      completion and is expected to block)
#  10. `goal attach-verifier` (checker != maker) + `goal complete
#      --human-confirmed`
#  11. `spine show` / `spine list` / `goal trace` read-backs
#
# Isolation: a fresh BEACON_STORE_PATH temp DB + a fresh temp project
# directory per run. The default store (~/.beacon/beacon.db) is never
# touched; no Go source is modified; no data is seeded by hand.
#
# Exit codes:
#   0  full chain passed (run completed, status done)
#   1  an unexpected step failed (set -e fail-fast)
# (Historical: the v2.0.0-go.3/go.4 runs stopped at step 10 with exit 3
# on the QA-evidence gap; the artifact registration face in go.5
# closed that gap, so exit 3 no longer exists.)
#
# Log: e2e/last_run.log (full tee of stdout+stderr).

set -euo pipefail

E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BEACON="${BEACON_BIN:-$E2E_DIR/../bin/beacon}"
LOG_FILE="$E2E_DIR/last_run.log"

# ---- tee plumbing: run main() once, mirror everything to the log ----
main() {
  local START_TS=$SECONDS
  STEP="setup"
  echo "=================================================================="
  echo "Beacon Go P6 E2E — full chain"
  echo "started: $(date '+%F %T %Z')"
  echo "beacon:  $BEACON"
  "$BEACON" --version
  "$BEACON" --version | grep -q "v2.0.0-go.8" || {
    echo "FAIL: this script targets Beacon CLI v2.0.0-go.8"
    return 1
  }

  # ---- isolated workspace (temp store + temp project; never default DB)
  WORK="$(mktemp -d /tmp/beacon-e2e.XXXXXXXX)"
  PROJ="$WORK/proj"
  AGENT="$WORK/bin/agent.sh"
  export BEACON_STORE_PATH="$WORK/beacon.db"
  echo "workspace:        $WORK"
  echo "BEACON_STORE_PATH=$BEACON_STORE_PATH (isolated temp store)"
  echo "project:          $PROJ"

  SLUG="e2e-login-2fa"
  VERSION="v1.0.0"
  MAKER="e2e-maker"       # goal run --executor-id (maker)
  CHECKER="e2e-checker"   # attach-verifier --source (checker, != maker)
  EVALUATOR="e2e-evaluator"

  # jval <dotted.path> : extract a field from stdin JSON
  jval() {
    python3 -c '
import json, sys
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    d = d[k]
print(d)
' "$1"
  }

  # ==================================================================
  STEP="1 projects register (temp git project)"
  echo; echo "===== STEP 1: $STEP ====="
  mkdir -p "$PROJ"
  git init -q -b main "$PROJ"
  git -C "$PROJ" config user.email "e2e@local"
  git -C "$PROJ" config user.name "e2e"
  printf '# e2e temp project\n\nTemporary project created by run_e2e.sh for the Beacon Go P6 end-to-end chain.\n' \
    > "$PROJ/README.md"
  git -C "$PROJ" add README.md
  git -C "$PROJ" commit -qm "initial commit (README)"
  git -C "$PROJ" log --oneline
  "$BEACON" projects register --path "$PROJ" --version "$VERSION" --json

  # ==================================================================
  STEP="2 truth create"
  echo; echo "===== STEP 2: $STEP ====="
  OUT="$("$BEACON" truth create --project "$PROJ" --slug "$SLUG" --json)"
  echo "$OUT"
  PKG_DIR="$PROJ/$(printf '%s' "$OUT" | jval package_dir)"
  for f in truth.md tests.md tasks.md evidence.md; do
    [ -s "$PKG_DIR/$f" ] || { echo "FAIL: package member missing/empty: $f"; return 1; }
  done
  echo "package dir verified: $PKG_DIR (four-piece skeleton present)"

  # ==================================================================
  STEP="3 fill four-piece package"
  echo; echo "===== STEP 3: $STEP ====="
  PKG_DIR="$PKG_DIR" python3 - <<'PYEOF'
import os, pathlib
d = pathlib.Path(os.environ["PKG_DIR"])

p = d / "truth.md"; t = p.read_text()
old = "（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）"
new = "给已注册用户提供 TOTP 二次验证：开启后每次登录在密码校验通过后还必须输入动态码，动态码错误或重复使用一律拒绝登录，做到账号密码泄露后仍不能直接登录算完。"
assert old in t, "truth.md placeholder not found"
t = t.replace(old, new)
assert 'utterance: ""' in t
t = t.replace('utterance: ""', 'utterance: "给登录加 TOTP 二次验证，动态码错误或重放时拒绝登录"')
old_ac = "| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |"
new_ac = ("| AC-001 | 开启 TOTP 的用户登录时，密码正确但未提供有效动态码则登录被拒绝并返回明确错误；提供当前窗口有效动态码则登录成功 |\n"
          "| AC-002 | 同一动态码在有效窗口内成功使用一次后，第二次提交同一动态码必须被拒绝（防重放） |")
assert old_ac in t, "truth.md AC-001 skeleton row not found"
t = t.replace(old_ac, new_ac)
p.write_text(t)

p = d / "tests.md"; t = p.read_text()
old_tc = "| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | ac_001_holds |"
new_tc = ("| AC-001 | TC-001 | behavior | `go test ./... -run TestLogin2FAChallenge` | login_rejected_without_valid_totp_and_accepted_with_valid_totp |\n"
          "| AC-002 | TC-002 | behavior | `go test ./... -run TestLogin2FAReplay` | replayed_totp_rejected |")
assert old_tc in t, "tests.md TC-001 skeleton row not found"
t = t.replace(old_tc, new_tc)
p.write_text(t)

p = d / "tasks.md"; t = p.read_text()
old_task = "- [ ] TASK-001 Implement AC-001 · ac=AC-001"
new_task = "- [ ] TASK-001 Implement TOTP challenge on login (AC-001) and replay rejection (AC-002) · ac=AC-001"
assert old_task in t, "tasks.md TASK-001 skeleton row not found"
t = t.replace(old_task, new_task)
p.write_text(t)
print("filled truth.md / tests.md / tasks.md (evidence.md keeps bare-name index per skeleton boundary)")
PYEOF
  grep -q "防重放" "$PKG_DIR/truth.md"
  grep -q "TestLogin2FAReplay" "$PKG_DIR/tests.md"
  grep -q "replay rejection" "$PKG_DIR/tasks.md"
  echo "four-piece content verified on disk"

  # ==================================================================
  STEP="3b ask + sign intent"
  echo; echo "===== STEP 3b: $STEP ====="
  OUT="$("$BEACON" truth ask --project "$PROJ" --feature "$SLUG" \
    --question "动态码错误、过期、重放分别返回给用户什么？" \
    --recommend "统一返回登录失败，不泄露具体原因；重放按失败计数" --json)"
  echo "$OUT"
  OUT="$("$BEACON" truth intent --project "$PROJ" --feature "$SLUG" \
    --utterance "给登录加 TOTP 二次验证，动态码错误或重放时拒绝登录" \
    --goal "账号密码泄露后仍不能直接登录" \
    --non-goals "不做短信验证码；不改注册流程" \
    --constraints "离线可验证；不引入外部服务" --scope-mode focused --json)"
  echo "$OUT"
  OUT="$("$BEACON" truth answer --project "$PROJ" --feature "$SLUG" --id Q1 \
    --answer "所有失败统一返回登录失败，并计入失败次数" --json)"
  echo "$OUT"
  OUT="$("$BEACON" truth answer --project "$PROJ" --feature "$SLUG" --id Q2 \
    --answer "统一登录失败，不区分过期/错误/重放" --json)"
  echo "$OUT"
  OUT="$("$BEACON" truth confirm --project "$PROJ" --feature "$SLUG" --by e2e-author --json)"
  echo "$OUT"
  grep -q "^intent_confirmed: true" "$PKG_DIR/truth.md" || {
    echo "FAIL: intent_confirmed is not true after confirm"; return 1; }
  echo "intent signed: snapshot + closed questions + per-AC coverage verified"

  # ==================================================================
  STEP="4 truth freeze"
  echo; echo "===== STEP 4: $STEP ====="
  OUT="$("$BEACON" truth freeze --project "$PROJ" --feature "$SLUG" --by e2e-author --json)"
  echo "$OUT"
  grep -q "^status: frozen" "$PKG_DIR/truth.md"
  echo "truth.md frontmatter status=frozen verified"
  # S1 (AC-139): the freeze alone is not yet a consumable
  # baseline — the frozen body is still uncommitted here.
  [ "$(printf '%s' "$OUT" | jval baseline_status)" = "uncommitted" ] || {
    echo "FAIL: freeze should report baseline_status=uncommitted before the canonical commit"; return 1; }
  echo "baseline_status=uncommitted after freeze verified (AC-139)"

  # ==================================================================
  STEP="4b commit frozen package + truth baseline (S1, AC-139)"
  echo; echo "===== STEP 4b: $STEP ====="
  git -C "$PROJ" add docs
  git -C "$PROJ" commit -qm "freeze $SLUG truth package (canonical baseline)"
  OUT="$("$BEACON" truth baseline --project "$PROJ" --feature "$SLUG" --by e2e-author --json)"
  echo "$OUT"
  [ "$(printf '%s' "$OUT" | jval baseline_status)" = "committed" ] || {
    echo "FAIL: truth baseline did not record a committed baseline"; return 1; }
  [ "$(printf '%s' "$OUT" | jval baseline_recorded)" = "True" ] || {
    echo "FAIL: truth baseline did not record the baseline"; return 1; }
  echo "baseline committed + recorded verified (AC-139)"

  # ==================================================================
  STEP="5 gate evaluate truth_freeze"
  echo; echo "===== STEP 5: $STEP ====="
  OUT="$("$BEACON" gate evaluate truth_freeze --project "$PROJ" --feature "$SLUG" \
    --evaluator "$EVALUATOR" --json)"
  echo "$OUT"
  [ "$(printf '%s' "$OUT" | jval verdict)" = "pass" ] || {
    echo "FAIL: truth_freeze gate did not pass"; return 1; }
  echo "truth_freeze verdict=pass verified"

  # ==================================================================
  STEP="5b goal topology project (S3, AC-171/173)"
  echo; echo "===== STEP 5b: $STEP ====="
  printf '{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[{"scope_id":"primary","git_root":"%s","role":"primary","dirty_policy":"fail_closed"}]}' "$PROJ" > "$WORK/graph.json"
  OUT="$("$BEACON" goal topology project --project "$PROJ" --feature "$SLUG" --graph "$WORK/graph.json" --by e2e-author --json)"
  echo "$OUT"
  TOPO_REF="$(printf '%s' "$OUT" | jval topology_gate_ref)"
  case "$TOPO_REF" in bc_*) : ;; *) echo "FAIL: topology_gate_ref is not a bc_ ref: $TOPO_REF"; return 1 ;; esac
  [ "$(printf '%s' "$OUT" | jval topology_gate_status)" = "pass" ] || {
    echo "FAIL: projected topology gate status is not pass on the clean committed tree"; return 1; }
  echo "topology projected: ref=$TOPO_REF gate=pass (AC-171); step 7 admission will consume it (AC-173)"

  # ==================================================================
  STEP="6 goal run (subprocess profile executor, real agent script)"
  echo; echo "===== STEP 6: $STEP ====="
  mkdir -p "$WORK/bin"
  cat > "$AGENT" <<'AGENTEOF'
#!/bin/sh
# E2E stage agent: real work per stage, driven by the harness BEACON_* env
# (BEACON_STAGE / BEACON_RUN_ID / BEACON_CONTEXT_FILE for profile runs).
set -eu
stage="${BEACON_STAGE:-unknown}"
echo "agent stage=$stage run=${BEACON_RUN_ID:-} iteration=${BEACON_ITERATION:-} context=${BEACON_CONTEXT_FILE:-none}"
mkdir -p e2e-work
printf 'stage=%s run=%s\n' "$stage" "${BEACON_RUN_ID:-}" >> e2e-work/agent-output.txt
case "$stage" in
  implement)
    printf 'totp challenge + replay guard implemented for AC-001/AC-002\n' \
      > e2e-work/implementation.txt
    ;;
  qa)
    printf '# QA Report: e2e-login-2fa\n\n- TC-001 TestLogin2FAChallenge: PASS (login rejected without valid TOTP, accepted with valid TOTP)\n- TC-002 TestLogin2FAReplay: PASS (replayed TOTP rejected)\n- AC-001 verified\n- AC-002 verified\n' \
      > e2e-work/qa-report.md
    ;;
esac
echo "agent stage $stage done"
exit 0
AGENTEOF
  chmod +x "$AGENT"
  echo "agent script: $AGENT"
  OUT="$("$BEACON" goal run "给登录加 TOTP 二次验证" \
    --project "$PROJ" --feature "$SLUG" \
    --executor-id "$MAKER" \
    --exec-cmd "$AGENT" --profile codex-style --json)"
  echo "$OUT"
  RUN_ID="$(printf '%s' "$OUT" | jval run_id)"
  echo "run_id=$RUN_ID"
  # No --ux was passed: the stage plan must NOT contain a design stage.
  if printf '%s' "$OUT" | grep -q '"stage": "design"'; then
    echo "FAIL: design stage inserted without --ux"; return 1
  fi
  echo "stage plan has no design stage (no --ux) verified"

  # ==================================================================
  STEP="7 goal admit"
  echo; echo "===== STEP 7: $STEP ====="
  OUT="$("$BEACON" goal admit --run-id "$RUN_ID" --json)"
  echo "$OUT"
  [ "$(printf '%s' "$OUT" | jval allow_implement)" = "True" ] || {
    echo "FAIL: implement admission did not fully pass"; return 1; }
  echo "admit allow_implement=true verified (design_bound not applicable: no --ux)"
  printf '%s' "$OUT" | grep -q '"check": "topology_gate"' || {
    echo "FAIL: admission output lacks the topology_gate item (AC-173)"; return 1; }
  echo "admission topology_gate item present and passing (AC-173, ref $TOPO_REF)"

  # ==================================================================
  STEP="8 driver run-loop to release_gate"
  echo; echo "===== STEP 8: $STEP ====="
  "$BEACON" goal driver start --run-id "$RUN_ID" --max-turns 10 --json
  "$BEACON" goal driver run-loop --run-id "$RUN_ID" \
    --executor subprocess:codex-style --max-rounds 10 --json \
    | tee "$WORK/run-loop.json"
  [ "$(jval final_state < "$WORK/run-loop.json")" = "awaiting_human" ] || {
    echo "FAIL: driver did not stop at release_gate awaiting human"; return 1; }
  [ "$(jval run.loop_stage < "$WORK/run-loop.json")" = "release_gate" ] || {
    echo "FAIL: run is not at release_gate after run-loop"; return 1; }
  echo "driver rounds=$(jval rounds < "$WORK/run-loop.json"), final_state=awaiting_human, loop_stage=release_gate"
  # The real subprocess agent must have worked every stage.
  for s in plan truth implement qa; do
    grep -q "stage=$s " "$PROJ/e2e-work/agent-output.txt" || {
      echo "FAIL: agent did not execute stage $s"; return 1; }
  done
  [ -s "$PROJ/e2e-work/implementation.txt" ] || { echo "FAIL: implement work product missing"; return 1; }
  [ -s "$PROJ/e2e-work/qa-report.md" ] || { echo "FAIL: qa report file missing"; return 1; }
  ls -la "$PROJ/e2e-work" "$PROJ/.beacon/artifacts/$RUN_ID"
  echo "agent work products verified (implementation.txt, qa-report.md, kernel stage captures)"

  # ==================================================================
  STEP="8b goal artifacts register (QA report -> registered evidence)"
  echo; echo "===== STEP 8b: $STEP ====="
  OUT="$("$BEACON" goal artifacts register --run-id "$RUN_ID" \
    --kind qa_report --path "$PROJ/e2e-work/qa-report.md" \
    --by "$EVALUATOR" --json)"
  echo "$OUT"
  [ "$(printf '%s' "$OUT" | jval artifact.kind)" = "qa_report" ] || {
    echo "FAIL: registered artifact kind is not qa_report"; return 1; }
  echo "QA report registered as artifact $(printf '%s' "$OUT" | jval artifact.id) (sha256 recorded by the register face)"
  "$BEACON" goal artifacts list --run-id "$RUN_ID" --json

  # ==================================================================
  STEP="9 gate evaluate design/implement/qa/release"
  echo; echo "===== STEP 9: $STEP ====="
  # eval_gate <type> <expect: pass|blocked|any> — the gate CLI exits 1 on
  # a blocked verdict BY DESIGN; the verdict JSON is the result, recorded
  # as-is. Only an unexpected verdict fails the script.
  eval_gate() {
    local gtype="$1" expect="$2" out rc verdict
    echo "--- gate evaluate $gtype (expect: $expect) ---"
    set +e
    out="$("$BEACON" gate evaluate "$gtype" --project "$PROJ" --feature "$SLUG" \
      --evaluator "$EVALUATOR" --run "$RUN_ID" --json 2>&1)"
    rc=$?
    set -e
    echo "$out"
    echo "gate $gtype cli_rc=$rc"
    verdict="$(printf '%s' "$out" | grep -o '"verdict": "[a-z_]*"' | head -1 | cut -d'"' -f4 || true)"
    echo "gate $gtype verdict=$verdict"
    case "$expect" in
      pass)    [ "$verdict" = "pass" ]    || { echo "FAIL: gate $gtype expected pass"; return 1; } ;;
      blocked) [ "$verdict" = "blocked" ] || { echo "FAIL: gate $gtype expected blocked"; return 1; } ;;
      any)     : ;;
    esac
    LAST_VERDICT="$verdict"
  }
  eval_gate design pass      # non-UX run: not applicable -> pass verdict
  eval_gate implement pass   # same shared admission checker as step 7
  eval_gate qa pass          # registered QA evidence (step 8b) must pass
  eval_gate release blocked  # before completion: must block (missing
                             # human confirmation)
  "$BEACON" gate status --project "$PROJ" --feature "$SLUG" --json

  # ==================================================================
  STEP="10 attach-verifier + complete"
  echo; echo "===== STEP 10: $STEP ====="
  "$BEACON" goal attach-verifier --run-id "$RUN_ID" \
    --source "$CHECKER" --ref "$PROJ/e2e-work/qa-report.md" --json
  echo "verifier attached (source=$CHECKER != maker=$MAKER)"
  set +e
  OUT="$("$BEACON" goal complete --run-id "$RUN_ID" \
    --human-confirmed --confirmed-by lai --json 2>&1)"
  COMPLETE_RC=$?
  set -e
  echo "$OUT"
  echo "complete cli_rc=$COMPLETE_RC"
  if [ "$COMPLETE_RC" -ne 0 ]; then
    echo "FAIL: goal complete failed (rc=$COMPLETE_RC)"
    return 1
  fi
  [ "$(printf '%s' "$OUT" | jval loop_stage)" = "done" ] || {
    echo "FAIL: complete succeeded but run is not at loop_stage=done"; return 1; }
  echo "run completed (loop_stage=done)"

  # ==================================================================
  STEP="11 spine + trace read-back"
  echo; echo "===== STEP 11: $STEP ====="
  "$BEACON" spine show --project "$PROJ" --feature "$SLUG" --json
  "$BEACON" spine list --project "$PROJ" --json
  "$BEACON" goal trace --run-id "$RUN_ID"
  "$BEACON" goal status --run-id "$RUN_ID" --json

  echo "=================================================================="
  echo "E2E RESULT: PASS — full chain completed (run $RUN_ID done)"
  echo "elapsed: $((SECONDS - START_TS))s"
  echo "workspace kept at: $WORK"
  return 0
}

if [ "${E2E_INNER:-}" != "1" ]; then
  E2E_INNER=1 main "$@" 2>&1 | tee "$LOG_FILE"
  exit "${PIPESTATUS[0]}"
fi
main "$@"
