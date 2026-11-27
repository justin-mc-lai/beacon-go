#!/usr/bin/env python3
"""Real CLI success/refusal probes; mutations only touch disposable copies."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts/check_self_tree.sh"
VERSION = "v0.0.1"  # Fixture delivery line, not the binary release version.


def hashes(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob("*") if p.is_file()}


class SelfTreeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory(prefix="self-tree-probes-")
        cls.base = Path(cls.tmp.name).resolve()
        cls.binary = cls.base / "beacon"
        subprocess.run(["go", "build", "-o", str(cls.binary), "./cmd/beacon"],
                       cwd=ROOT, check=True)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def test_real_entry(self):
        tree_source = ROOT / "docs/beacon" / VERSION / "tree"
        source_before = hashes(tree_source)
        for case in ("success-relative", "success-absolute", "fingerprint-drift",
                     "index-drift", "empty", "blank-id", "duplicate-id",
                     "duplicate-key", "missing-node", "wrong-version"):
            with self.subTest(case=case):
                project = self.base / case
                tree = project / "docs/beacon" / VERSION / "tree"
                shutil.copytree(tree_source, tree)
                index_path = tree / "index.json"
                index = json.loads(index_path.read_text())
                leaf = tree / "nodes/n_b935c7e6837a.json"
                if case == "fingerprint-drift":
                    body = json.loads(leaf.read_text())
                    body["acs"][0]["text"] = "rebuild must fail (opposite contract)"
                    leaf.write_text(json.dumps(body))
                elif case == "index-drift":
                    index["nodes"][0]["fingerprint"] = "0" * 64
                elif case == "empty":
                    index["nodes"] = []
                elif case == "blank-id":
                    index["nodes"][0]["id"] = " "
                elif case == "duplicate-id":
                    index["nodes"].append(index["nodes"][0])
                elif case == "missing-node":
                    leaf.unlink()
                index_path.write_text(json.dumps(index))
                if case == "duplicate-key":
                    index_path.write_text('{"nodes": [],' + index_path.read_text()[1:])
                before = hashes(tree)
                # If the guard leaks the inherited ledger path, SQLite cannot
                # open this directory as a DB. It must use its own temp store.
                sentinel = project / "caller-store"
                sentinel.mkdir()
                env = dict(os.environ, BEACON_STORE_PATH=str(sentinel))
                path = os.path.relpath(project, ROOT) if case.endswith("relative") else str(project)
                result = subprocess.run(
                    ["bash", str(SCRIPT), "--project", path, "--version",
                     "v9.9.9" if case == "wrong-version" else VERSION,
                     "--beacon", str(self.binary)], cwd=ROOT, env=env,
                    text=True, capture_output=True)
                output = result.stdout + result.stderr
                self.assertEqual(hashes(tree), before, output)
                self.assertEqual(list(sentinel.iterdir()), [], output)
                if case.startswith("success"):
                    self.assertEqual(result.returncode, 0, output)
                    count = len(index["nodes"])
                    self.assertIn(f"indexed={count} rebuilt={count} projected={count}", output)
                else:
                    self.assertNotEqual(result.returncode, 0, output)
                    self.assertIn("self-tree: FAIL", output)
                    if case == "fingerprint-drift":
                        self.assertIn("fingerprint drift", output)
                print(f"probe {case}: exit={result.returncode}; canonical unchanged", flush=True)
        self.assertEqual(hashes(tree_source), source_before)


if __name__ == "__main__":
    unittest.main()
