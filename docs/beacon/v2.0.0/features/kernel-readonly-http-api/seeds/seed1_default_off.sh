#!/bin/zsh
# Seed 1 (AC-HTTP-001): `beacon goal serve` without --http is default-off.
#
# Contract: exits 0 within 5s, prints a disabled notice (a line containing
# "disabled"), and adds no TCP listener. Port probe: capture the listeners on
# the default port 8787 before and after with lsof; a new pid means the command
# started listening. A second probe uses curl http://127.0.0.1:8787/health:
# curl exit 7 / status 000 means nothing answers on that port.
#
# Today: `serve` is not a goal subcommand, so cobra prints the goal help, exits
# 0 and never prints a disabled notice -- the seed fails on the notice check.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"

DEFAULT_PORT=8787
before_pids=$(lsof -nP -iTCP:$DEFAULT_PORT -sTCP:LISTEN -t 2>/dev/null | sort -n | tr '\n' ' ' || true)
before_probe=open
port_closed "$DEFAULT_PORT" && before_probe=closed

started=$(date +%s)
"$BEACON_BIN" goal serve >"$SEED_OUT/seed1.out" 2>"$SEED_OUT/seed1.err" &
pid=$!
for i in {1..50}; do
  kill -0 "$pid" 2>/dev/null || break
  sleep 0.1
done
timed_out=no
if kill -0 "$pid" 2>/dev/null; then
  timed_out=yes
  kill -9 "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  serve_exit=124
else
  serve_exit=0
  wait "$pid" 2>/dev/null || serve_exit=$?
fi
elapsed=$(( $(date +%s) - started ))

after_pids=$(lsof -nP -iTCP:$DEFAULT_PORT -sTCP:LISTEN -t 2>/dev/null | sort -n | tr '\n' ' ' || true)
new_pids=""
for p in ${=after_pids}; do
  case " $before_pids " in
    *" $p "*) ;;
    *) new_pids="$new_pids $p" ;;
  esac
done
new_listener=no
[ -z "$new_pids" ] || new_listener=yes

notice=no
if grep -qi 'disabled' "$SEED_OUT/seed1.out" "$SEED_OUT/seed1.err" 2>/dev/null; then
  notice=yes
fi

CANON="$SEED_OUT/seed1.canon"
{
  print -r -- "seed=seed1_default_off"
  print -r -- "serve_exit=$serve_exit"
  print -r -- "timed_out=$timed_out"
  print -r -- "disabled_notice=$notice"
  print -r -- "port=$DEFAULT_PORT"
  print -r -- "probe_before=$before_probe"
  print -r -- "new_listener=$new_listener"
} >"$CANON"
cat "$CANON"

check_eq "serve exits 0" "0" "$serve_exit"
check_eq "serve exits within 5s" "no" "$timed_out"
check_eq "prints a disabled notice" "yes" "$notice"
check_eq "adds no listener on $DEFAULT_PORT" "no" "$new_listener"
if [ "$notice" = "no" ]; then
  print -r -- 'NOTE: output carried no disabled notice; goal serve is not a known subcommand at this HEAD'
fi

finish seed1_default_off "$CANON"
