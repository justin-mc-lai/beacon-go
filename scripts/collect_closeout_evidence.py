#!/usr/bin/env python3
"""Collect real per-leaf closeout reports; never set implementation/verification state."""
import argparse
import hashlib
import json
import pathlib
import subprocess
from datetime import datetime, timezone


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--beacon", required=True)
    args = parser.parse_args()
    root = pathlib.Path(__file__).resolve().parents[1]
    binary = pathlib.Path(args.beacon).resolve()
    source = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    definitions = {
        "n_799afe679d02": ["go", "test", "./internal/gate", "./internal/cli", "-run", "TestFSMNegativeCoverage|TestParseCoverage", "-count=1", "-json"],
        "n_2b2a883e7f64": ["go", "test", "./internal/orchestrator", "./internal/cli", "-run", "TestPerTCVerdict", "-count=1", "-json"],
        "n_97cff751330d": ["go", "test", "./internal/cli", "-run", "TestNodeDeprecate|TestTreeBindingCacheFailureRollsBack|TestTreeBindingRollbackFailureRetainsRecoveryJournal", "-count=1", "-json"],
    }
    output = root / "docs/beacon/v0.0.1/evidence/closeout" / source
    output.mkdir(parents=True, exist_ok=True)
    for node_id, command in definitions.items():
        node = json.loads((root / "docs/beacon/v0.0.1/tree/nodes" / (node_id + ".json")).read_text())
        stdout = output / (node_id + ".stdout.jsonl")
        stderr = output / (node_id + ".stderr.log")
        report = output / (node_id + ".json")
        if any(p.exists() for p in (stdout, stderr, report)):
            raise SystemExit(f"immutable evidence already exists: {node_id}")
        start = datetime.now(timezone.utc).isoformat()
        run = subprocess.run(command, cwd=root, capture_output=True)
        stdout.write_bytes(run.stdout)
        stderr.write_bytes(run.stderr)
        events = [json.loads(line) for line in run.stdout.splitlines() if line.strip()]
        passed = [e for e in events if e.get("Action") == "pass" and e.get("Test")]
        failed = [e for e in events if e.get("Action") in ("fail", "skip") and e.get("Test")]
        summary = {
            "schema": "beacon-closeout-observation.v1",
            "source_commit": source,
            "binary_sha256": digest(binary),
            "node_id": node_id,
            "node_fingerprint": node["fingerprint"],
            "command": command,
            "started_at": start,
            "finished_at": datetime.now(timezone.utc).isoformat(),
            "exit_code": run.returncode,
            "passed_cases": [e["Test"] for e in passed],
            "failed_or_skipped_cases": failed,
            "stdout": {"path": stdout.name, "sha256": digest(stdout)},
            "stderr": {"path": stderr.name, "sha256": digest(stderr)},
            "scope": "Observed behavior regression, not a Red/Green delivery receipt or release approval",
            "state_transition": "none",
            "ci": "https://github.com/justin-mc-lai/beacon-go/actions/runs/46105800430",
        }
        report.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n")
        if run.returncode or not passed or failed:
            raise SystemExit(f"behavior evidence rejected: {node_id}")
        print(f"{node_id}: exit=0 observed_pass_cases={len(passed)} report={report}")


if __name__ == "__main__":
    main()
