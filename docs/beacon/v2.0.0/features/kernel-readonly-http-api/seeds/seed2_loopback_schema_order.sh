#!/bin/zsh
# Seed 2 (AC-HTTP-002..006): loopback-only bind plus the five read endpoints.
#
# Starts `beacon goal serve --http --addr 127.0.0.1:<free port>` against a
# fixture with two runs and two gates, then asserts:
#   - exactly one LISTEN socket, on 127.0.0.1:<port> (never 0.0.0.0 / * / ::)
#   - GET /health                 keys {status, version, read_only}
#   - GET /spine/runs             keys {runs, count}; run keys fixed;
#                                 order started_at desc, run_id asc
#   - GET /spine/gates?feature=   keys {feature, gates}; order evaluated_at desc
#   - GET /runs/{id}              keys {run}; run keys fixed
#   - GET /features/{slug}        keys {feature}; feature keys fixed
#   - unknown id/slug and missing feature -> 404/400 with {error:{code,message}}
# Default --addr (127.0.0.1:8787) is checked when that port is free; when the
# host already listens there the canon records default_addr=skipped.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

PORT=$(pick_port)
BASE="http://127.0.0.1:$PORT"
CANON="$SEED_OUT/seed2.canon"

if ! start_serve "$PORT" --http --addr "127.0.0.1:$PORT"; then
  {
    print -r -- "seed=seed2_loopback_schema_order"
    print -r -- "serve_started=no"
    print -r -- "serve_exit=${SERVE_EXIT:-?}"
    print -r -- "serve_stderr=$(serve_error_token)"
  } >"$CANON"
  cat "$CANON"
  print -r -- "NOTE: \`goal serve --http\` did not start: $(head -3 "$SERVE_LOG" | tr '\n' ' ')"
  bad "goal serve --http starts on 127.0.0.1:$PORT"
  finish seed2_loopback_schema_order "$CANON"
fi

listeners=$(lsof -nP -iTCP:$PORT -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $9}' | sort)
listener_count=$(print -r -- "$listeners" | grep -c . || true)
bind_ok=no
if [ "$listener_count" = "1" ] && [ "$listeners" = "127.0.0.1:$PORT" ]; then
  bind_ok=yes
fi
check_eq "binds only 127.0.0.1:$PORT (one socket)" "yes" "$bind_ok"

hdr="$SEED_OUT/health.hdr"
code=$(curl -sS -m 3 -D "$hdr" -o "$SEED_OUT/health.json" -w '%{http_code}' "$BASE/health" 2>/dev/null || print -r -- 000)
check_eq "GET /health status" "200" "$code"
ct_ok=no
grep -qi '^content-type: application/json' "$hdr" 2>/dev/null && ct_ok=yes
check_eq "GET /health content-type json" "yes" "$ct_ok"
check_keys "/health" "$SEED_OUT/health.json" "read_only,status,version"
check_jq "/health status=ok" "$SEED_OUT/health.json" '.status == "ok"'
check_jq "/health read_only=true" "$SEED_OUT/health.json" '.read_only == true'
check_jq "/health version non-empty" "$SEED_OUT/health.json" '(.version | type) == "string" and (.version | length) > 0'

code=$(http_status "$BASE/spine/runs" "$SEED_OUT/runs.json")
check_eq "GET /spine/runs status" "200" "$code"
check_keys "/spine/runs" "$SEED_OUT/runs.json" "count,runs"
check_jq "/spine/runs count == len(runs)" "$SEED_OUT/runs.json" '.count == (.runs | length)'
check_jq "/spine/runs has both fixture runs" "$SEED_OUT/runs.json" '([.runs[].run_id] | sort) == (["'$RUN_A'", "'$RUN_B'"] | sort)'
check_jq "/spine/runs run keys fixed" "$SEED_OUT/runs.json" 'all(.runs[]; (keys | sort | join(",")) == "blocked,current_stage,feature,run_id,status")'
check_jq "/spine/runs order started_at desc, run_id asc" "$SEED_OUT/runs.json" '.runs as $r | ($r | length) >= 2 and all(range(0; ($r | length) - 1); . as $i | ($r[$i].started_at > $r[$i+1].started_at) or (($r[$i].started_at == $r[$i+1].started_at) and ($r[$i].run_id < $r[$i+1].run_id)))'

