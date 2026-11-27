#!/usr/bin/env python3
"""Apply the user-authorized 17-node tree verification contract update.

Only public Beacon CLI commands write canonical tree state. This script
rewrites the previously generated proposal in place so each node binds the
portable behavior test that actually covers its claim, and narrows the three
ACs that cannot be proven by an in-process CLI test.
"""
import argparse
import hashlib
import json
import pathlib
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
TREE = ROOT / "docs/beacon/v0.0.1/tree"
PROPOSAL = pathlib.Path("/tmp/beacon-cl744/proposal/nodes.json")
TEST_REL = "e2e/portableacceptance/portable_acceptance_test.go"

BEHAVIOR = {
    "n_a78ea52722a0": "^TestPortableStrictDeliveryGuard$",
    "n_a3b9d2c7f296": "^TestPortableStrictDeliveryGuard$",
    "n_5eaf5288ed83": "^TestPortableStrictDeliveryGuard$",
    "n_6ee4eee57ba1": "^TestPortableStrictDeliveryGuard$",
    "n_54606a05e1b6": "^TestPortableStrictDeliveryGuard$",
    "n_d8063fefcf41": "^TestPortableStrictDeliveryGuard$",
    "n_ec5052dd2cc5": "^TestPortableStrictDeliveryGuard$",
    "n_a11841bbc44c": "^TestPortableStrictDeliveryContract$",
    "n_ea9234b5e82c": "^TestPortableStrictDeliveryContract$",
    "n_a7c847346c2c": "^TestPortableTreeDrift$",
    "n_5d6dde8f4e1a": "^TestPortableModuleRun$",
    "n_3670106f522e": "^TestPortableModuleAdmission$",
    "n_8fe4e4482eee": "^TestPortableParallelPlan$",
    "n_17162700446e": "^TestPortableTreeHTMLPanels$",
    "n_adda25ea8f26": "^TestPortableModuleRun$",
    "n_40642d4f01bd": "^TestPortableVersionManifestAlignment$",
    "n_bd880ecae011": "^TestPortableTreeHTMLPanels$",
}

NARROW_ACS = {
    "n_ec5052dd2cc5": {
        "AC-001": "内核合同校验捕获 reviewer 身份、原始需求/候选/日志引用与 Green 后时序；真实上下文隔离由宿主流程另行提供证据，不由 Go 测试证明。",
        "AC-003": "宿主不支持独立 reviewer 时记录阻塞；身份字符串检查不宣称真实身份认证或上下文隔离已证明。",
    },
    "n_bd880ecae011": {
        "AC-001": "渲染 HTML 含 data-bind-ac/data-bind-testcase 绑定属性和写回表单字段；HTTP 绑定入口能够将 TestCase 写回指定 AC 并更新 canonical 指纹。",
    },
    "n_a11841bbc44c": {
        "AC-001": "七项 skill 文本合同声明完成定义、输入输出、边界、AC、任务依赖及适用检查，并声明确认前产品写入边界。",
        "AC-002": "七项 skill 文本合同声明一次推进一个原子任务，不自动提交；宿主实际执行纪律不由结构文本测试证明。",
    },
}

GROUP_ROOTS = {
    "n_9e4515fdf99c": [
        "n_a11841bbc44c", "n_a78ea52722a0", "n_ec5052dd2cc5",
        "n_6ee4eee57ba1", "n_a3b9d2c7f296", "n_ea9234b5e82c",
    ],
    "n_4bdf0311e438": ["n_5eaf5288ed83", "n_d8063fefcf41", "n_54606a05e1b6"],
}
STANDALONE = [
    "n_a7c847346c2c", "n_5d6dde8f4e1a", "n_3670106f522e",
    "n_8fe4e4482eee", "n_17162700446e", "n_adda25ea8f26",
    "n_40642d4f01bd", "n_bd880ecae011",
]
TOP = ["n_3c1f1d8ec396", "nroot_6b8991789867"]


def load_json(path):
    return json.loads(path.read_text())


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def test_sha256():
    return hashlib.sha256((ROOT / TEST_REL).read_bytes()).hexdigest()


