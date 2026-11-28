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
from datetime import datetime, timezone

BEACON = sys.argv[1]
BASE = pathlib.Path(tempfile.mkdtemp(prefix="beacon-dogfood."))
PROJ = BASE / "proj"
PROJ.mkdir()
FIXTURE = PROJ / "docs/beacon/dogfood"
FIXTURE.mkdir(parents=True)
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


def git(*args):
    return subprocess.run(["git", *args], cwd=PROJ, env=ENV, text=True,
                          capture_output=True, check=True).stdout.strip()


def commit(message):
    git("add", "behavior.json", "docs/beacon/dogfood/check.py")
    git("-c", "user.name=Dogfood Fixture", "-c", "user.email=dogfood@example.invalid",
        "-c", "commit.gpgsign=false", "commit", "-m", message)
    return git("rev-parse", "HEAD")


def write_behavior(ready):
    (PROJ / "behavior.json").write_text(json.dumps({ac: ready for ac in ac_ids}))


def capture(leaf, phase, red=None):
    args = ["implement", "capture", "--project", str(PROJ), "--version", "v0.1.0",
            "--node", leaf, "--ac", leaf_acs[leaf], "--phase", phase, "--by", "dogfood-dev"]
    if red:
        args.extend(["--red", red["artifact_id"]])
    result = run(*args)
    assert result["status"] == "accepted" and result["exit_code"] == (1 if phase == "red" else 0), result
    assert result["binary_sha256"] == sha(BEACON), result
    return result


def deliver(leaf, red, green, generation):
    directory = FIXTURE / f"{leaf}-{generation}"
    directory.mkdir()
    runs = {}
    for phase, result in [("red", red), ("green", green)]:
        log = directory / f"{phase}.log"
        shutil.copyfile(result["stdout_path"], log)
        runs[phase] = {key: result[key] for key in
                       ["command", "exit_code", "started_at", "finished_at", "test_sha256"]}
        runs[phase].update(capture_id=result["artifact_id"], log_path=log.name, log_sha256=sha(log))
    binding = {key: green[key] for key in
               ["node_id", "requirement_fingerprint", "source_commit", "binary_sha256"]}
    # Synthetic independent-review fixture, as in capture_delivery_fixture_test.go;
    # this exercises receipt binding, not an independent agent's judgment.
    review = dict(binding, reviewer="dogfood-qa", verdict="pass", findings=[],
                  reviewed_at=datetime.now(timezone.utc).isoformat(),
                  source_sha256=green["source_sha256"],
                  captures={leaf_acs[leaf]: green["artifact_id"]})
    review_path = directory / "review.json"
    review_path.write_text(json.dumps(review))
    receipt = dict(binding, schema="beacon-delivery.v1", maker="dogfood-dev",
                   tasks=[dict(ac_id=leaf_acs[leaf], status="done", **runs)],
                   review=dict(reviewer="dogfood-qa", verdict="pass", open_findings=0,
                               receipt_path=review_path.name, receipt_sha256=sha(review_path)))
    artifact = directory / "delivery.txt"
    artifact.write_text("".join(f"{key}: {binding[key]}\n" for key in
                                ["source_commit", "binary_sha256", "requirement_fingerprint"])
                        + "delivery_receipt: " + json.dumps(receipt) + "\n")
    run("implement", "report", "--project", str(PROJ), "--node", leaf,
        "--by", "dogfood-dev", "--artifact", str(artifact), "--json")
    run("verify", "leaf", "--project", str(PROJ), "--node", leaf,
        "--artifact", str(artifact), "--by", "dogfood-qa", "--json")
    return str(artifact)


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
    proposal = FIXTURE / "proposal.json"
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
ac_ids = []
spec = json.dumps({"schema": "beacon-capture-command.v1", "adapter": "custom-json",
                   "argv": ["./docs/beacon/dogfood/check.py"],
                   "test_files": ["docs/beacon/dogfood/check.py"], "required_tests": ["behavior"]})


def feature_doc(prefix, flow):
    return (f"## 问题\n球友缺少可靠的 {prefix} {flow} 操作闭环。\n"
            "## 目标用户\n组织活动、参与活动并交流的羽毛球俱乐部成员。\n"
            f"## 场景\n成员进入 {prefix} {flow}，完成主操作或取消占位。\n"
            "## 范围\n主操作成功，以及取消后释放名额两条触点。\n"
            "## 非目标\n不包含支付、消息推送和第三方账号接入。\n"
            "## 验收标准\n主操作返回成功；取消后名额释放。每条触点由绑定的 custom-json 断言验证。\n")


for module, prefix in zip(modules, ["event", "community", "identity"]):
    features = apply(module, [
        {"kind": "feature", "title": f"{prefix}-core", "intent": f"{prefix} core flow",
         "doc": feature_doc(prefix, "core")},
        {"kind": "feature", "title": f"{prefix}-extra", "intent": f"{prefix} supporting flow",
         "doc": feature_doc(prefix, "supporting")},
    ])
    for i, feature in enumerate(features):
        tps = apply(feature, [
            {"kind": "touchpoint", "title": f"{prefix}-{i}-primary",
             "intent": "the user completes the primary action",
             "acs": [{"id": f"{prefix}-{i}-p1", "text": "primary action succeeds"}],
             "verification": {"type": "test", "spec": spec}},
            {"kind": "touchpoint", "title": f"{prefix}-{i}-cancel",
             "intent": "the user can cancel the action",
             "acs": [{"id": f"{prefix}-{i}-c1", "text": "cancel releases the slot"}],
             "verification": {"type": "test", "spec": spec}},
        ])
        leaves.extend(tps)
        ac_ids.extend([f"{prefix}-{i}-p1", f"{prefix}-{i}-c1"])

