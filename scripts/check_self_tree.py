#!/usr/bin/env python3
"""Read-only canonical tree check using the real Beacon rebuild entry."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sqlite3
import subprocess
import tempfile


def load(path):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"duplicate JSON key: {key} in {path}")
            result[key] = value
        return result
    return json.loads(path.read_text(), object_pairs_hook=unique)


def snapshot(tree):
    return {str(p.relative_to(tree)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(tree.rglob("*")) if p.is_file()}


def check(project, version, beacon, scratch):
    tree = project / "docs" / "beacon" / version / "tree"
    before = snapshot(tree)
    if not before:
        raise ValueError(f"missing or empty canonical tree: {tree}")
    try:
        idx = load(tree / "index.json")
        nodes = idx["nodes"]
        if idx["version"] != version or not nodes:
            raise ValueError("index version mismatch or empty nodes")
        ids = [n["id"] for n in nodes]
        if any(not re.fullmatch(r"[A-Za-z0-9_-]+", i) for i in ids):
            raise ValueError("invalid or blank node id")
        if len(ids) != len(set(ids)):
            raise ValueError("duplicate node id")
        bodies = {i: load(tree / "nodes" / f"{i}.json") for i in ids}
        fields = ("id", "parent_id", "kind", "path", "status", "title",
                  "fingerprint", "fingerprint_alg")
        for entry in nodes:
            body = bodies[entry["id"]]
            for field in fields:
                if entry.get(field, "") != body.get(field, ""):
                    raise ValueError(f"index/body mismatch: {entry['id']}.{field}")

        # Ignore the caller's store, even when it points at a real user ledger.
        store = scratch / "store.db"
        env = dict(os.environ, BEACON_STORE_PATH=str(store))

        def run(*args):
            result = subprocess.run([str(beacon), *args], env=env, cwd=project,
                                    text=True, capture_output=True)
            if result.returncode:
                raise ValueError(f"{' '.join(args)} exited {result.returncode}: "
                                 f"{result.stderr.strip()}")
            return json.loads(result.stdout)

        registered = run("projects", "register", "--path", str(project),
                         "--version", version, "--json")
        if registered["current_version"] != version:
            raise ValueError("register returned a different current_version")
        rebuilt = run("doctor", "rebuild-tree", "--project", str(project),
                      "--version", version, "--json")
        if type(rebuilt.get("rebuilt")) is not int or rebuilt["rebuilt"] != len(nodes):
            raise ValueError(f"rebuilt count mismatch: {rebuilt}, expected {len(nodes)}")
        if rebuilt.get("version") != version:
            raise ValueError("rebuild returned a different version")
        versions = [v["id"] for v in registered["versions"] if v["version"] == version]
        if len(versions) != 1:
            raise ValueError("registered version scope is not unique")
        # CLI processes have closed/checkpointed this disposable DB. immutable
        # also avoids creating WAL/SHM files during the read-only comparison.
        with sqlite3.connect(f"file:{store}?mode=ro&immutable=1", uri=True) as db:
            rows = db.execute(f"SELECT {', '.join(fields)} FROM tree_nodes "
                              "WHERE project_version_id = ? ORDER BY id",
                              (versions[0],)).fetchall()
        expected = sorted(tuple(bodies[i].get(f, "") for f in fields) for i in ids)
        actual = [tuple("" if v is None else v for v in row) for row in rows]
        if actual != expected:
            raise ValueError("SQLite projection differs from canonical nodes")
        print(f"self-tree: PASS version={version} indexed={len(nodes)} "
              f"rebuilt={rebuilt['rebuilt']} projected={len(rows)}")
    finally:
        if snapshot(tree) != before:
            raise ValueError("canonical files changed during check")
        print(f"self-tree: canonical unchanged ({len(before)} files)")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", default=str(Path(__file__).resolve().parent.parent))
    parser.add_argument("--version", required=True, help="explicit docs version")
    parser.add_argument("--beacon", help="existing binary; otherwise build in a temporary directory")
    args = parser.parse_args()
    if not re.fullmatch(r"v?[0-9]+\.[0-9]+\.[0-9]+", args.version):
        parser.error("--version must be an explicit docs version (x.y.z or vx.y.z)")
    source = Path(__file__).resolve().parent.parent
    try:
        with tempfile.TemporaryDirectory(prefix="beacon-self-tree-") as tmp:
            scratch = Path(tmp).resolve()
            beacon = Path(args.beacon).resolve() if args.beacon else scratch / "beacon"
            if not args.beacon:
                subprocess.run(["go", "build", "-o", str(beacon), "./cmd/beacon"],
                               cwd=source, check=True)
            print(f"self-tree: binary_sha256={hashlib.sha256(beacon.read_bytes()).hexdigest()}")
            check(Path(args.project).resolve(), args.version, beacon, scratch)
    except (ValueError, KeyError, TypeError, OSError, sqlite3.Error,
            subprocess.CalledProcessError) as exc:
        parser.exit(1, f"self-tree: FAIL: {exc}\n")


if __name__ == "__main__":
    main()
