#!/usr/bin/env python3
"""Apply the explicitly authorized nine-leaf verification change via Beacon CLI.

Canonical JSON is read only here; all tree writes are kernel commands.
Run with --apply only after user approval. Generated patches are reviewable.
"""
import argparse
import json
import pathlib
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
TREE = ROOT / "docs/beacon/v0.0.1/tree"
GROUPS = {
    "n_9e4515fdf99c": {
        "n_a11841bbc44c": ("./skills", "^TestStrictDelivery"),
        "n_a78ea52722a0": ("./internal/cli", "^TestCapture"),
        "n_ec5052dd2cc5": ("./internal/cli", "^TestCaptureDeliveryRefusesCandidateAndReferenceForgery$"),
        "n_6ee4eee57ba1": ("./internal/cli", "^TestCapture"),
        "n_a3b9d2c7f296": ("./internal/cli", "^TestCaptureGoalSDDConnectedLedger$"),
        "n_ea9234b5e82c": ("./skills", "^TestStrictDelivery"),
    },
    "n_4bdf0311e438": {
        "n_5eaf5288ed83": ("./internal/cli", "^TestCapture"),
        "n_d8063fefcf41": ("./internal/cli", "^TestCapture"),
        "n_54606a05e1b6": ("./internal/cli", "^TestCapture"),
    },
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--beacon", required=True)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    binary = str(pathlib.Path(args.beacon).resolve())

    def call(*command):
        subprocess.run([binary, *command, "--project", str(ROOT), "--version", "v0.0.1", "--json"], check=True)

    with tempfile.TemporaryDirectory(prefix="beacon-capture-contracts-") as temp:
        patches = {}
        for leaves in GROUPS.values():
            for node_id, (package, pattern) in leaves.items():
                package_info = json.loads(subprocess.check_output(["go", "list", "-json", package], cwd=ROOT))
                directory = pathlib.Path(package_info["Dir"])
                tests = sorted(str((directory / name).relative_to(ROOT)) for name in package_info.get("TestGoFiles", []) + package_info.get("XTestGoFiles", []))
                names = subprocess.check_output(["go", "test", package, "-list", pattern], cwd=ROOT, text=True)
                required = [line for line in names.splitlines() if line.startswith("Test") and " " not in line]
                if not tests or not required:
                    raise SystemExit(f"empty tests/required cases: {node_id}")
                spec = {"schema": "beacon-capture-command.v1", "argv": ["go", "test", package, "-run", pattern], "test_files": tests, "required_tests": required}
                patch = {"verification": {"type": "test", "spec": json.dumps(spec, ensure_ascii=False, separators=(",", ":"))}}
                path = pathlib.Path(temp) / f"{node_id}.json"
                path.write_text(json.dumps(patch, ensure_ascii=False))
                patches[node_id] = path
                print(json.dumps({"node": node_id, "patch": patch}, ensure_ascii=False), flush=True)
        if not args.apply:
            return
        for parent, leaves in GROUPS.items():
            node = json.loads((TREE / "nodes" / f"{parent}.json").read_text())
            call("change", "request", "--node", parent, "--base-fingerprint", node["fingerprint"], "--summary", "用户授权：文字验证规格绑定当前可执行采集配置；AC和实现状态不变，不补造历史Red", "--by", "user-authorized-host")
            for node_id in leaves:
                call("truth", "node", "update", "--node", node_id, "--file", str(patches[node_id]))
                call("truth", "node", "freeze", "--node", node_id)
            call("truth", "node", "freeze", "--node", parent)
        for node_id in ("n_3c1f1d8ec396", "nroot_6b8991789867"):
            call("truth", "node", "freeze", "--node", node_id)


if __name__ == "__main__":
    main()
