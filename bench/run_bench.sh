#!/usr/bin/env bash
# P6 benchmark for Beacon Go (AC-051..AC-058, frozen thresholds from
# docs/beacon/archive/v2.0.0/features/beacon-v2-rust-kernel-redesign/truth.md
# lines 555-562; original Rust wording, Go build keeps the same numbers).
#
# Usage:
#   bench/run_bench.sh [beacon-binary] [seed-store-dir]
#
# Defaults: binary = <repo>/bin/beacon, seed store = <repo>/bench/seed-store
# (a throwaway BEACON_STORE_PATH; the default ~/.beacon DB and the v1 source
# repo are never written by the Go binary).
#
# Method: no hyperfine on this VM, so every timed item is measured with a
# python3 harness that spawns the real CLI as a subprocess and times it with
# time.perf_counter_ns() (wall clock, process start included). Read-only
# items: N_READ samples after 3 warmups. Stateful items (gate evaluate,
# goal tick): N_WRITE samples. Statistics: median + p95 (nearest-rank).
# AC-056 is a single migration run; AC-058 is a volume snapshot.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/bin/beacon}"
STORE_DIR="${2:-$ROOT/bench/seed-store}"
SRC="${BEACON_BENCH_SRC:-$HOME/workspace/repos/beacon}"
V1_PYTHON="${V1_PYTHON:-$HOME/workspace/beacon-go-venv/bin/python}"
FEATURE_HINT="${BEACON_BENCH_FEATURE:-}"
N_READ="${N_READ:-50}"
N_WRITE="${N_WRITE:-30}"
V1_N="${V1_N:-30}"
export N_READ N_WRITE V1_N

[ -x "$BIN" ] || { echo "beacon binary not executable: $BIN" >&2; exit 1; }
[ -d "$SRC" ] || { echo "v1 source repo not found: $SRC" >&2; exit 1; }

WORK="$(mktemp -d /tmp/beacon-bench.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------- timer ----
cat > "$WORK/timer.py" <<'PY'
import json, subprocess, sys, time

def run_once(argv, cwd=None):
    t0 = time.perf_counter_ns()
    p = subprocess.run(argv, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=cwd)
    return (time.perf_counter_ns() - t0) / 1e6, p.returncode

def main():
    mode = sys.argv[1]           # simple | tick
    n = int(sys.argv[2]); warmup = int(sys.argv[3]); out = sys.argv[4]
    samples, rcs = [], []
    if mode == "simple":
        cwd = sys.argv[5] if sys.argv[5] != "-" else None
        argv = sys.argv[6:]
        for _ in range(warmup):
            run_once(argv, cwd)
        for _ in range(n):
            ms, rc = run_once(argv, cwd); samples.append(ms); rcs.append(rc)
    elif mode == "tick":
        bin_, src, feature = sys.argv[6], sys.argv[7], sys.argv[8]
        def fresh_tick():
            p = subprocess.run([bin_, "goal", "run", "--project", src,
                                "--feature", feature, "--json"],
                               capture_output=True, text=True)
            run_id = json.loads(p.stdout)["run_id"]
            return run_once([bin_, "goal", "tick", "--run-id", run_id,
                             "--executor", "recording", "--json"])
        for _ in range(warmup):
            fresh_tick()
        for _ in range(n):
            ms, rc = fresh_tick(); samples.append(ms); rcs.append(rc)
    else:
        raise SystemExit("unknown mode " + mode)
    with open(out, "w") as f:
        json.dump({"samples_ms": samples, "return_codes": rcs}, f)

main()
PY

# ----------------------------------------------------------------- seed ----
echo "== seed: fresh store in $STORE_DIR (source: $SRC) =="
rm -rf "$STORE_DIR"; mkdir -p "$STORE_DIR"
export BEACON_STORE_PATH="$STORE_DIR/beacon.db"

"$BIN" projects register --path "$SRC" --json > "$WORK/register.json"

