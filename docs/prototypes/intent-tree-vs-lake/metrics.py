#!/usr/bin/env python3
"""Prototype metrics for ticket #17.

Reads the frozen reference list, the two decomposition JSONs and the edit
log. Writes metrics.json and metrics.md. Stdlib only; no kernel imports.

Definitions (fixed before measurement):
- leaf unit: sea-lake -> feature; intent-tree -> node with kind=touchpoint
- coverage: distinct reference ids declared by leaf units / 15
- ambiguity points: explicit ambiguity entries + vague-token hits on leaf
  statements (title + intent + AC text), deduplicated per unit+token
- manual edits: entries in edit-log.json per model
- verification binding: leaf units with a non-empty verification spec / leaves
"""
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
SAMPLE = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else HERE
VAGUE = [
    "合理", "若干", "友好", "尽快", "适当", "较好", "完善", "丰富",
    "良好", "及时", "顺畅", "体验好", "相关功能", "等等",
]


def load(name):
    return json.loads((SAMPLE / name).read_text(encoding="utf-8"))


def walk(node):
    yield node
    for child in node.get("children") or []:
        yield from walk(child)


def leaf_units(model, doc):
    if model == "sea-lake":
        return [f for lake in doc["lakes"] for f in lake["features"]]
    return [n for n in walk(doc["root"]) if n["kind"] == "touchpoint"]


def statements(unit, model):
    if model == "sea-lake":
        parts = [unit.get("title", "")] + list(unit.get("acs") or [])
    else:
        parts = [unit.get("title", ""), unit.get("intent", "")]
        parts += [ac.get("text", "") for ac in unit.get("acs") or []]
    return " ".join(parts)


def explicit_ambiguities(model, doc):
    if model == "sea-lake":
        return [(a["unit"], a["text"]) for a in doc.get("ambiguities") or []]
    out = []
    for node in walk(doc["root"]):
        for a in node.get("ambiguities") or []:
            out.append((node["id"], a["text"]))
    return out


def metrics(model, doc, refs, edits):
    leaves = leaf_units(model, doc)
    covered = set()
    for unit in leaves:
        covered.update(unit.get("covers") or [])
    covered &= refs

    amb = set(explicit_ambiguities(model, doc))
    for unit in leaves:
        text = statements(unit, model)
        for token in VAGUE:
            if token in text:
                amb.add((unit["id"], token))

    bound = [
        u for u in leaves
        if isinstance(u.get("verification"), dict)
        and len((u["verification"].get("spec") or "").strip()) >= 8
    ]
    return {
        "leaf_units": len(leaves),
        "coverage_covered": len(covered),
        "coverage_total": len(refs),
        "coverage_missing": sorted(refs - covered),
        "ambiguity_points": len(amb),
        "ambiguity_detail": sorted(f"{u}:{t}" for u, t in amb),
        "manual_edits": edits.get(model, 0),
        "verified_bound": len(bound),
        "verification_binding_rate": round(len(bound) / len(leaves), 4) if leaves else 0.0,
    }


def main():
    refs = {i["id"] for i in load("reference-touchpoints.json")["items"]}
    lake = load("lake-model.json")
    tree = load("intent-tree.json")
    log = load("edit-log.json")
    edits = {}
    for entry in log["entries"]:
        edits[entry["model"]] = edits.get(entry["model"], 0) + 1

    lake_m = metrics("sea-lake", lake, refs, edits)
    tree_m = metrics("intent-tree", tree, refs, edits)

    pairs = [
        ("coverage_covered", "higher"),
        ("ambiguity_points", "lower"),
        ("manual_edits", "lower"),
        ("verification_binding_rate", "higher"),
    ]
    no_worse = 0
    for key, direction in pairs:
        a, b = lake_m[key], tree_m[key]
        if (direction == "higher" and b >= a) or (direction == "lower" and b <= a):
            no_worse += 1
    verdict = {
        "no_worse_metrics": no_worse,
        "required_no_worse": 3,
        "tree_binding_complete": tree_m["verification_binding_rate"] == 1.0,
        "pass": no_worse >= 3 and tree_m["verification_binding_rate"] == 1.0,
    }

    result = {
        "schema": "prototype.metrics.v1",
        "reference_count": len(refs),
        "models": {"sea-lake": lake_m, "intent-tree": tree_m},
        "verdict": verdict,
    }
    (SAMPLE / "metrics.json").write_text(
        json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )

    def row(name, a, b, direction):
        if b == a:
            label = "持平"
        elif (direction == "higher" and b > a) or (direction == "lower" and b < a):
            label = "优于"
        else:
            label = "劣于"
        good = label != "劣于"
        return f"| {name} | {a} | {b} | {'不劣于' if good else '劣于'}（{label}） |"

    md = [
        "# 指标结果（ticket #17）",
        "",
        "由 `metrics.py` 生成；口径见脚本头部。",
        "",
        "| 指标 | 湖海模型 | 意图树 | 对比 |",
        "|---|---:|---:|---|",
        row("叶子覆盖率（覆盖/15）", lake_m["coverage_covered"], tree_m["coverage_covered"], "higher"),
        row("歧义点数", lake_m["ambiguity_points"], tree_m["ambiguity_points"], "lower"),
        row("人工修改次数", lake_m["manual_edits"], tree_m["manual_edits"], "lower"),
        row("叶子验证绑定率", lake_m["verification_binding_rate"], tree_m["verification_binding_rate"], "higher"),
        "",
        f"- 湖海叶子单元：{lake_m['leaf_units']}；意图树叶子单元：{tree_m['leaf_units']}",
        f"- 湖海未覆盖基准项：{', '.join(lake_m['coverage_missing']) or '无'}",
        f"- 意图树未覆盖基准项：{', '.join(tree_m['coverage_missing']) or '无'}",
        f"- 湖海歧义明细：{', '.join(lake_m['ambiguity_detail']) or '无'}",
        f"- 意图树歧义明细：{', '.join(tree_m['ambiguity_detail']) or '无'}",
        "",
        f"**判定**：不劣于项 {verdict['no_worse_metrics']}/4（需 ≥3），"
        f"意图树绑定率 {tree_m['verification_binding_rate']:.0%}（需 100%）→ "
        f"{'通过' if verdict['pass'] else '未通过'}。",
        "",
    ]
    (SAMPLE / "metrics.md").write_text("\n".join(md), encoding="utf-8")
    print("\n".join(md))


if __name__ == "__main__":
    sys.exit(main())
