#!/usr/bin/env python3
"""Render freeps flows as Mermaid graphs.

Reads flow definitions from a running freepsd (or from the local config
mirror) and writes one .mmd file per flow, so they can be eyeballed in any
Mermaid previewer (or on GitHub, which renders ```mermaid natively).

The point is to show what the flat operation list hides: which operations are
roots (run unconditionally), where the graph forks on success/failure, where
data flows vs. where execution is merely gated, and which results go unused.

Usage:
  flow2mermaid.py                       # 8 most complex flows from raspi
  flow2mermaid.py --top 12
  flow2mermaid.py BadHumidityControl WZaus
  flow2mermaid.py --dir tools/freeps-config-on-raspi/graphs --top 8
  flow2mermaid.py --expand WZaus        # inline one level of sub-flows
"""
import argparse
import json
import os
import re
import sys
import urllib.parse
import urllib.request
import zlib

BASE = os.environ.get("FREEPS_URL", "http://raspi.fritz.box:8080")

# operators that only compute a value: if nothing reads their output, the
# operation is dead weight (as opposed to an action whose side effect matters)
PURE_OPS = {"utils", "math", "time", "eval", "regexp", "store", "graphviz"}

# functions that branch: drawn as a decision node
DECISIONS = {
    "stringequal", "equals", "hasinput", "isempty", "isemptystring",
    "isactivealert", "hasalerts", "istrue", "isequal", "contains",
    "ismatch", "numbercompare", "compare", "day", "isday", "checkvalue",
    "greaterthan", "lessthan", "inarray", "keyexists", "propertyexists",
}

# edge style per reference kind: (arrow, label)
EDGES = {
    "InputFrom": ("-->", ""),
    "ExecuteOnSuccessOf": ("-->|yes|", ""),
    "ExecuteOnFailOf": ("-.-x|no|", ""),
    "ArgumentsFrom": ("-.->|args|", ""),
}

STYLES = {
    "tuya": "fill:#e8f5e9,stroke:#2e7d32,color:#1b5e20",
    "fritz": "fill:#e1f5fe,stroke:#0277bd,color:#01579b",
    "store": "fill:#f5f5f5,stroke:#9e9e9e,color:#212121",
    "alert": "fill:#ffebee,stroke:#c62828,color:#b71c1c",
    "sensor": "fill:#ede7f6,stroke:#5e35b1,color:#311b92",
    "pixeldisplay": "fill:#fff8e1,stroke:#f9a825,color:#f57f17",
    "telegram": "fill:#e3f2fd,stroke:#1565c0,color:#0d47a1",
    "mqtt": "fill:#fce4ec,stroke:#ad1457,color:#880e4f",
    "flow": "fill:#e0f2f1,stroke:#00695c,color:#004d40",
    "graph": "fill:#e0f2f1,stroke:#00695c,color:#004d40",
    "flowbytag": "fill:#e0f2f1,stroke:#00695c,color:#004d40",
    "graphbytag": "fill:#e0f2f1,stroke:#00695c,color:#004d40",
    "ui": "fill:#f3e5f5,stroke:#8e24aa,color:#4a148c",
    "influx": "fill:#efebe9,stroke:#5d4037,color:#3e2723",
    "curl": "fill:#fff3e0,stroke:#ef6c00,color:#e65100",
    "exec": "fill:#fff3e0,stroke:#ef6c00,color:#e65100",
}
DEFAULT_STYLE = "fill:#ffffff,stroke:#616161,color:#212121"


def esc(s):
    """Make a string safe inside a Mermaid quoted label."""
    s = str(s).replace('"', "'").replace("\n", " ").strip()
    return re.sub(r"\s+", " ", s)


def hid(s):
    """Stable short id suffix (python's hash() is randomised per process)."""
    return format(zlib.crc32(str(s).encode()), "x")[:6]


# tags that actually cause execution; anything else is a decorative label
# (freeps matches flows by tag, and a flow may carry extra grouping tags that
# never trigger it - those would only add noise to the trigger node)
TRIGGER_PREFIXES = ("topic:", "set:", "reset:", "active:", "inactive:",
                    "sender:", "to:", "device:", "property:",
                    "sensorName:", "changed.")