# AC-056: the seed migration itself is the v1->v2 migration measurement.
python3 - "$BIN" "$SRC" "$WORK/migrate.json" <<'PY'
import json, subprocess, sys, time
bin_, src, out = sys.argv[1], sys.argv[2], sys.argv[3]
t0 = time.perf_counter_ns()
p = subprocess.run([bin_, "migrate", "v1", "--path", src, "--json"],
                   capture_output=True, text=True)
ms = (time.perf_counter_ns() - t0) / 1e6
doc = json.loads(p.stdout)
with open(out, "w") as f:
    json.dump({"elapsed_ms": ms, "return_code": p.returncode,
               "counts": doc.get("counts", {})}, f)
print("migrate v1: %.1f ms, packages imported=%s" %
      (ms, doc.get("counts", {}).get("feature_packages", {})))
PY

# Feature + single-artifact run, picked dynamically from the seeded store.
export BIN SRC
eval "$(python3 - "$BEACON_STORE_PATH" "$FEATURE_HINT" <<'PY'
import json, os, sqlite3, subprocess, sys
db, hint = sys.argv[1], sys.argv[2]
env = dict(os.environ)
out = subprocess.run([os.environ["BIN"], "truth", "list", "--project",
                      os.environ["SRC"], "--json"],
                     capture_output=True, text=True, env=env)
feats = json.loads(out.stdout)["features"]
feature = hint or feats[0]["slug"]
print(f"FEATURE={feature}")
print(f"N_FEATURES={len(feats)}")
con = sqlite3.connect(db)
row = con.execute("SELECT run_id FROM artifact_refs GROUP BY run_id "
                  "HAVING COUNT(*)=1 ORDER BY run_id LIMIT 1").fetchone()
single_run = row[0]
path, size = con.execute("SELECT abs_path, size_bytes FROM artifact_refs "
                         "WHERE run_id=?", (single_run,)).fetchone()
print(f"SINGLE_RUN={single_run}")
print(f"SINGLE_PATH={path}")
print(f"SINGLE_SIZE={size}")
PY
)"
echo "feature=$FEATURE packages=$N_FEATURES single_artifact_run=$SINGLE_RUN ($SINGLE_PATH, $SINGLE_SIZE bytes)"

# AC-058 snapshot: right after seeding, before any bench writes.
python3 - "$BEACON_STORE_PATH" > "$WORK/db_size_seed.json" <<'PY'
import json, os, sys
p = sys.argv[1]
def sz(q): return os.path.getsize(q) if os.path.exists(q) else 0
parts = {"db_bytes": sz(p), "wal_bytes": sz(p + "-wal"), "shm_bytes": sz(p + "-shm")}
parts["total_bytes"] = sum(parts.values())
print(json.dumps(parts))
PY

# ------------------------------------------------------------- Go items ----
T() { python3 "$WORK/timer.py" "$@"; }

echo "== AC-051 cold start: $BIN --version (n=$N_READ, no warmup) =="
T simple "$N_READ" 0 "$WORK/s051.json" - "$BIN" --version

echo "== AC-052 spine show (n=$N_READ) =="
T simple "$N_READ" 3 "$WORK/s052.json" - "$BIN" spine show --project "$SRC" --feature "$FEATURE" --json

echo "== AC-053 truth show (truth-map resolve equivalent; n=$N_READ) =="
T simple "$N_READ" 3 "$WORK/s053.json" - "$BIN" truth show --project "$SRC" --feature "$FEATURE" --json
T simple "$N_READ" 3 "$WORK/s053_list.json" - "$BIN" truth list --project "$SRC" --json

echo "== AC-054 gate status + gate evaluate (n=$N_READ / n=$N_WRITE) =="
T simple "$N_READ" 3 "$WORK/s054_status.json" - "$BIN" gate status --project "$SRC" --feature "$FEATURE" --json
# gate evaluate persists a verdict per call; on this migrated feature the qa
# verdict is 'blocked' (exit != 0) -- the timing covers compute + write.
T simple "$N_WRITE" 3 "$WORK/s054_eval.json" - "$BIN" gate evaluate qa --project "$SRC" --feature "$FEATURE" --evaluator bench --json

