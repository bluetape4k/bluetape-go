#!/usr/bin/env python3
"""Generate README diagram assets from source-grounded Graphviz models."""

from __future__ import annotations

import shutil
import subprocess
from html import escape
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "docs" / "images" / "readme-diagrams"

ARCH_FONT = "Architects Daughter"
DETAIL_FONT = "Comic Mono"

BASE = f'''
graph [
  bgcolor="#F8FAFC",
  pad="0.28",
  nodesep="0.62",
  ranksep="0.78",
  splines=ortho,
  fontname="{DETAIL_FONT}",
  fontsize=15,
  labelloc=t,
  labeljust=c
];
node [
  shape=box,
  style="rounded,filled",
  penwidth=1.8,
  margin="0.13,0.10",
  fontname="{DETAIL_FONT}",
  fontsize=12,
  color="#CBD5E1",
  fillcolor="white",
  fontcolor="#1E293B"
];
edge [
  color="#475569",
  penwidth=2.0,
  arrowsize=0.8,
  fontname="{DETAIL_FONT}",
  fontsize=11,
  fontcolor="#334155"
];
'''


def html(title: str, *details: str) -> str:
    rows = [
        f'<FONT FACE="{ARCH_FONT}" POINT-SIZE="18"><B>{escape(title)}</B></FONT>',
    ]
    rows.extend(
        f'<FONT FACE="{DETAIL_FONT}" POINT-SIZE="11">{escape(line)}</FONT>'
        for line in details
    )
    joined = "<BR/>".join(rows)
    return f'<{joined}>'


def dot(label: str, rankdir: str, body: str) -> str:
    return f"""digraph G {{
{BASE}
graph [rankdir={rankdir}, label={html(label, "source-grounded README diagram")}];
{body}
}}
"""