code=$(http_status "$BASE/spine/gates?feature=demo" "$SEED_OUT/gates.json")
check_eq "GET /spine/gates status" "200" "$code"
check_keys "/spine/gates" "$SEED_OUT/gates.json" "feature,gates"
check_jq "/spine/gates feature=demo" "$SEED_OUT/gates.json" '.feature == "demo"'
check_jq "/spine/gates has two fixture gates" "$SEED_OUT/gates.json" '(.gates | length) == 2'
check_jq "/spine/gates gate keys fixed" "$SEED_OUT/gates.json" 'all(.gates[]; (keys | sort | join(",")) == "evaluated_at,gate_id,gate_type,reason_codes,verdict")'
check_jq "/spine/gates order evaluated_at desc" "$SEED_OUT/gates.json" '[.gates[].evaluated_at] == ([.gates[].evaluated_at] | sort | reverse)'

code=$(http_status "$BASE/spine/gates" "$SEED_OUT/gates-missing.json")
check_eq "GET /spine/gates without feature -> 400" "400" "$code"
check_keys "error missing feature" "$SEED_OUT/gates-missing.json" "error"
check_jq "error code bad_request" "$SEED_OUT/gates-missing.json" '.error.code == "bad_request" and ((.error | keys | sort | join(",")) == "code,message")'

code=$(http_status "$BASE/runs/$RUN_A" "$SEED_OUT/run.json")
check_eq "GET /runs/{id} status" "200" "$code"
check_keys "/runs/{id}" "$SEED_OUT/run.json" "run"
check_keys "/runs/{id}.run" <(jq '.run' "$SEED_OUT/run.json") "blocked,current_stage,feature,iteration,run_id,started_at,status"
check_jq "/runs/{id} echoes id" "$SEED_OUT/run.json" '.run.run_id == "'"$RUN_A"'"'
check_jq "/runs/{id} feature=demo" "$SEED_OUT/run.json" '.run.feature == "demo"'
check_jq "/runs/{id} types" "$SEED_OUT/run.json" '(.run.blocked | type) == "boolean" and (.run.iteration | type) == "number" and (.run.started_at | type) == "string" and ((.run.current_stage == null) or ((.run.current_stage | type) == "string"))'

code=$(http_status "$BASE/runs/run_missing" "$SEED_OUT/run-missing.json")
check_eq "GET /runs/unknown -> 404" "404" "$code"
check_jq "unknown run not_found" "$SEED_OUT/run-missing.json" '.error.code == "not_found"'

code=$(http_status "$BASE/features/demo" "$SEED_OUT/feature.json")
check_eq "GET /features/{slug} status" "200" "$code"
check_keys "/features/{slug}" "$SEED_OUT/feature.json" "feature"
check_keys "/features/{slug}.feature" <(jq '.feature' "$SEED_OUT/feature.json") "slug,status,tests_path,truth_path"
check_jq "/features/{slug} fields" "$SEED_OUT/feature.json" '.feature.slug == "demo" and (.feature.status | type) == "string" and (.feature.truth_path | endswith("/demo/truth.md")) and (.feature.tests_path | endswith("/demo/tests.md"))'

code=$(http_status "$BASE/features/no-such-feature" "$SEED_OUT/feature-missing.json")
check_eq "GET /features/unknown -> 404" "404" "$code"
check_jq "unknown feature not_found" "$SEED_OUT/feature-missing.json" '.error.code == "not_found"'

stop_serve

default_addr=skipped
if port_closed 8787; then
  if start_serve 8787 --http; then
    default_addr=verified
    check_eq "default --addr listens on 127.0.0.1:8787 only" \
      "127.0.0.1:8787" "$(lsof -nP -iTCP:8787 -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $9}' | sort | tr '\n' ' ' | sed 's/ $//')"
    stop_serve
  else
    default_addr=failed
    bad "goal serve --http (default addr) starts on 127.0.0.1:8787"
  fi
else
  print -r -- "NOTE: port 8787 already has a listener; default-addr sub-check skipped"
fi

{
  print -r -- "seed=seed2_loopback_schema_order"
  print -r -- "serve_started=yes"
  print -r -- "bind=loopback_only"
  print -r -- "health=ok"
  print -r -- "runs_schema=ok"
  print -r -- "runs_count=2"
  print -r -- "runs_order=started_at_desc_run_id_asc"
  print -r -- "gates_schema=ok"
  print -r -- "gates_count=2"
  print -r -- "gates_order=evaluated_at_desc"
  print -r -- "run_detail=ok"
  print -r -- "feature_detail=ok"
  print -r -- "default_addr=$default_addr"
} >"$CANON"
cat "$CANON"

finish seed2_loopback_schema_order "$CANON"