leaf_acs = dict(zip(leaves, ac_ids))
# The protocol output reflects an executed assertion, never the requested phase.
checker = FIXTURE / "check.py"
checker.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
state = json.loads(Path("behavior.json").read_text())
try:
    assert state[os.environ["BEACON_CAPTURE_AC"]] is True, "action not implemented"
    passed = True
except AssertionError:
    passed = False
print(json.dumps({"schema": "beacon-capture-tests.v1",
    "invocation": os.environ["BEACON_CAPTURE_RESULT"],
    "node_id": os.environ["BEACON_CAPTURE_NODE"], "ac_id": os.environ["BEACON_CAPTURE_AC"],
    "phase": os.environ["BEACON_CAPTURE_PHASE"],
    "cases": [{"name": "behavior", "status": "passed" if passed else "failed", "assertions": 1}]}))
sys.exit(0 if passed else 1)
''')
checker.chmod(0o755)
write_behavior(False)
baseline_commit = commit("fixture: unimplemented badminton actions")

# Bottom-up freeze: leaves, features, modules, root.
for node in leaves:
    run("truth", "node", "freeze", "--project", str(PROJ), "--node", node, "--json")
for n in plan_tree()["nodes"]:
    if n["kind"] == "feature":
        run("truth", "node", "freeze", "--project", str(PROJ), "--node", n["id"], "--json")
for module in modules:
    run("truth", "node", "freeze", "--project", str(PROJ), "--node", module, "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", root, "--json")

# Capture all Reds before changing source, then all Greens against one candidate.
reds = {leaf: capture(leaf, "red") for leaf in leaves}
assert all(result["source_commit"] == baseline_commit for result in reds.values())
write_behavior(True)
candidate_commit = commit("fixture: implement badminton actions")
greens = {leaf: capture(leaf, "green", reds[leaf]) for leaf in leaves}
assert all(result["source_commit"] == candidate_commit for result in greens.values())
assert len({result["source_sha256"] for result in greens.values()}) == 1
evidence = {leaf: deliver(leaf, reds[leaf], greens[leaf], "initial") for leaf in leaves}
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
patch = FIXTURE / "change.json"
patch.write_text(json.dumps({"intent": "cancel releases the reserved slot immediately"}))
run("truth", "node", "update", "--project", str(PROJ), "--node", changed,
    "--file", str(patch), "--json")

# Refreeze the changed branch, re-implement, re-verify, then roll up.
run("truth", "node", "freeze", "--project", str(PROJ), "--node", changed, "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", ancestor, "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", modules[0], "--json")
run("truth", "node", "freeze", "--project", str(PROJ), "--node", root, "--json")
state = json.loads((PROJ / "behavior.json").read_text())
state[leaf_acs[changed]] = False
(PROJ / "behavior.json").write_text(json.dumps(state))
red = capture(changed, "red")
write_behavior(True)
green = capture(changed, "green", red)
assert green["requirement_fingerprint"] != greens[changed]["requirement_fingerprint"]
assert green["source_sha256"] == greens[changed]["source_sha256"]
evidence[changed] = deliver(changed, red, green, "refrozen")
run("verify", "rollup", "--project", str(PROJ), "--node", root, "--json")

# History + HTML.
snaps = run("truth", "snapshot", "list", "--project", str(PROJ), "--json")["snapshots"]
assert len(snaps) >= 2, snaps
run("truth", "tree", "--project", str(PROJ), "--snapshot", "latest", "--json")
html = run("plan", "tree", "--project", str(PROJ), "--format", "html", doc=False)
html_path = pathlib.Path(html.split()[1])
assert "index.html" in html and html_path.exists(), html
# A non-self project must get the same reading template, not a
# project-specific one-off.
rendered = html_path.read_text()
assert "需求工作台" in rendered and "data-panel" in rendered, "dogfood did not get the unified HTML template"

# Evidence hash re-check against the node-scope registration.
listed = run("goal", "artifacts", "list", "--project", str(PROJ), "--scope-kind", "node",
             "--scope-id", changed, "--json")["artifact_refs"]
assert any(a["content_hash"] == sha(evidence[changed]) for a in listed), listed

expansions = list((PROJ / "docs" / "beacon" / "v0.1.0" / "tree" / "expansions").glob("*.json"))
assert expansions, "no expansion provenance recorded"
prov_doc = json.loads(expansions[0].read_text())
assert prov_doc["provenance"]["model"] == "host-agent", prov_doc

audit = run("verify", "audit", "--project", str(PROJ), "--json")
assert audit["ok"] is True, audit

# Recheck real reserved logs after tampering, without refreshing their hashes.
log_path = pathlib.Path(green["stdout_path"])
original = log_path.read_bytes()
log_path.write_bytes(original + b"tampered\n")
rejected = run("verify", "audit", "--project", str(PROJ), "--json", expect=1)
assert rejected["ok"] is False and "capture_stale" in json.dumps(rejected), rejected
log_path.write_bytes(original)
assert run("verify", "audit", "--project", str(PROJ), "--json")["ok"] is True

final = plan_tree()
assert all(n["status"] == "verified" for n in final["nodes"]), final
print(json.dumps({
    "verdict": "PASS",
    "nodes": len(final["nodes"]),
    "leaves": len(leaves),
    "snapshots": len(snaps),
    "root_status": next(n for n in final["nodes"] if n["id"] == root)["status"],
    "audit": audit["ok"],
    "tamper_rejected": True,
    "captures": 2 * (len(leaves) + 1),
    "review": "synthetic fixture bound to real captures",
}, ensure_ascii=False))
shutil.rmtree(BASE, ignore_errors=True)
