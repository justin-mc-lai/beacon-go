#!/usr/bin/env python3
"""Badminton dogfood: greenfield tree -> expand -> freeze -> implement ->
verify -> rollup -> change/refreeze -> snapshot -> HTML.

The host (this script) does the model work; the kernel only validates,
records provenance, freezes, and verifies evidence.
"""
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

BEACON = sys.argv[1]
BASE = pathlib.Path(tempfile.mkdtemp(prefix="beacon-dogfood."))
PROJ = BASE / "proj"
PROJ.mkdir()
ENV = dict(os.environ, BEACON_STORE_PATH=str(BASE / "store.db"))


def run(*args, expect=0, doc=True):
    p = subprocess.run([BEACON, *args], cwd=PROJ, env=ENV, text=True,
                       capture_output=True)
    if p.returncode != expect:
        print("FAIL:", args, "\nstdout:", p.stdout, "\nstderr:", p.stderr)
        sys.exit(1)
    return json.loads(p.stdout) if doc and p.stdout.strip() else p.stdout


def sha(path):
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()


def plan_tree():
    return run("plan", "tree", "--project", str(PROJ), "--json")


subprocess.run(["git", "init", "-b", "main"], cwd=PROJ, check=True,
               capture_output=True)
run("projects", "register", "--path", str(PROJ), "--version", "v0.1.0", "--json")

root = run("plan", "init", "--project", str(PROJ), "--title", "badminton community",
           "--intent", "let players find games, join events, and share their club life",
           "--json")["root"]["id"]

prov = {"model": "host-agent", "prompt_hash": "sha256:dogfood", "skill": "beacon-intent-tree",
        "tokens": 1200, "cost_usd": 0.02, "by": "dogfood-host"}


def apply(parent, children):
    proposal = BASE / "proposal.json"
    proposal.write_text(json.dumps({"children": children, "provenance": prov}, ensure_ascii=False))
    out = run("plan", "apply", "--project", str(PROJ), "--node", parent,
              "--proposal", str(proposal), "--json")
    return [n["id"] for n in out["nodes"]]


def by_title(kind, title):
    for n in plan_tree()["nodes"]:
        if n["kind"] == kind and n["title"] == title:
            return n
    raise SystemExit(f"missing {kind} {title}")


modules = apply(root, [
    {"kind": "module", "title": "events", "intent": "discover, join, and run events"},
    {"kind": "module", "title": "community", "intent": "share and discuss club life"},
    {"kind": "module", "title": "identity", "intent": "a trusted player profile"},
])

leaves = []
for module, prefix in zip(modules, ["event", "community", "identity"]):
    features = apply(module, [
        {"kind": "feature", "title": f"{prefix}-core", "intent": f"{prefix} core flow"},
        {"kind": "feature", "title": f"{prefix}-extra", "intent": f"{prefix} supporting flow"},
    ])
    for i, feature in enumerate(features):
        tps = apply(feature, [
            {"kind": "touchpoint", "title": f"{prefix}-{i}-primary",
             "intent": "the user completes the primary action",
             "acs": [{"id": f"{prefix}-{i}-p1", "text": "primary action succeeds"}],
             "verification": {"type": "test", "spec": "when the primary action runs, it returns success"}},
            {"kind": "touchpoint", "title": f"{prefix}-{i}-cancel",
             "intent": "the user can cancel the action",
             "acs": [{"id": f"{prefix}-{i}-c1", "text": "cancel releases the slot"}],
             "verification": {"type": "test", "spec": "when cancel runs, the slot is released"}},
        ])
        leaves.extend(tps)

# Bottom-up freeze: leaves, features, modules, root.
for node in leaves:
    run("truth", "node", "freeze", "--project", str(PROJ), "--node", node, "--json")
for n in plan_tree()["nodes"]:
    if n["kind"] == "feature":
        run("truth", "node", "freeze", "--project", str(PROJ), "--node", n["id"], "--json")
for module in modules:
    run("truth", "node", "freeze", "--project", str(PROJ), "--node", module, "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", root, "--json")

# Implement + verify every leaf with real evidence files.
evidence = {}
for i, leaf in enumerate(leaves):
    art = PROJ / "evidence" / f"{leaf}.txt"
    art.parent.mkdir(exist_ok=True)
    art.write_text(f"test output for {leaf} #{i}\n")
    evidence[leaf] = str(art)
    run("implement", "report", "--project", str(PROJ), "--node", leaf,
        "--by", "dogfood-dev", "--artifact", str(art), "--json")
    run("verify", "leaf", "--project", str(PROJ), "--node", leaf,
        "--artifact", str(art), "--by", "dogfood-qa", "--json")
run("verify", "rollup", "--project", str(PROJ), "--node", root, "--json")

doc = plan_tree()
assert all(n["status"] == "verified" for n in doc["nodes"] if n["kind"] == "touchpoint")
assert next(n for n in doc["nodes"] if n["id"] == root)["status"] == "verified"

# One subtree change: cancel touchpoint of the first feature.
changed = leaves[1]
base = next(n for n in doc["nodes"] if n["id"] == changed)["fingerprint"]
run("change", "request", "--project", str(PROJ), "--node", changed,
    "--summary", "tighten cancel rule", "--by", "dogfood-pm",
    "--base-fingerprint", base, "--json")
after = plan_tree()
status = {n["id"]: n["status"] for n in after["nodes"]}
assert status[changed] == "draft"
ancestor = by_title("feature", "event-core")["id"]
assert status[ancestor] == "change_pending"

# Refreeze the changed branch, re-implement, re-verify, then roll up.
run("truth", "node", "freeze", "--project", str(PROJ), "--node", changed, "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", ancestor, "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", modules[0], "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", root, "--json")
run("implement", "report", "--project", str(PROJ), "--node", changed,
    "--by", "dogfood-dev", "--artifact", evidence[changed], "--json")
run("verify", "leaf", "--project", str(PROJ), "--node", changed,
    "--artifact", evidence[changed], "--by", "dogfood-qa", "--json")
run("verify", "rollup", "--project", str(PROJ), "--node", root, "--json")

# History + HTML.
snaps = run("truth", "snapshot", "list", "--project", str(PROJ), "--json")["snapshots"]
assert len(snaps) >= 2, snaps
run("truth", "tree", "--project", str(PROJ), "--snapshot", "latest", "--json")
html = run("plan", "tree", "--project", str(PROJ), "--format", "html", doc=False)
assert "index.html" in html and pathlib.Path(html.split()[1]).exists(), html

# Evidence hash re-check against the node-scope registration.
listed = run("goal", "artifacts", "list", "--project", str(PROJ), "--scope-kind", "node",
             "--scope-id", changed, "--json")["artifact_refs"]
assert listed and listed[0]["content_hash"] == sha(evidence[changed]), listed

expansions = list((PROJ / "docs" / "beacon" / "v0.1.0" / "tree" / "expansions").glob("*.json"))
assert expansions, "no expansion provenance recorded"
prov_doc = json.loads(expansions[0].read_text())
assert prov_doc["provenance"]["model"] == "host-agent", prov_doc

audit = run("verify", "audit", "--project", str(PROJ), "--json")
assert audit["ok"] is True, audit

final = plan_tree()
print(json.dumps({
    "verdict": "PASS",
    "nodes": len(final["nodes"]),
    "leaves": len(leaves),
    "snapshots": len(snaps),
    "root_status": next(n for n in final["nodes"] if n["id"] == root)["status"],
    "audit": audit["ok"],
}, ensure_ascii=False))
shutil.rmtree(BASE, ignore_errors=True)