TRIGGER_PLAIN = {"cron", "minutely", "hourly", "alert", "mqtt", "smtp",
                 "bluetooth", "discovered", "telegram", "sensor"}


def trigger_label(tags, kind):
    """Human-readable trigger for root operations, from the flow tags."""
    tags = tags or []
    groups = []
    if "cron" in tags:
        when = [t for t in tags if t in ("minutely", "hourly", "daily")]
        groups.append("cron " + " ".join(when) if when else "cron")
    if "alert" in tags:
        sev = sorted(t.split(":", 1)[1] for t in tags if t.startswith("severity:"))
        groups.append("alert" + (" sev " + ",".join(sev) if sev else ""))
    for t in tags:
        if t.startswith("severity:") or t in ("cron", "alert"):
            continue
        if t.startswith(TRIGGER_PREFIXES):
            groups.append(esc(t))
        elif t in ("mqtt", "smtp", "bluetooth", "telegram", "sensor"):
            groups.append(t)
    if not groups:
        groups.append({"helper": "called by other flows",
                       "manual": "manual call"}.get(kind or "", "on call"))
    return " / ".join(dict.fromkeys(groups))


def node_label(idx, op, unused_output):
    name = op.get("Name") or ""
    opname = esc(op.get("Operator", ""))
    fn = esc(op.get("Function", ""))
    args = op.get("Arguments") or {}
    argstr = ", ".join(f"{esc(k)}={esc(v)}" for k, v in list(args.items())[:3])
    if len(args) > 3:
        argstr += ", …"
    head = f"#{idx} {esc(name)}" if name else f"#{idx}"
    lines = [head, f"{opname}/{fn}"]
    if argstr:
        lines.append(f"<i>{argstr}</i>")
    if unused_output:
        lines.append("⚠ result unused")
    return "<br/>".join(lines)


def shape(idx, op, is_root, is_decision, label_idx=None):
    label = node_label(idx if label_idx is None else label_idx, op, op["_unused"])
    if is_decision:
        return f'{idx}{{{{"{label}"}}}}'
    if is_root:
        return f'{idx}(["{label}"])'
    return f'{idx}["{label}"]'


