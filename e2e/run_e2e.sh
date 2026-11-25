#!/usr/bin/env bash
# Beacon Go unreleased candidate — end-to-end full-chain script
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
# directory per run. The default store (~/.beacon/beacon-go.db) is never
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
LOG_FILE="${BEACON_E2E_LOG:-$E2E_DIR/last_run.log}"

# ---- tee plumbing: run main() once, mirror everything to the log ----
main() {
  local START_TS=$SECONDS
  STEP="setup"
  echo "=================================================================="
  echo "Beacon Go P6 E2E — full chain"
  echo "started: $(date '+%F %T %Z')"
  echo "beacon:  $BEACON"
  "$BEACON" --version
  "$BEACON" --version | grep -qE '^Beacon CLI v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$' || {
    echo "FAIL: --version is not SemVer (vX.Y.Z or vX.Y.Z-rc.N)"
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
  export GIT_AUTHOR_EMAIL=e2e@local GIT_AUTHOR_NAME=e2e GIT_COMMITTER_EMAIL=e2e@local GIT_COMMITTER_NAME=e2e
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
# Freeze precheck requires an explicit surface declaration. This is
# the fixture's own contract (a CLI-only demo), not a relaxation of
# the gate: the surface list is real and stays in the frozen body.
if "\nsurfaces:" not in t:
    assert "status: draft\n" in t, "truth.md skeleton status line not found"
    t = t.replace("status: draft\n", "status: draft\nsurfaces: [cli]\nsurface_strict: true\n", 1)
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
# restatement gate: every AC must point into a signed five-section
# restatement beside the truth document (real gate, not waived).
t = t.replace("则登录成功 |", "则登录成功 restates=1 |", 1)
t = t.replace("（防重放） |", "（防重放） restates=2 |", 1)
p.write_text(t)

rest_body = ("## 1 交付形态\n\n第一条 AC 的交付形态。\n\n"
             "## 2 界面与交互\n\n第二条 AC 的交互口径。\n\n"
             "## 3 业务流转\n\n登录校验顺序。\n\n"
             "## 4 边界与不做\n\n不做短信验证码。\n\n"
             "## 5 歧义与默认值\n\n失败统一提示。\n")
import hashlib
rest = ("---\nslug: e2e-login-2fa\nconfirmed_by: e2e-author\n"
        "confirmed_at: 2026-11-13T00:00:00Z\n"
        "hash: " + hashlib.sha256(rest_body.encode()).hexdigest() + "\n---\n" + rest_body)
(d / "restatement.md").write_text(rest)


p = d / "tests.md"; t = p.read_text()
old_tc = "| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | ac_001_holds |"
new_tc = ("| AC-001 | TC-001 | behavior | `sh e2e-work/tc-ac001.sh` | login_rejected_without_valid_totp_and_accepted_with_valid_totp |\n"
          "| AC-002 | TC-002 | behavior | `sh e2e-work/tc-ac002.sh` | replayed_totp_rejected |")
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
  grep -q "tc-ac002.sh" "$PKG_DIR/tests.md"
  grep -q "replay rejection" "$PKG_DIR/tasks.md"
  echo "four-piece content verified on disk"
  # Real TC commands the qa autotest face can execute later: each one
  # asserts the implement stage's work product actually contains its AC.
  mkdir -p "$PROJ/e2e-work"
  cat > "$PROJ/e2e-work/tc-ac001.sh" <<'TCEOF'
#!/bin/sh
set -eu
[ -s e2e-work/implementation.txt ] || { echo "no implementation" >&2; exit 1; }
grep -q "AC-001" e2e-work/implementation.txt
echo "TC-001 ok: AC-001 implemented"
python3 e2e-work/machine-proof.py
TCEOF
  cat > "$PROJ/e2e-work/tc-ac002.sh" <<'TCEOF'
#!/bin/sh
set -eu
[ -s e2e-work/implementation.txt ] || { echo "no implementation" >&2; exit 1; }
grep -q "AC-002" e2e-work/implementation.txt
echo "TC-002 ok: AC-002 implemented"
python3 e2e-work/machine-proof.py
TCEOF
  chmod +x "$PROJ/e2e-work/tc-ac001.sh" "$PROJ/e2e-work/tc-ac002.sh"
  cat > "$PROJ/e2e-work/machine-proof.py" <<'PROOFEOF'
import json, os, pathlib
text = pathlib.Path('e2e-work/implementation.txt').read_text()
assert os.environ['BEACON_AC_ID'] in text
pathlib.Path(os.environ['BEACON_TEST_RESULT_PATH']).write_text(json.dumps({
    'schema': 'beacon-test-execution.v1', 'run_id': os.environ['BEACON_RUN_ID'],
    'tc': os.environ['BEACON_TC_ID'], 'candidate_hash': os.environ['BEACON_QA_CANDIDATE_HASH'],
    'cases': [{'name': os.environ['BEACON_TC_ID'] + '-implementation', 'status': 'passed', 'assertions': 1}]}))
PROOFEOF
  echo "executable fixture TCs written (tc-ac001.sh, tc-ac002.sh)"

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
  STEP="3c truth intent record (kernel-driven runs need the aligned briefing)"
  echo; echo "===== STEP 3c: $STEP ====="
  # intent-contract-bridge AC-002: a kernel-driven run (subprocess
  # profile here) cannot start without a confirmed intent.json beside
  # truth.md. The fixture records its own aligned briefing.
  cat > "$WORK/briefing.json" <<BRIEFEOF
{
  "original_prompt": "给登录加 TOTP 二次验证，动态码错误或重放时拒绝登录",
  "restated_intent": "给已注册用户提供 TOTP 二次验证：密码通过后还必须提供当前窗口有效动态码，错误或重放一律拒绝登录。",
  "identified_risks": ["动态码重放", "时钟漂移导致误拒"],
  "boundaries": ["不做短信验证码", "不改注册流程", "不引入外部服务"],
  "proposed_slugs": ["e2e-login-2fa"],
  "confirmed": true,
  "degraded": false
}
BRIEFEOF
  OUT="$("$BEACON" truth intent record --project "$PROJ" --feature "$SLUG" \
    --briefing "$WORK/briefing.json" --by e2e-author --json)"
  echo "$OUT"
  [ -s "$PKG_DIR/intent.json" ] || { echo "FAIL: intent.json not recorded beside truth.md"; return 1; }
  grep -q '"confirmed": true' "$PKG_DIR/intent.json" || {
    echo "FAIL: recorded intent.json is not confirmed"; return 1; }
  echo "intent.json recorded and confirmed (kernel-driven run may start)"

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
    printf 'def login(password_ok, totp_ok, replayed):\n    return password_ok and totp_ok and not replayed\n' > e2e-work/implementation.py
    git add e2e-work
    git commit -qm 'implement fixture login guard'
    ;;
  qa)
    echo 'QA stage consumes the registered independent product report'
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
  STEP="8 execute plan/truth/implement before independent QA"
  echo; echo "===== STEP 8: $STEP ====="
  for s in plan truth implement; do
    "$BEACON" goal tick --run-id "$RUN_ID" --executor subprocess:codex-style --json
  done
  # The real subprocess agent must have worked every stage.
  for s in plan truth implement; do
    grep -q "stage=$s " "$PROJ/e2e-work/agent-output.txt" || {
      echo "FAIL: agent did not execute stage $s"; return 1; }
  done
  [ -s "$PROJ/e2e-work/implementation.txt" ] || { echo "FAIL: implement work product missing"; return 1; }
  ls -la "$PROJ/e2e-work" "$PROJ/.beacon/artifacts/$RUN_ID"
  echo "agent implementation and kernel stage captures verified"

  # ==================================================================
  STEP="8b goal artifacts register current source candidate"
  echo; echo "===== STEP 8b: $STEP ====="
  PROJ="$PROJ" PKG_DIR="$PKG_DIR" RUN_ID="$RUN_ID" MAKER="$MAKER" python3 - <<'CANDIDATEEOF'
import hashlib,json,os,pathlib,subprocess
r=pathlib.Path(os.environ['PROJ']);pkg=pathlib.Path(os.environ['PKG_DIR'])
sha=lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
c={'schema':'beacon-qa-candidate.v1','run_id':os.environ['RUN_ID'],'feature_slug':'e2e-login-2fa',
   'truth_sha256':sha(pkg/'truth.md'),'tests_sha256':sha(pkg/'tests.md'),
   'source_commit':subprocess.check_output(['git','-C',str(r),'rev-parse','HEAD'],text=True).strip(),
   'source_files':[{'path':'e2e-work/implementation.py','sha256':sha(r/'e2e-work/implementation.py')}],
   'build_command':'fixture Python source execution; no compiler','maker':os.environ['MAKER'],
   'artifacts':[{'path':str(r/'e2e-work/implementation.py'),'sha256':sha(r/'e2e-work/implementation.py')}]}
(r/'candidate.json').write_text(json.dumps(c))
CANDIDATEEOF
  OUT="$("$BEACON" goal artifacts register --run-id "$RUN_ID" \
    --kind qa_candidate --path "$PROJ/candidate.json" --by "$MAKER" --json)"
  echo "$OUT"
  [ "$(printf '%s' "$OUT" | jval artifact.kind)" = "qa_candidate" ] || {
    echo "FAIL: registered artifact kind is not qa_candidate"; return 1; }
  echo "candidate registered as artifact $(printf '%s' "$OUT" | jval artifact.id)"
  "$BEACON" goal artifacts list --run-id "$RUN_ID" --json

  # ==================================================================
  STEP="8c qa autotest (real TC commands -> autotest evidence)"
  echo; echo "===== STEP 8c: $STEP ====="
  OUT="$("$BEACON" verify autotest run --run-id "$RUN_ID" --by "$EVALUATOR" --json)"
  echo "$OUT"
  [ -s "$PROJ/$(printf '%s' "$OUT" | jval report_path)" ] || {
    echo "FAIL: autotest report missing"; return 1; }
  printf '%s' "$OUT" | grep -q '"failed": 0' || {
    echo "FAIL: autotest reported failing TCs"; return 1; }
  echo "autotest executed the fixture TCs for real; evidence at .beacon/autotest/$RUN_ID"

  # ==================================================================
  STEP="8d surface evidence: CLI e2e report"
  echo; echo "===== STEP 8d: $STEP ====="
  # The feature declares surfaces: [cli]; the qa surface check wants a
  # registered e2e_report artifact for it. This report is produced by
  # the chain that just ran (real steps, real exit codes), not typed in.
  cat > "$WORK/e2e-report.md" <<REPEOF
# CLI E2E report — e2e-login-2fa

- verdict: PASS
- run: $RUN_ID
- checker: $EVALUATOR
- chain: steps 1,2,3,3b,3c,4,4b,5,5b,6,7,8,8b,8c completed in this isolated workspace
- TC-001: \`sh e2e-work/tc-ac001.sh\` exit 0 (AC-001)
- TC-002: \`sh e2e-work/tc-ac002.sh\` exit 0 (AC-002)
- surface: cli (one binary drives the whole chain)
REPEOF
  OUT="$("$BEACON" goal artifacts register --run-id "$RUN_ID" \
    --kind e2e_report --path "$WORK/e2e-report.md" --by "$EVALUATOR" --json)"
  echo "$OUT"
  [ "$(printf '%s' "$OUT" | jval artifact.kind)" = "e2e_report" ] || {
    echo "FAIL: e2e report was not registered as e2e_report"; return 1; }
  echo "CLI surface evidence registered"
  PROJ="$PROJ" WORK="$WORK" RUN_ID="$RUN_ID" MAKER="$MAKER" CHECKER="$CHECKER" python3 - <<'REVIEWEOF'
import hashlib,json,os,pathlib,sys
r=pathlib.Path(os.environ['PROJ']);sys.path.insert(0,str(r/'e2e-work'))
from implementation import login
assert not login(True,False,False)
assert login(True,True,False)
assert not login(True,True,True)
c=hashlib.sha256((r/'candidate.json').read_bytes()).hexdigest()
(r/'product-qa.json').write_text(json.dumps({'schema':'beacon-product-qa.v1','run_id':os.environ['RUN_ID'],
 'candidate_hash':c,'scope':'product','verdict':'pass','maker':os.environ['MAKER'],'checker':os.environ['CHECKER'],
 'covered_tcs':['TC-001','TC-002'],'evidence_refs':[str(pathlib.Path(os.environ['WORK'])/'e2e-report.md')]}))
REVIEWEOF
  "$BEACON" goal artifacts register --run-id "$RUN_ID" --kind qa_report --path "$PROJ/product-qa.json" --by "$CHECKER" --json
  "$BEACON" goal attach-verifier --run-id "$RUN_ID" --source "$CHECKER" --ref "$PROJ/product-qa.json" --json
  "$BEACON" gate evaluate qa --project "$PROJ" --feature "$SLUG" --run "$RUN_ID" --evaluator "$CHECKER" --json
  "$BEACON" goal driver start --run-id "$RUN_ID" --max-turns 10 --json
  "$BEACON" goal driver run-loop --run-id "$RUN_ID" --executor subprocess:codex-style --max-rounds 10 --json | tee "$WORK/run-loop.json"
  [ "$(jval final_state < "$WORK/run-loop.json")" = "awaiting_human" ] || { echo "FAIL: driver did not reach human gate"; return 1; }
  [ "$(jval run.loop_stage < "$WORK/run-loop.json")" = "release_gate" ] || { echo "FAIL: run not at release gate"; return 1; }

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
    --source "$CHECKER" --ref "$PROJ/product-qa.json" --json
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

  STEP="12 empty machine execution cannot inherit earlier QA PASS"
  printf 'pass\n' > "$PROJ/e2e-work/machine-proof.py"
  if "$BEACON" verify autotest run --run-id "$RUN_ID" --by "$MAKER" --json; then
    echo "FAIL: exit-0 empty runner was accepted"; return 1
  fi
  if "$BEACON" gate evaluate qa --project "$PROJ" --feature "$SLUG" --run "$RUN_ID" --evaluator "$CHECKER" --json; then
    echo "FAIL: failed latest invocation inherited previous product PASS"; return 1
  fi
  [ "$("$BEACON" goal status --run-id "$RUN_ID" --json | jval status)" = "completed" ] || {
    echo "FAIL: new evaluation rewrote completed history"; return 1; }

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
