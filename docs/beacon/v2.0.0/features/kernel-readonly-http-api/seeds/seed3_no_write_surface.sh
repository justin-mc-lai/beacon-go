#!/bin/zsh
# Seed 3 (AC-HTTP-007..008): no write surface on the read-only HTTP face.
#
# Against a live loopback server:
#   - POST/PUT/PATCH/DELETE on each of the five known paths -> 405
#     with {"error":{"code":"method_not_allowed","message":...}}
#   - GET/POST on release/publish/complete-style paths -> 404 not_found
#   - runs / feature_packages / gate_results / lifecycle_events counts unchanged
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

PORT=$(pick_port)
BASE="http://127.0.0.1:$PORT"
CANON="$SEED_OUT/seed3.canon"

if ! start_serve "$PORT" --http --addr "127.0.0.1:$PORT"; then
  {
    print -r -- "seed=seed3_no_write_surface"
    print -r -- "serve_started=no"
    print -r -- "serve_exit=${SERVE_EXIT:-?}"
    print -r -- "serve_stderr=$(serve_error_token)"
  } >"$CANON"
  cat "$CANON"
  print -r -- "NOTE: \`goal serve --http\` did not start: $(head -3 "$SERVE_LOG" | tr '\n' ' ')"
  bad "goal serve --http starts on 127.0.0.1:$PORT"
  finish seed3_no_write_surface "$CANON"
fi

state_counts() {
  sqlite3 "$BEACON_STORE_PATH" \
    "select (select count(*) from runs) || '/' || (select count(*) from feature_packages) || '/' || (select count(*) from gate_results) || '/' || (select count(*) from lifecycle_events);"
}
counts_before=$(state_counts)

paths=(
  "/health"
  "/spine/runs"
  "/spine/gates?feature=demo"
  "/runs/$RUN_A"
  "/features/demo"
)
methods=(POST PUT PATCH DELETE)
method_ok=0
method_total=0
for path in $paths; do
  for method in $methods; do
    method_total=$((method_total + 1))
    code=$(http_status "$BASE$path" "$SEED_OUT/method.json" -X "$method")
    body_ok=no
    if jq -e '.error.code == "method_not_allowed" and ((.error | keys | sort | join(",")) == "code,message")' \
      "$SEED_OUT/method.json" >/dev/null 2>&1; then
      body_ok=yes
    fi
    if [ "$code" = "405" ] && [ "$body_ok" = "yes" ]; then
      method_ok=$((method_ok + 1))
    else
      bad "$method $path -> 405 method_not_allowed (got $code, body_ok=$body_ok)"
    fi
  done
done
check_eq "non-GET on known paths all 405" "$method_total" "$method_ok"

write_paths=(
  "/release"
  "/publish"
  "/complete"
  "/runs/$RUN_A/release"
  "/runs/$RUN_A/complete"
  "/features/demo/publish"
)
write_ok=0
write_total=0
for path in $write_paths; do
  for method in GET POST; do
    write_total=$((write_total + 1))
    code=$(http_status "$BASE$path" "$SEED_OUT/write.json" -X "$method")
    body_ok=no
    if jq -e '.error.code == "not_found" and ((.error | keys | sort | join(",")) == "code,message")' \
      "$SEED_OUT/write.json" >/dev/null 2>&1; then
      body_ok=yes
    fi
    if [ "$code" = "404" ] && [ "$body_ok" = "yes" ]; then
      write_ok=$((write_ok + 1))
    else
      bad "$method $path -> 404 not_found (got $code, body_ok=$body_ok)"
    fi
  done
done
check_eq "write/release paths all 404" "$write_total" "$write_ok"

counts_after=$(state_counts)
check_eq "store rows unchanged by requests" "$counts_before" "$counts_after"

stop_serve
check_eq "listener gone after SIGTERM" "closed" "$(port_closed "$PORT" && print -r -- closed || print -r -- open)"

{
  print -r -- "seed=seed3_no_write_surface"
  print -r -- "serve_started=yes"
  print -r -- "methods_405=$method_ok/$method_total"
  print -r -- "write_paths_404=$write_ok/$write_total"
  print -r -- "error_schema=ok"
  print -r -- "state_unchanged=$([ "$counts_before" = "$counts_after" ] && print -r -- yes || print -r -- no)"
  print -r -- "listener_after_shutdown=closed"
} >"$CANON"
cat "$CANON"

finish seed3_no_write_surface "$CANON"