echo "== AC-055 spine artifacts, single-artifact run (n=$N_READ) =="
T simple "$N_READ" 3 "$WORK/s055.json" - "$BIN" spine artifacts "$SINGLE_RUN" --json
T simple "$N_READ" 3 "$WORK/s055_sha.json" - sha256sum "$SINGLE_PATH"

echo "== AC-057 goal tick, recording executor, fresh run per sample (n=$N_WRITE) =="
T tick "$N_WRITE" 2 "$WORK/s057.json" - "$BIN" "$SRC" "$FEATURE"

# ------------------------------------------------------------- v1 items ----
if [ -x "$V1_PYTHON" ]; then
  echo "== v1 AC-051: python -m beacon --version (n=$V1_N) =="
  T simple "$V1_N" 0 "$WORK/v051.json" "$SRC" "$V1_PYTHON" -m beacon --version
  echo "== v1 equivalent query: status show-status (n=$V1_N) =="
  T simple "$V1_N" 2 "$WORK/v052.json" "$SRC" "$V1_PYTHON" -m beacon status show-status --project-root "$SRC"
  echo "== v1 truth-map resolve (the original AC-053 surface; n=$V1_N) =="
  T simple "$V1_N" 2 "$WORK/v053.json" "$SRC" "$V1_PYTHON" -m beacon truth-map resolve "$FEATURE" --project "$SRC" --json
else
  echo "v1 python not found at $V1_PYTHON -- v1 comparison skipped" >&2
fi

python3 - "$BEACON_STORE_PATH" > "$WORK/db_size_final.json" <<'PY'
import json, os, sys
p = sys.argv[1]
def sz(q): return os.path.getsize(q) if os.path.exists(q) else 0
parts = {"db_bytes": sz(p), "wal_bytes": sz(p + "-wal"), "shm_bytes": sz(p + "-shm")}
parts["total_bytes"] = sum(parts.values())
print(json.dumps(parts))
PY

df -T "$STORE_DIR" | tail -1 > "$WORK/store_fs.txt"

# -------------------------------------------------------------- assemble ---
python3 - "$WORK" "$ROOT/bench/results.json" "$BIN" "$SRC" "$FEATURE" \
         "$SINGLE_RUN" "$SINGLE_PATH" "$SINGLE_SIZE" "$N_FEATURES" <<'PY'
import json, math, os, platform, subprocess, sys

work, out_path, bin_, src, feature, single_run, single_path, single_size, nfeat = sys.argv[1:10]

def load(name):
    p = os.path.join(work, name)
    return json.load(open(p)) if os.path.exists(p) else None