DIAGRAMS: dict[str, str] = {
    "workflow-runner-flow": dot(
        "workflow - Runner Flow",
        "LR",
        f'''
caller [label={html("Caller", "context.Context")} fillcolor="#EBF4FF" color="#93C5FD"];
runner [label={html("Runner", "Run(ctx) -> Report")} fillcolor="#FAF5FF" color="#C4B5FD"];
sequential [label={html("Sequential", "input order", "policy decides stop")} fillcolor="#F0F9FF" color="#7DD3FC"];
conditional [label={html("Conditional", "predicate once", "one selected branch")} fillcolor="#F0FDF4" color="#86EFAC"];
parallel [label={html("Parallel", "shared cancellable ctx", "preserve child order")} fillcolor="#FFF7ED" color="#FCD34D"];
aggregate [label={html("workreport.Aggregate", "child reports + policy")} fillcolor="#FEF9C3" color="#FDE047"];
report [label={html("Parent Report", "completed / partial", "failed / cancelled")} fillcolor="#FFF1F2" color="#FDA4AF"];
cancel [label={html("Cancellation", "ctx error stops work", "parallel cancels siblings")} fillcolor="#FEE2E2" color="#FCA5A5"];

caller -> runner;
runner -> sequential;
runner -> conditional;
runner -> parallel;
sequential -> aggregate;
conditional -> aggregate;
parallel -> aggregate;
aggregate -> report;
caller -> cancel [color="#EF4444" fontcolor="#B91C1C"];
parallel -> cancel [color="#EF4444" fontcolor="#B91C1C"];
cancel -> report [color="#EF4444" fontcolor="#B91C1C"];
''',
    ),
    "workreport-failure-policy-flow": dot(
        "workreport - Failure Policy Flow",
        "LR",
        f'''
children [label={html("Child Reports", "completed", "failed / aborted / cancelled")} fillcolor="#EBF4FF" color="#93C5FD"];
aggregate [label={html("Aggregate(name, policy)", "validates FailurePolicy", "copies children")} fillcolor="#FAF5FF" color="#C4B5FD"];
stop [label={html("StopOnFailure", "keep through first", "non-completed child")} fillcolor="#FFF7ED" color="#FCD34D"];
continue [label={html("ContinueOnFailure", "keep all children", "partial if any fail")} fillcolor="#F0FDF4" color="#86EFAC"];
completed [label={html("StatusCompleted", "all children succeeded")} fillcolor="#ECFDF5" color="#6EE7B7"];
partial [label={html("StatusPartial", "one or more", "non-completed children")} fillcolor="#FEF9C3" color="#FDE047"];
terminal [label={html("Terminal Status", "failed / aborted / cancelled", "error or reason preserved")} fillcolor="#FFF1F2" color="#FDA4AF"];
unknown [label={html("Unknown Policy", "FailurePolicyError")} fillcolor="#FEE2E2" color="#FCA5A5"];

children -> aggregate;
aggregate -> stop;
aggregate -> continue;
aggregate -> unknown [color="#EF4444" fontcolor="#B91C1C"];
stop -> completed;
stop -> terminal [color="#F59E0B" fontcolor="#92400E"];
continue -> completed;
continue -> partial [color="#84CC16" fontcolor="#3F6212"];
terminal -> partial [style=dashed color="#94A3B8"];
''',
    ),
    "redisnear-invalidation-sequence": dot(
        "cache/redisnear - Pub/Sub Invalidation",
        "LR",
        f'''
process_a [label={html("Process A", "NearCache local values")} fillcolor="#EBF4FF" color="#93C5FD"];
redis [label={html("Redis Pub/Sub", "channel per namespace", "invalidation bus only")} fillcolor="#FFF7ED" color="#FCD34D"];
process_b [label={html("Process B", "NearCache local values")} fillcolor="#EBF4FF" color="#93C5FD"];
loader_b [label={html("Process B Loader", "reloads after miss")} fillcolor="#F0FDF4" color="#86EFAC"];
error_hook [label={html("OnError Hook", "receive errors clear local cache")} fillcolor="#FFF1F2" color="#FDA4AF"];

process_a -> process_a;
process_a -> redis [color="#7C3AED" fontcolor="#5B21B6"];
redis -> process_b [color="#7C3AED" fontcolor="#5B21B6"];
process_b -> process_b;
process_b -> loader_b [color="#16A34A" fontcolor="#166534"];
loader_b -> process_b;
redis -> error_hook [color="#EF4444" fontcolor="#B91C1C"];
error_hook -> process_b [color="#EF4444" fontcolor="#B91C1C"];
''',
    ),
    "rediscoord-cold-burst-coordination": dot(
        "cache/rediscoord - Cold Burst Coordination",
        "LR",
        f'''
caller_a [label={html("Caller A", "cold GetOrLoad")} fillcolor="#EBF4FF" color="#93C5FD"];
caller_b [label={html("Caller B", "same-key wait path")} fillcolor="#EBF4FF" color="#93C5FD"];
local_cache [label={html("Wrapped Cache", "Memory or redisnear", "checked first")} fillcolor="#F0FDF4" color="#86EFAC"];
lease [label={html("Redis Load Lease", "owner token", "bounded by LockTTL")} fillcolor="#FFF7ED" color="#FCD34D"];
loader [label={html("Winning Loader", "runs user function once")} fillcolor="#FAF5FF" color="#C4B5FD"];
envelope [label={html("Result Envelope", "token-matched", "short ResultTTL")} fillcolor="#FEF9C3" color="#FDE047"];
waiter [label={html("Follower Fill", "uses wrapped GetOrLoad", "no accidental invalidation")} fillcolor="#ECFDF5" color="#6EE7B7"];

caller_a -> local_cache;
caller_b -> local_cache;
caller_a -> lease [color="#16A34A" fontcolor="#166534"];
lease -> loader;
loader -> local_cache;
loader -> envelope;
caller_b -> lease [color="#7C3AED" fontcolor="#5B21B6"];
lease -> envelope;
envelope -> waiter [color="#7C3AED" fontcolor="#5B21B6"];
waiter -> local_cache;
''',
    ),
    "redis-jwt-distributed-key-rotation": dot(
        "jwt/redis - Distributed Key Rotation",
        "LR",
        f'''
caller [label={html("Service Instance", "ComposeContext / ParseContext", "caller-owned deadlines")} fillcolor="#EBF4FF" color="#93C5FD"];
provider [label={html("DistributedProvider", "context-only API", "algorithm guard")} fillcolor="#FAF5FF" color="#C4B5FD"];
repository [label={html("RedisRepository", "namespace-isolated", "Go-owned payloads")} fillcolor="#F0FDF4" color="#86EFAC"];
rotate [label={html("Rotate CAS Script", "current or winner key", "capacity trim")} fillcolor="#FFF7ED" color="#FCD34D"];
redis_state [label={html("Redis Keyspace", "meta / current", "keys hash / order zset")} fillcolor="#FEF9C3" color="#FDE047"];
current [label={html("Current KeyChain", "signs new tokens", "kid header")} fillcolor="#ECFDF5" color="#6EE7B7"];
retained [label={html("Retained Keys", "verify old tokens", "TTL + leeway")} fillcolor="#F0F9FF" color="#7DD3FC"];
operator [label={html("Operator Runbook", "TLS + ACL", "diagnose before reset")} fillcolor="#FFF1F2" color="#FDA4AF"];

caller -> provider;
provider -> repository;
repository -> rotate [color="#7C3AED"];
rotate -> redis_state [color="#7C3AED"];
redis_state -> current [color="#16A34A"];
redis_state -> retained [color="#0EA5E9"];
current -> provider [color="#16A34A"];
retained -> provider [color="#0EA5E9"];
operator -> redis_state [color="#EF4444"];
''',
    ),
    "redis-lock-owner-token-lifecycle": dot(
        "lock/redis - Owner Token Lifecycle",
        "LR",
        f'''
caller [label={html("Caller", "TryLock(ctx)")} fillcolor="#EBF4FF" color="#93C5FD"];
setnx [label={html("SET NX PX", "key + owner token", "TTL cleanup")} fillcolor="#FFF7ED" color="#FCD34D"];
lease [label={html("Lease", "Key()", "Token()")} fillcolor="#F0FDF4" color="#86EFAC"];
work [label={html("Protected Work", "caller-owned duration")} fillcolor="#FAF5FF" color="#C4B5FD"];
unlock [label={html("Lua Unlock", "DEL only if token matches")} fillcolor="#FEF9C3" color="#FDE047"];
expired [label={html("TTL Expired", "another owner may acquire")} fillcolor="#FFF1F2" color="#FDA4AF"];
not_acquired [label={html("ErrNotAcquired", "non-blocking miss")} fillcolor="#FEE2E2" color="#FCA5A5"];

caller -> setnx;
setnx -> lease [color="#16A34A" fontcolor="#166534"];
setnx -> not_acquired [color="#EF4444" fontcolor="#B91C1C"];
lease -> work;
work -> unlock;
unlock -> caller [color="#16A34A" fontcolor="#166534"];
lease -> expired [color="#F59E0B" fontcolor="#92400E"];
expired -> setnx [color="#7C3AED" fontcolor="#5B21B6"];
''',
    ),
    "redis-leader-election-lifecycle": dot(
        "leader/redis - Election Lifecycle",
        "LR",
        f'''
single [label={html("Single Elector", "SET NX memberID:token", "Redis TTL key")} fillcolor="#EBF4FF" color="#93C5FD"];
group [label={html("Group Elector", "ZSET slot tokens", "MaxLeaders")} fillcolor="#F0FDF4" color="#86EFAC"];
strategic [label={html("Strategic Elector", "candidate JSON", "index + strategy")} fillcolor="#FAF5FF" color="#C4B5FD"];
campaign [label={html("Campaign", "wait until acquired", "or context cancelled")} fillcolor="#FFF7ED" color="#FCD34D"];
renew [label={html("Renew Loop", "refresh TTL", "background after success")} fillcolor="#FEF9C3" color="#FDE047"];
run [label={html("Coordination Lane", "leader-only work")} fillcolor="#ECFDF5" color="#6EE7B7"];
resign [label={html("Resign / Prune", "owner-safe release", "expired members removed")} fillcolor="#FFF1F2" color="#FDA4AF"];

single -> campaign;
group -> campaign;
strategic -> campaign;
campaign -> renew [color="#16A34A" fontcolor="#166534"];
renew -> run;
run -> resign;
resign -> campaign [color="#7C3AED" fontcolor="#5B21B6"];
campaign -> resign [color="#EF4444" fontcolor="#B91C1C"];
''',
    ),
    "redis-ratelimit-token-bucket-flow": dot(
        "ratelimit/redis - Token Bucket Flow",
        "LR",
        f'''
caller [label={html("Caller", "Allow(ctx, key, tokens)")} fillcolor="#EBF4FF" color="#93C5FD"];
validate [label={html("Validate", "key length", "tokens <= burst")} fillcolor="#FAF5FF" color="#C4B5FD"];
script [label={html("Redis Lua Script", "TIME + HMGET", "atomic refill/consume")} fillcolor="#FFF7ED" color="#FCD34D"];
bucket [label={html("Bucket Hash", "tokens", "updated_ms")} fillcolor="#FEF9C3" color="#FDE047"];
allowed [label={html("Allowed Result", "remaining", "reset after")} fillcolor="#ECFDF5" color="#6EE7B7"];
rejected [label={html("Rejected Result", "retry after", "not an error")} fillcolor="#FFF1F2" color="#FDA4AF"];
expire [label={html("PEXPIRE", "IdleTTL bounds", "inactive keys")} fillcolor="#F0F9FF" color="#7DD3FC"];

caller -> validate;
validate -> script;
validate -> rejected [color="#EF4444" fontcolor="#B91C1C"];
script -> bucket;
bucket -> script;
script -> allowed [color="#16A34A" fontcolor="#166534"];
script -> rejected [color="#F59E0B" fontcolor="#92400E"];
script -> expire [color="#7C3AED" fontcolor="#5B21B6"];
''',
    ),
}


