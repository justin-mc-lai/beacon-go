#!/usr/bin/env python3
"""Render the prototype intent tree to one self-contained HTML file.

Output goes to out/tree.html (gitignored): HTML is a view, never a truth
source. Stdlib only.
"""
import html
import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
SAMPLE = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else HERE
OUT = SAMPLE / "out" / "tree.html"

KIND_LABEL = {
    "intent": "意图",
    "module": "模块",
    "feature": "功能",
    "touchpoint": "触达点",
}


def esc(text):
    return html.escape(text or "")


def render_node(node):
    kids = node.get("children") or []
    badges = [f'<span class="kind k-{esc(node["kind"])}">{KIND_LABEL.get(node["kind"], node["kind"])}</span>']
    if node.get("verification"):
        badges.append('<span class="verified">可验证</span>')
    if node.get("ambiguities"):
        badges.append('<span class="ambiguous">歧义</span>')
    acs = "".join(f"<li>{esc(ac['text'])}</li>" for ac in node.get("acs") or [])
    ver = node.get("verification")
    ver_html = ""
    if ver:
        ver_html = f'<p class="ver"><b>验证（{esc(ver["type"])}）：</b>{esc(ver["spec"])}</p>'
    children = ""
    if kids:
        children = '<div class="children">' + "".join(render_node(k) for k in kids) + "</div>"
    summary = f'{esc(node["title"])} {"".join(badges)}'
    body = f'<p class="intent">{esc(node.get("intent"))}</p>'
    if acs:
        body += f'<ul class="acs">{acs}</ul>'
    body += ver_html + children
    return f"<details open><summary>{summary}</summary>{body}</details>"


def count(node, kind):
    n = 1 if node["kind"] == kind else 0
    return n + sum(count(c, kind) for c in node.get("children") or [])


def main():
    tree = json.loads((SAMPLE / "intent-tree.json").read_text(encoding="utf-8"))
    root = tree["root"]
    counts = {k: count(root, k) for k in KIND_LABEL}
    OUT.parent.mkdir(parents=True, exist_ok=True)
    doc = f"""<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<title>意图树原型 · {esc(root['title'])}</title>
<style>
body{{font:15px/1.6 -apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;margin:32px;max-width:1000px;color:#1f2328}}
h1{{font-size:22px}} details{{margin:6px 0 6px 16px;border-left:1px solid #e3e6ea;padding-left:12px}}
summary{{cursor:pointer;font-weight:600}} .intent{{color:#4b5563;margin:6px 0}}
.kind{{font-size:11px;padding:1px 6px;border-radius:10px;margin-left:6px;vertical-align:middle}}
.k-intent{{background:#e8f0fe;color:#1a56db}} .k-module{{background:#fef3c7;color:#92400e}}
.k-feature{{background:#e0f2fe;color:#075985}} .k-touchpoint{{background:#dcfce7;color:#166534}}
.verified{{font-size:11px;background:#dcfce7;color:#166534;padding:1px 6px;border-radius:10px;margin-left:4px}}
.ambiguous{{font-size:11px;background:#fee2e2;color:#991b1b;padding:1px 6px;border-radius:10px;margin-left:4px}}
.acs{{margin:4px 0}} .ver{{font-size:13px;color:#374151;margin:4px 0}}
.meta{{color:#6b7280;font-size:13px}}
</style></head><body>
<h1>{esc(root['title'])}</h1>
<p class="meta">原型视图 · 由 canonical JSON 渲染 · 不是真相源。节点：意图 {counts['intent']} / 模块 {counts['module']} / 功能 {counts['feature']} / 触达点 {counts['touchpoint']}</p>
{render_node(root)}
</body></html>"""
    OUT.write_text(doc, encoding="utf-8")
    print(OUT)


if __name__ == "__main__":
    main()