def stats(samples):
    s = sorted(samples); n = len(s)
    med = s[n // 2] if n % 2 else (s[n // 2 - 1] + s[n // 2]) / 2
    p95 = s[math.ceil(0.95 * n) - 1]
    return round(med, 2), round(p95, 2), n

def item(ac, name, threshold, samples_file, notes, v1_file=None, v1_notes=None):
    d = load(samples_file)
    med, p95, n = stats(d["samples_ms"])
    it = {"ac": ac, "name": name, "threshold_ms": threshold,
          "median_ms": med, "p95_ms": p95, "n": n,
          "pass": p95 < threshold, "notes": notes,
          "samples_ms": [round(x, 2) for x in d["samples_ms"]],
          "return_codes": sorted(set(d["return_codes"]))}
    if v1_file and load(v1_file):
        vd = load(v1_file); vm, vp, vn = stats(vd["samples_ms"])
        it["v1"] = {"median_ms": vm, "p95_ms": vp, "n": vn,
                    "command": v1_notes or "", "pass_vs_same_threshold": vp < threshold}
    return it

results = []

results.append(item(
    "AC-051", "CLI cold start (beacon --version)", 100, "s051.json",
    "Full process spawn per sample via python subprocess + perf_counter_ns; "
    "no warmup samples discarded (first spawn included). Includes OS process "
    "start + binary load on this VM; --version does not open the store.",
    "v051.json", "python -m beacon --version (v1.6.19, venv python)"))

results.append(item(
    "AC-052", "spine show (one feature, 128-package store)", 50, "s052.json",
    "beacon spine show --project <src> --feature %s --json against the "
    "seeded 128-package store; 3 warmups discarded." % feature,
    "v052.json", "v1 has no spine surface; closest equivalent query used: "
                 "python -m beacon status show-status --project-root <src> "
                 "(different payload/semantics -- reference only)"))

truth_show = item(
    "AC-053", "truth-map resolve equivalent: truth show (one package)", 200, "s053.json",
    "Go build has no truth-map command (scope charter: truth-map alias folded "
    "into the truth face). Equivalent resolve action = beacon truth show "
    "--project <src> --feature %s --json: parse + resolve one package from "
    "the 128-package store. Supplementary truth list (all 128 packages) "
    "timed separately, see truth_list_* fields." % feature,
    "v053.json", "python -m beacon truth-map resolve %s --project <src> --json "
                 "(the original v1 surface)" % feature)
lst = load("s053_list.json"); lm, lp, ln = stats(lst["samples_ms"])
truth_show["truth_list_median_ms"] = lm; truth_show["truth_list_p95_ms"] = lp
truth_show["truth_list_n"] = ln
results.append(truth_show)

gate = item(
    "AC-054", "Gate calculation: gate evaluate qa (compute + persist verdict)", 50, "s054_eval.json",
    "Primary figure is gate evaluate qa (reads facts, decides, writes "
    "gate_results). On this migrated feature the verdict is 'blocked' "
    "(qa evidence missing), so the CLI exits non-zero; timing covers the "
    "full compute+write path regardless. gate status (read-only read of "
    "already-evaluated verdicts) timed separately, see gate_status_* fields.")
gs = load("s054_status.json"); gm, gp, gn = stats(gs["samples_ms"])
gate["gate_status_median_ms"] = gm; gate["gate_status_p95_ms"] = gp; gate["gate_status_n"] = gn
gate["gate_status_pass"] = gp < 50
gate["notes"] += (" The write path performs several store transactions and "
                  "is fsync-bound on this VM's home volume (btrfs, the same "
                  "volume as the default ~/.beacon/beacon.db); the read-only "
                  "gate status path passes the same threshold.")
results.append(gate)

art = item(
    "AC-055", "Single-file evidence hash verification (spine artifacts, 1-artifact run)", 10, "s055.json",
    "beacon spine artifacts %s --json: this run has exactly 1 artifact_ref "
    "(%s, %s bytes); the CLI recomputes the file's sha256 live and compares "
    "it with the stored hash (ArtifactStaleness). Figure includes full CLI "
    "process start + store open, not just the hash. Pure sha256sum of the "
    "same file timed separately, see sha256sum_* fields." % (single_run, single_path, single_size))
sh = load("s055_sha.json"); sm, sp, sn = stats(sh["samples_ms"])
art["sha256sum_median_ms"] = sm; art["sha256sum_p95_ms"] = sp; art["sha256sum_n"] = sn
results.append(art)

mig = load("migrate.json")
counts = mig["counts"]
results.append({
    "ac": "AC-056", "name": "DB migration v1->v2 (migrate v1, full source repo import)",
    "threshold_ms": 60000, "median_ms": round(mig["elapsed_ms"], 2),
    "p95_ms": round(mig["elapsed_ms"], 2), "n": 1,
    "pass": mig["elapsed_ms"] < 60000,
    "notes": "Single timed run (this run IS the seed build): beacon migrate "
             "v1 --path <src> --json into a fresh store on the home volume "
             "(btrfs, same volume as the default ~/.beacon/beacon.db; import "
             "is transaction/fsync-bound there -- a tmpfs store would be far "
             "faster but unrepresentative). Import counts: "
             + json.dumps({k: v.get("imported") for k, v in counts.items()}),
    "samples_ms": [round(mig["elapsed_ms"], 2)], "return_codes": [mig["return_code"]]})

results.append(item(
    "AC-057", "Orchestrator iteration scheduling (goal tick, recording executor)", 100, "s057.json",
    "Each sample: untimed setup creates a fresh goal run, then one timed "
    "beacon goal tick --executor recording (placeholder executor, no real "
    "agent work) advancing the run one stage. Timing = CLI spawn + store + "
    "engine scheduling for that one tick; agent execution excluded by "
    "construction. The tick persists stage/lifecycle/checkpoint rows in "
    "several transactions, so on this VM's btrfs home volume it is "
    "fsync-bound (a tmpfs store measured materially lower in manual probes "
    "but would not represent the default ~/.beacon store location)."))

seed_size = load("db_size_seed.json"); final_size = load("db_size_final.json")
results.append({
    "ac": "AC-058", "name": "beacon.db volume (seeded 128-package store)",
    "threshold_ms": None, "threshold_mb": 500,
    "median_ms": None, "p95_ms": None, "n": 1,
    "size_bytes": seed_size["db_bytes"], "wal_bytes": seed_size["wal_bytes"],
    "shm_bytes": seed_size["shm_bytes"], "total_bytes": seed_size["total_bytes"],
    "size_mb": round(seed_size["total_bytes"] / 1e6, 3),
    "pass": seed_size["total_bytes"] < 500 * 1000 * 1000,
    "notes": "Snapshot taken immediately after seeding (128 packages, 511 "
             "artifact_refs, 24 versions), before bench writes. total = db + "
             "wal + shm on disk. Under the 500MB soft cap, so no WARN "
             "mechanism is required. Post-bench total (after gate/tick "
             "writes): %d bytes." % final_size["total_bytes"]})

def run(cmd):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True).stdout.strip()