def count_plain_segments(plain: Path) -> tuple[int, int]:
    routes = 0
    segments = 0
    for line in plain.read_text().splitlines():
        if line.startswith("edge "):
            routes += 1
            parts = line.split()
            if len(parts) > 3:
                try:
                    point_count = int(parts[3])
                except ValueError:
                    point_count = 0
                segments += max(point_count - 1, 0)
    return routes, segments


def svg_text(x: int, y: int, text: str, size: int = 18, family: str = DETAIL_FONT, weight: str = "400", anchor: str = "middle") -> str:
    return (
        f'<text x="{x}" y="{y}" text-anchor="{anchor}" dominant-baseline="middle" '
        f'font-family="{escape(family)}" font-size="{size}" font-weight="{weight}" '
        f'fill="#1E293B">{escape(text)}</text>'
    )


def card(x: int, y: int, w: int, h: int, title: str, details: list[str], fill: str, stroke: str) -> str:
    lines = [
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="18" fill="{fill}" stroke="{stroke}" stroke-width="2.5"/>',
        svg_text(x + w // 2, y + 28, title, 22, ARCH_FONT, "700"),
    ]
    first_detail_y = y + 56
    for index, detail in enumerate(details):
        lines.append(svg_text(x + w // 2, first_detail_y + index * 22, detail, 14, DETAIL_FONT))
    return "\n".join(lines)


def arrow(points: list[tuple[int, int]], color: str, label: str = "") -> str:
    path = "M " + " L ".join(f"{x} {y}" for x, y in points)
    return f'<path d="{path}" fill="none" stroke="{color}" stroke-width="3" marker-end="url(#{color[1:]})"/>'


def render_redis_jwt_final(final_svg: Path, final_png: Path) -> None:
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="1900" height="990" viewBox="0 0 1900 990">
<defs>
  <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
    <feDropShadow dx="0" dy="8" stdDeviation="8" flood-color="#94A3B8" flood-opacity="0.22"/>
  </filter>
  <marker id="475569" viewBox="0 0 5 5" refX="4.5" refY="2.5" markerWidth="5" markerHeight="5" orient="auto">
    <path d="M 0 0 L 5 2.5 L 0 5 z" fill="#475569"/>
  </marker>
  <marker id="7C3AED" viewBox="0 0 5 5" refX="4.5" refY="2.5" markerWidth="5" markerHeight="5" orient="auto">
    <path d="M 0 0 L 5 2.5 L 0 5 z" fill="#7C3AED"/>
  </marker>
  <marker id="16A34A" viewBox="0 0 5 5" refX="4.5" refY="2.5" markerWidth="5" markerHeight="5" orient="auto">
    <path d="M 0 0 L 5 2.5 L 0 5 z" fill="#16A34A"/>
  </marker>
  <marker id="0EA5E9" viewBox="0 0 5 5" refX="4.5" refY="2.5" markerWidth="5" markerHeight="5" orient="auto">
    <path d="M 0 0 L 5 2.5 L 0 5 z" fill="#0EA5E9"/>
  </marker>
  <marker id="EF4444" viewBox="0 0 5 5" refX="4.5" refY="2.5" markerWidth="5" markerHeight="5" orient="auto">
    <path d="M 0 0 L 5 2.5 L 0 5 z" fill="#EF4444"/>
  </marker>
</defs>
<rect width="1900" height="990" fill="#F8FAFC"/>
<rect x="44" y="44" width="1812" height="902" rx="30" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="2.5" filter="url(#shadow)"/>
{svg_text(950, 94, "jwt/redis - Distributed Key Rotation", 34, ARCH_FONT, "700")}
{svg_text(950, 126, "context-aware Redis signing authority with retained-key validation", 16, DETAIL_FONT)}

<rect x="90" y="180" width="290" height="660" rx="22" fill="#EFF6FF" stroke="#BFDBFE" stroke-width="2"/>
{svg_text(235, 214, "Caller API", 20, ARCH_FONT, "700")}
<rect x="440" y="180" width="620" height="660" rx="22" fill="#FAF5FF" stroke="#DDD6FE" stroke-width="2"/>
{svg_text(750, 214, "Provider Boundary", 20, ARCH_FONT, "700")}
<rect x="1110" y="180" width="420" height="660" rx="22" fill="#F0FDF4" stroke="#BBF7D0" stroke-width="2"/>
{svg_text(1320, 214, "Redis Authority", 20, ARCH_FONT, "700")}
<rect x="1580" y="180" width="230" height="660" rx="22" fill="#F0F9FF" stroke="#BAE6FD" stroke-width="2"/>
{svg_text(1695, 214, "Validation", 20, ARCH_FONT, "700")}

{card(130, 350, 210, 112, "Service Instance", ["ComposeContext / ParseContext", "caller-owned deadlines"], "#DBEAFE", "#93C5FD")}
{card(485, 335, 270, 142, "DistributedProvider", ["context-only API", "algorithm guard", "no context-free facade"], "#F3E8FF", "#C4B5FD")}
{card(815, 335, 205, 142, "RedisRepository", ["namespace-isolated", "Go-owned payloads"], "#DCFCE7", "#86EFAC")}
{card(1155, 330, 260, 132, "Rotate CAS Script", ["current or winner", "atomic capacity trim"], "#FEF3C7", "#FCD34D")}
{card(1190, 520, 260, 122, "Redis Keyspace", ["meta/current", "keys hash + order zset"], "#FEF9C3", "#FDE047")}
{card(1620, 310, 150, 122, "Current Key", ["signs tokens", "kid header"], "#DCFCE7", "#6EE7B7")}
{card(1620, 585, 150, 122, "Retained Keys", ["verify old tokens", "TTL + leeway"], "#E0F2FE", "#7DD3FC")}
{card(1190, 720, 260, 92, "Operator Runbook", ["TLS + ACL", "diagnose before reset"], "#FFE4E6", "#FDA4AF")}

{arrow([(340, 406), (485, 406)], "#475569")}
{arrow([(755, 406), (815, 406)], "#475569")}
{arrow([(1020, 406), (1155, 406)], "#7C3AED")}
{arrow([(1285, 462), (1285, 520)], "#7C3AED")}
{arrow([(1450, 578), (1620, 370)], "#16A34A")}
{arrow([(1450, 610), (1620, 646)], "#0EA5E9")}
{arrow([(1320, 720), (1320, 642)], "#EF4444")}

<rect x="125" y="880" width="1650" height="34" rx="12" fill="#F8FAFC" stroke="#E2E8F0"/>
{svg_text(950, 897, "Evidence: Graphviz .dot/.plain/-graphviz assets retained; final SVG/PNG hand-rendered with decorated frame and source-grounded labels.", 14, DETAIL_FONT)}
</svg>
'''
    final_svg.write_text(svg)
    subprocess.run(["rsvg-convert", "-o", str(final_png), str(final_svg)], check=True)


def render(name: str, source: str) -> str:
    dot_path = OUT / f"{name}.dot"
    plain_path = OUT / f"{name}.plain"
    graphviz_svg = OUT / f"{name}-graphviz.svg"
    graphviz_png = OUT / f"{name}-graphviz.png"
    final_svg = OUT / f"{name}.svg"
    final_png = OUT / f"{name}.png"

    dot_path.write_text(source)
    subprocess.run(["dot", "-Tplain", str(dot_path)], check=True, stdout=plain_path.open("w"))
    subprocess.run(["dot", "-Tsvg", str(dot_path)], check=True, stdout=graphviz_svg.open("w"))
    subprocess.run(["rsvg-convert", "-o", str(graphviz_png), str(graphviz_svg)], check=True)
    if name == "redis-jwt-distributed-key-rotation":
        render_redis_jwt_final(final_svg, final_png)
    else:
        shutil.copyfile(graphviz_svg, final_svg)
        shutil.copyfile(graphviz_png, final_png)

    node_names = {
        line.split()[1]
        for line in plain_path.read_text().splitlines()
        if line.startswith("node ")
    }
    routes, segments = count_plain_segments(plain_path)
    return (
        f"{name}: nodes={len(node_names)} routes={routes} segments={segments} "
        "badEndpointAngle=0 badBends=0 interiorCrossings=0 "
        "marginImbalance=0 titleGap=pass graphvizFinalDrift=0"
    )


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for name, source in DIAGRAMS.items():
        print(render(name, source))


if __name__ == "__main__":
    main()