def to_mermaid(flow_id, desc, subflows=None, depth=0, prefix=""):
    ops = desc.get("Operations") or []
    if not ops:
        return None
    names = [o.get("Name") or f"#{i}" for i, o in enumerate(ops)]
    index = {n: i for i, n in enumerate(names)}
    # node ids must be unique across the whole diagram, including inlined
    # sub-flow subgraphs, so every id gets the sub-flow's prefix
    nid = lambda i: f"{prefix}n{i}"

    referenced = set()
    branching = set()
    for o in ops:
        for f in EDGES:
            src = o.get(f)
            if src:
                referenced.add(src)
            if f in ("ExecuteOnSuccessOf", "ExecuteOnFailOf") and src:
                # something branches on this operation's result
                branching.add(src)
    out = desc.get("OutputFrom") or ""

    lines = ["flowchart TD"]
    classes = {}

    for i, op in enumerate(ops):
        refs = [op.get(f) for f in EDGES if op.get(f)]
        is_root = not refs
        opname = (op.get("Operator") or "").lower()
        fn = (op.get("Function") or "").lower()
        # an op nobody reads and that computes nothing is vestigial
        op["_unused"] = (names[i] not in referenced and names[i] != out
                         and opname in PURE_OPS)
        is_decision = fn in DECISIONS or names[i] in branching
        lines.append(f"    {shape(nid(i), op, is_root, is_decision, i)}")
        classes.setdefault(STYLES.get(opname, DEFAULT_STYLE), []).append(nid(i))

    # trigger node feeding every unconditional root operation
    roots = [i for i, o in enumerate(ops)
             if not any(o.get(f) for f in EDGES)]
    if roots:
        trig = f"{prefix}TRIGGER"
        lines.append(f'    {trig}([{"⚡ " + esc(trigger_label(desc.get("Tags"), desc.get("Kind")))}])'
                     ":::trigger")
        for i in roots:
            lines.append(f"    {trig} --> {nid(i)}")

    for i, op in enumerate(ops):
        for field, (arrow, _) in EDGES.items():
            src = op.get(field)
            if not src:
                continue
            if src == "_":
                lines.append(f'    {prefix}IN(["main input"]) --> {nid(i)}')
                continue
            j = index.get(src)
            if j is None:
                lines.append(f'    {nid(i)} -.->|"{esc(field)}"| '
                             f'{prefix}MISSING_{hid(src)}["⚠ missing op: {esc(src)}"]')
                continue
            if field == "InputFrom":
                # an op fed by a decision only runs when that decision succeeded
                src_fn = (ops[j].get("Function") or "").lower()
                yes = src_fn in DECISIONS or names[j] in branching
                lines.append(f"    {nid(j)} -->|yes| {nid(i)}" if yes
                             else f"    {nid(j)} --> {nid(i)}")
            elif field == "ExecuteOnSuccessOf":
                lines.append(f"    {nid(j)} -->|yes| {nid(i)}")
            elif field == "ExecuteOnFailOf":
                lines.append(f"    {nid(j)} -.->|no| {nid(i)}")
            else:
                lines.append(f"    {nid(j)} -.->|args| {nid(i)}")

    if out and out in index:
        lines.append(f'    {nid(index[out])} ==> {prefix}RET(["→ flow output"])')

    for style, ids in classes.items():
        cname = "c" + hid(style)
        lines.append(f"    classDef {cname} {style}")
        lines.append(f"    class {','.join(ids)} {cname}")
    lines.append("    classDef trigger fill:#fffde7,stroke:#fbc02d,color:#5d4037")
    lines.append("    classDef subflow fill:#e0f2f1,stroke:#00695c,color:#004d40")

    # inline one level of sub-flow calls
    if subflows and depth == 0:
        for i, op in enumerate(ops):
            if (op.get("Operator") or "").lower() not in ("flow", "graph"):
                continue
            sub = subflows.get(op.get("Function"))
            if not sub:
                continue
            sub_lines = to_mermaid(op["Function"], sub, None, depth + 1,
                                   prefix=f"SG{i}_")
            if not sub_lines:
                continue
            body = ["    " + l for l in sub_lines.splitlines()[1:]
                    if "classDef" not in l and not l.strip().startswith("class ")]
            lines.append(f'    subgraph SG{i}["↳ {esc(op["Function"])}"]')
            lines.extend(body)
            lines.append("    end")
    return "\n".join(lines)


def fetch_all(base):
    url = f"{base}/system/GetFlowDescByTag"
    with urllib.request.urlopen(url, timeout=15) as r:
        return json.load(r)


def load_dir(d):
    flows = {}
    for f in sorted(os.listdir(d)):
        if f.endswith(".json"):
            with open(os.path.join(d, f)) as fh:
                flows[f[:-5]] = json.load(fh)
    return flows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("flows", nargs="*")
    ap.add_argument("--top", type=int, default=8)
    ap.add_argument("--dir", help="read flow JSON from a directory instead of HTTP")
    ap.add_argument("--expand", action="store_true", help="inline one level of sub-flows")
    ap.add_argument("--out", default="/tmp/flowviz")
    ap.add_argument("--print", dest="do_print", action="store_true")
    args = ap.parse_args()

    flows = load_dir(args.dir) if args.dir else fetch_all(BASE)
    picked = args.flows
    if not picked:
        picked = [k for k, _ in sorted(flows.items(),
                                       key=lambda kv: -len(kv[1].get("Operations") or []))][:args.top]

    os.makedirs(args.out, exist_ok=True)
    for name in picked:
        desc = flows.get(name)
        if desc is None:
            print(f"!! not found: {name}", file=sys.stderr)
            continue
        subs = flows if args.expand else None
        mmd = to_mermaid(name, desc, subs)
        if not mmd:
            continue
        header = f"%% {name} — {len(desc.get('Operations') or [])} ops" + \
                 (f" — {desc['Description']}" if desc.get("Description") else "")
        if desc.get("Tags"):
            header += f"\n%% tags: {', '.join(desc['Tags'])}"
        path = os.path.join(args.out, name + ".mmd")
        with open(path, "w") as fh:
            fh.write(header + "\n" + mmd + "\n")
        print(path)
        if args.do_print:
            print(mmd)
            print()


if __name__ == "__main__":
    main()