def refresh_proposal():
    data = load_json(PROPOSAL)
    data["test_sha256"] = test_sha256()
    for change in data["changes"]:
        node_id = change["node_id"]
        path = TREE / "nodes" / f"{node_id}.json"
        node = load_json(path)
        change["base_fingerprint"] = node["fingerprint"]
        spec = {
            "schema": "beacon-capture-command.v1",
            "argv": ["go", "test", "./e2e/portableacceptance", "-run", BEHAVIOR[node_id]],
            "test_files": [TEST_REL],
            "required_tests": [BEHAVIOR[node_id].strip("^$")],
        }
        patch = {"verification": {"type": "test", "spec": json.dumps(spec, ensure_ascii=False, separators=(",", ":"))}}
        if BEHAVIOR[node_id] != "^TestPortableStrictDeliveryContract$" or not node.get("test_cases"):
            test_name = BEHAVIOR[node_id].strip("^$")
            cases = []
            for index, ac in enumerate(node.get("acs", [])):
                cases.append({
                    "id": f"TC-PORTABLE-{index + 1:03d}",
                    "ac_id": ac["id"],
                    "kind": "e2e",
                    "ref": f"{test_name} → {TEST_REL}",
                    "status": "existing",
                })
            if cases:
                patch["test_cases"] = cases
                change["note"] = (change.get("note", "") + " 补齐 portable 测试合同覆盖的用例映射。").strip()
        if node_id in NARROW_ACS:
            acs = []
            for ac in node["acs"]:
                item = dict(ac)
                item["text"] = NARROW_ACS[node_id].get(ac["id"], ac["text"])
                acs.append(item)
            patch["acs"] = acs
            change["note"] = "用户授权：收窄为内核可验证合同；宿主/浏览器行为需外部证据。"
        change["patch"] = patch
    write_json(PROPOSAL, data)
    return data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--beacon", required=True)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    binary = str(pathlib.Path(args.beacon).resolve())
    data = refresh_proposal()
    print(json.dumps({"proposal": str(PROPOSAL), "test_sha256": data["test_sha256"], "changes": len(data["changes"])}, ensure_ascii=False))
    if not args.apply:
        return
    with tempfile.TemporaryDirectory(prefix="beacon-authorized-contracts-") as temp:
        patch_paths = {}
        for change in data["changes"]:
            path = pathlib.Path(temp) / f"{change['node_id']}.json"
            write_json(path, change["patch"])
            patch_paths[change["node_id"]] = path
        for change in data["changes"]:
            node_id = change["node_id"]
            node = load_json(TREE / "nodes" / f"{node_id}.json")
            if node["status"] in {"frozen", "implemented", "verified", "delivered"}:
                subprocess.run([binary, "change", "request", "--project", str(ROOT), "--version", "v0.0.1",
                                "--node", node_id, "--base-fingerprint", node["fingerprint"],
                                "--summary", "用户授权：将冻结验证合同绑定到便携公开CLI行为测试并收窄不可内核证明AC",
                                "--by", "user-authorized-host", "--json"],
                               check=True, stdout=subprocess.DEVNULL)
            subprocess.run([binary, "truth", "node", "update", "--project", str(ROOT), "--version", "v0.0.1",
                            "--node", node_id, "--file", str(patch_paths[node_id]), "--json"],
                           check=True, stdout=subprocess.DEVNULL)
            subprocess.run([binary, "truth", "node", "freeze", "--project", str(ROOT), "--version", "v0.0.1",
                            "--node", node_id, "--json"], check=True, stdout=subprocess.DEVNULL)
        candidates = []
        for path in (TREE / "nodes").glob("*.json"):
            node = load_json(path)
            if node["id"] not in candidates:
                candidates.append(node["id"])
        def depth(node_id):
            node = load_json(TREE / "nodes" / f"{node_id}.json")
            parent = node.get("parent_id")
            return 0 if not parent else 1 + depth(parent)
        for node_id in sorted(candidates, key=depth, reverse=True):
            node = load_json(TREE / "nodes" / f"{node_id}.json")
            if node["status"] == "draft" or node["status"] == "change_pending":
                subprocess.run([binary, "truth", "node", "freeze", "--project", str(ROOT), "--version", "v0.0.1",
                                "--node", node_id, "--json"], check=True, stdout=subprocess.DEVNULL)


if __name__ == "__main__":
    main()