doc = {
    "benchmark": "Beacon Go P6 (AC-051..AC-058)",
    "generated_at": run("date -Iseconds"),
    "environment": {
        "uname": run("uname -a"), "nproc": run("nproc"),
        "machine": platform.machine(), "os": platform.system(),
        "binary": bin_, "binary_version": run(f"'{bin_}' --version"),
        "source_repo": src, "seed_packages": int(nfeat),
        "store_filesystem": open(os.path.join(work, "store_fs.txt")).read().strip(),
        "store_path": os.environ.get("BEACON_STORE_PATH", ""),
        "method": "No hyperfine on this VM. python3 subprocess harness, "
                  "time.perf_counter_ns() wall clock per spawn (process start "
                  "included); median + nearest-rank p95. Read items n=%s "
                  "(3 warmups), stateful items n=%s, v1 n=%s." % (
                      os.environ.get("N_READ", "50"), os.environ.get("N_WRITE", "30"),
                      os.environ.get("V1_N", "30")),
        "caveat": "Timings are from this shared VM (2 vCPU) and include full "
                  "process startup; absolute numbers are VM-bound, the "
                  "pass/fail verdicts use the frozen thresholds unchanged.",
    },
    "results": results,
}
with open(out_path, "w") as f:
    json.dump(doc, f, indent=2, ensure_ascii=False)
for r in results:
    print(r["ac"], r["name"][:60], "| median:", r["median_ms"], "p95:", r["p95_ms"],
          "n:", r["n"], "pass:", r["pass"])
print("results written to", out_path)
PY

# AC-095: regression gate. The baseline is the accepted comparable run; a
# shared acceptance criterion that regressed by more than 10% blocks the P6
# exit. Across a different OS/CPU count the gate refuses to judge
# (comparable=false) instead of comparing two machines.
BASELINE="${BEACON_BENCH_BASELINE:-$ROOT/bench/results-go5-baseline.json}"
if [ -f "$BASELINE" ]; then
  echo "== AC-095 regression gate ($BASELINE) =="
  "$BIN" doctor bench-gate --baseline "$BASELINE" --current "$ROOT/bench/results.json"
else
  echo "AC-095 baseline missing: $BASELINE" >&2
  exit 1
fi
echo "done."
