#!/usr/bin/env python3
"""Generate Redis README diagrams in the bluetape4k diagram family."""

from __future__ import annotations

import argparse
import base64
from html import escape
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "docs/images/readme-diagrams"
ICON_ROOT = Path("/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/icons")
ICONS = {
    "database": ICON_ROOT / "generic/database-server.svg",
    "docker": ICON_ROOT / "testcontainers/generic/docker.svg",
    "kafka": ICON_ROOT / "testcontainers/mq/apache-kafka.svg",
    "mysql": ICON_ROOT / "testcontainers/database/mysql.svg",
    "nats": ICON_ROOT / "testcontainers/mq/nats.svg",
    "postgres": ICON_ROOT / "testcontainers/database/postgresql.svg",
    "queue": ICON_ROOT / "generic/message-queue.svg",
    "redis": ICON_ROOT / "redis/redis-icon.svg",
}

FONT_TITLE = "Architects Daughter"
FONT_BODY = "Comic Mono"

COLORS = {
    "canvas": "#f6f9fc",
    "frame": "#ffffff",
    "frame_stroke": "#c4d5e8",
    "text": "#263a57",
    "muted": "#526276",
    "line": "#c5d4e3",
    "blue": "#4f77c9",
    "green": "#319873",
    "amber": "#cf8428",
    "red": "#c85d73",
    "purple": "#7e62d9",
    "teal": "#3f7d9c",
    "gray": "#64748b",
}


def esc(value: str) -> str:
    return escape(value, quote=False)


def icon_data(name: str) -> str:
    data = base64.b64encode(ICONS[name].read_bytes()).decode("ascii")
    return f"data:image/svg+xml;base64,{data}"


ICON_DATA = {name: icon_data(name) for name in ICONS}


def write(path: Path, lines: list[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines) + "\n")
    print(path.relative_to(ROOT))


def marker_defs() -> list[str]:
    lines = ["  <defs>"]
    lines.append(
        '    <filter id="softShadow" x="-8%" y="-10%" width="116%" height="124%">'
        '<feDropShadow dx="0" dy="5" stdDeviation="5" flood-color="#0F172A" flood-opacity="0.10"/>'
        '</filter>'
    )
    for name, color in [
        ("blue", COLORS["blue"]),
        ("green", COLORS["green"]),
        ("amber", COLORS["amber"]),
        ("red", COLORS["red"]),
        ("purple", COLORS["purple"]),
        ("teal", COLORS["teal"]),
        ("gray", COLORS["gray"]),
    ]:
        lines.append(
            f'    <marker id="arrow-{name}" markerWidth="14" markerHeight="14" '
            f'refX="13" refY="7" orient="auto" markerUnits="userSpaceOnUse" '
            f'data-role="primary" data-tip-direction="positive-x">'
            f'<path d="M 0 0 L 14 7 L 0 14 Z" fill="{color}" stroke-dasharray="none"/></marker>'
        )
    lines.append(
        """    <style>
      .canvas { fill: #f8fafc; }
      .frame { fill: #ffffff; stroke: #cbd5e1; stroke-width: 1.5; filter: url(#softShadow); }
      .title, .part-title, .card-title, .panel-title { font-family: "Architects Daughter"; fill: #263a57; }
      .title { font-size: 42px; }
      .subtitle, .body, .small, .msg, .region, .footer { font-family: "Comic Mono"; fill: #526276; }
      .subtitle { font-size: 17px; }
      .panel-title { font-size: 24px; }
      .card-title { font-size: 22px; }
      .body { font-size: 14px; }
      .small { font-size: 12px; }
      .msg { font-size: 13px; font-weight: 700; }
      .region { font-size: 12px; font-weight: 700; }
      .footer { font-size: 14px; }
      .panel { stroke: #d0deeb; stroke-width: 1.8; rx: 8; filter: url(#softShadow); }
      .card { stroke-width: 1.8; rx: 8; filter: url(#softShadow); }
      .card-divider { stroke-width: 1.1; opacity: .45; }
      .participant { stroke-width: 2.2; rx: 12; }
      .lifeline { stroke: #c5d4e3; stroke-width: 2; stroke-dasharray: 8 8; }
      .activation { fill: #f3eeff; stroke: #8a65d8; stroke-width: 1.7; rx: 7; opacity: .88; }
      .route { fill: none; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
      .return { fill: none; stroke-width: 2.6; stroke-linecap: round; stroke-linejoin: round; stroke-dasharray: 9 8; }
      .alt { fill: #f7f2ff; fill-opacity: .22; stroke: #78909c; stroke-width: 2.4; stroke-dasharray: 12 7; rx: 16; }
      .alt-label { fill: #ffffff; stroke: #d7e0e4; stroke-width: 1.5; rx: 15; }
      .else-line { stroke: #78909c; stroke-width: 2.2; stroke-dasharray: 12 8; }
      .pill { fill: #ffffff; stroke: #cbd9e7; stroke-width: 1.5; rx: 14; }
      .badge { stroke-width: 1.6; }
      .badge-text { font-family: "Comic Mono"; font-size: 10px; font-weight: 700; fill: #ffffff; }
      .legend { fill: #ffffff; stroke: #d4e0ec; stroke-width: 1.5; rx: 12; }
    </style>"""
    )
    lines.append("  </defs>")
    return lines


def header(lines: list[str], width: int, height: int, title: str, subtitle: str) -> None:
    lines.append(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
        f'viewBox="0 0 {width} {height}" role="img" aria-labelledby="title desc">'
    )
    lines.append(f"  <title id=\"title\">{esc(title)}</title>")
    lines.append(f"  <desc id=\"desc\">{esc(subtitle)}</desc>")
    lines.extend(marker_defs())
    lines.append(f'  <rect class="canvas" width="{width}" height="{height}"/>')
    lines.append(f'  <rect class="frame" x="34" y="30" width="{width-68}" height="{height-64}" rx="8"/>')
    lines.append(f'  <text class="title" x="72" y="88">{esc(title)}</text>')
    lines.append(f'  <text class="subtitle" x="76" y="120">{esc(subtitle)}</text>')


def card(
    lines: list[str],
    x: int,
    y: int,
    w: int,
    h: int,
    title: str,
    body: list[str],
    fill: str,
    stroke: str,
    icon: str | bool = False,
) -> None:
    lines.append(f'  <rect class="card" x="{x}" y="{y}" width="{w}" height="{h}" rx="8" fill="{fill}" stroke="{stroke}"/>')
    if icon:
        icon_name = "redis" if icon is True else str(icon)
        lines.append(f'  <image href="{ICON_DATA[icon_name]}" x="{x+20}" y="{y+18}" width="46" height="46"/>')
        title_x = x + w // 2 + 20
    else:
        title_x = x + w // 2
    lines.append(f'  <text class="card-title" x="{title_x}" y="{y+42}" text-anchor="middle">{esc(title)}</text>')
    lines.append(f'  <path class="card-divider" d="M{x} {y+62}H{x+w}" stroke="{stroke}"/>')
    split_y = y + min(h - 36, 132)
    if len(body) > 2 and h >= 165:
        lines.append(f'  <path class="card-divider" d="M{x} {split_y}H{x+w}" stroke="{stroke}"/>')
    for i, text in enumerate(body):
        y_offset = y + 90 + i * 24
        if len(body) > 2 and i >= 2 and h >= 165:
            y_offset += 18
        lines.append(f'  <text class="body" x="{x+24}" y="{y_offset}">{esc(text)}</text>')


def path(lines: list[str], d: str, color: str, marker: str, dashed: bool = False) -> None:
    cls = "return" if dashed else "route"
    lines.append(f'  <path class="{cls}" d="{d}" stroke="{color}" marker-end="url(#{marker})"/>')


def label(lines: list[str], x: int, y: int, text: str, color: str) -> None:
    lines.append(f'  <text class="msg" x="{x}" y="{y}" text-anchor="middle" fill="{color}">{esc(text)}</text>')


def sequence_label(lines: list[str], x: int, y: int, text: str, color: str) -> None:
    match = re.match(r"^(\d+[a-z]?)\.\s*(.+)$", text)
    if match:
        step, body = match.groups()
    else:
        step, body = "", text
    width = max(132, min(420, 46 + len(body) * 8))
    left = x - width // 2
    lines.append(f'  <rect class="pill" x="{left}" y="{y-28}" width="{width}" height="30" rx="15"/>')
    if step:
        lines.append(f'  <circle class="badge" cx="{left+18}" cy="{y-13}" r="13" fill="{color}"/>')
        lines.append(f'  <text class="badge-text" x="{left+18}" y="{y-9}" text-anchor="middle">{esc(step)}</text>')
        text_x = left + 38 + (width - 46) // 2
    else:
        text_x = x
    lines.append(f'  <text class="msg" x="{text_x}" y="{y-8}" text-anchor="middle" fill="{color}">{esc(body)}</text>')


def footer(lines: list[str], width: int, height: int, text: str) -> None:
    lines.append(f'  <rect class="legend" x="82" y="{height-78}" width="{width-164}" height="40" rx="8"/>')
    lines.append(f'  <text class="footer" x="{width//2}" y="{height-52}" text-anchor="middle">{esc(text)}</text>')
    lines.append("</svg>")


def static_diagram(name: str, title: str, subtitle: str, cards: list[dict], arrows: list[dict], footer_text: str) -> None:
    width, height = 1500, 920
    lines: list[str] = []
    header(lines, width, height, title, subtitle)
    for item in cards:
        card(lines, **item)
    for item in arrows:
        path(lines, item["d"], item["color"], item["marker"], item.get("dashed", False))
        if "label" in item:
            label(lines, item["lx"], item["ly"], item["label"], item["color"])
    footer(lines, width, height, footer_text)
    write(OUT / f"{name}.svg", lines)


def sequence_diagram(name: str, title: str, subtitle: str, participants: list[dict], messages: list[dict], regions: list[dict], footer_text: str) -> None:
    max_message_y = max((message["y"] for message in messages), default=560)
    max_region_y = max((region["y"] + region["h"] for region in regions), default=560)
    width, height = 1500, max(920, max_message_y + 170, max_region_y + 150)
    lines: list[str] = []
    header(lines, width, height, title, subtitle)
    top = 174
    bottom = height - 126
    for p in participants:
        x = p["x"]
        w = p.get("w", 160)
        lines.append(f'  <rect class="participant" x="{x-w//2}" y="{top}" width="{w}" height="68" fill="{p["fill"]}" stroke="{p["stroke"]}"/>')
        lines.append(f'  <text class="part-title" x="{x}" y="{top+28}" text-anchor="middle">{esc(p["title"])}</text>')
        lines.append(f'  <text class="small" x="{x}" y="{top+50}" text-anchor="middle">{esc(p["role"])}</text>')
        lines.append(f'  <line class="lifeline" x1="{x}" y1="{top+68}" x2="{x}" y2="{bottom}"/>')
    for p in participants:
        x = p["x"]
        touched = [m["y"] for m in messages if m["from"] == x or m["to"] == x]
        if len(touched) >= 2:
            lines.append(f'  <rect class="activation" x="{x-7}" y="{min(touched)-12}" width="14" height="{max(touched)-min(touched)+44}" rx="7"/>')
    for r in regions:
        lines.append(f'  <rect class="alt" x="78" y="{r["y"]}" width="1276" height="{r["h"]}" rx="16"/>')
        lines.append(f'  <rect class="alt-label" x="96" y="{r["y"]+18}" width="{max(150, len(r["title"]) * 9 + 48)}" height="30" rx="15"/>')
        lines.append(f'  <text class="region" x="118" y="{r["y"]+38}">{esc(r["title"])}</text>')
        if "else_y" in r:
            lines.append(f'  <line class="else-line" x1="92" y1="{r["else_y"]}" x2="1340" y2="{r["else_y"]}"/>')
            lines.append(f'  <rect class="alt-label" x="96" y="{r["else_y"]+18}" width="{max(150, len(r["else_title"]) * 9 + 48)}" height="30" rx="15"/>')
            lines.append(f'  <text class="region" x="118" y="{r["else_y"]+38}">{esc(r["else_title"])}</text>')
    for msg in messages:
        y = msg["y"]
        source = msg["from"]
        target = msg["to"]
        color = msg["color"]
        marker = msg["marker"]
        dashed = msg.get("return", False)
        if source <= target:
            d = f"M{source} {y} L{target} {y}"
        else:
            d = f"M{source} {y} L{target} {y}"
        path(lines, d, color, marker, dashed)
        sequence_label(lines, (source + target) // 2, y - 14, msg["text"], color)
    footer(lines, width, height, footer_text)
    write(OUT / f"{name}.svg", lines)


def build_static() -> None:
    static_diagram(
        "core-helper-boundary-map",
        "core Helper Boundary Map",
        "Narrow helpers cover repeated validation, pointer, zero/default, string, and number checks.",
        [
            dict(x=92, y=205, w=330, h=180, title="Caller code", body=["prefer standard library", "use helpers for repeated gaps", "errors returned"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="core package", body=["validation helpers", "Ptr / ValueOr", "Zero and defaults", "UTF-8 truncation"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="Number/string checks", body=["Clamp bounds", "prefixed hex validation", "blank/default text", "no hidden decoding"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=500, y=545, w=390, h=165, title="Error boundary", body=["no panic contracts", "invalid input is error", "caller handles fallback"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="Package rule", body=["small shared surface", "no catch-all utilities", "stdlib first"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="call", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="specific helper", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["purple"], marker="arrow-purple", label="errors", lx=742, ly=484),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="scope", lx=950, ly=602),
        ],
        "Keep core boring and narrow; add helpers only when repeated service code becomes clearer.",
    )
    static_diagram(
        "collections-transform-pipeline",
        "collections Transform Pipeline",
        "Generic slice and map helpers keep common transformations explicit while preserving first-error behavior.",
        [
            dict(x=92, y=205, w=330, h=180, title="Input data", body=["slice or map", "comparable values", "key functions"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="Transform helpers", body=["Chunk / ChunkBy", "GroupBy / CountBy", "Distinct / DistinctBy", "MapErr / FilterErr"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="Result shape", body=["ordered slices", "map buckets", "first-seen distinct", "typed errors"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=500, y=545, w=390, h=165, title="Validation", body=["positive chunk size", "non-nil functions", "nil input allowed when sensible"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="Boundary", body=["no lazy streams", "no concurrency", "stdlib when enough"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="transform", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="emit", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["purple"], marker="arrow-purple", label="guard", lx=742, ly=484),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="scope", lx=950, ly=602),
        ],
        "Use collections for small eager transforms; keep large or streaming workflows in caller code.",
    )
    static_diagram(
        "codec-encoding-surface-map",
        "codec Encoding Surface Map",
        "Byte and string helpers expose stable alphabets for identifiers, keys, URLs, and interoperability tests.",
        [
            dict(x=92, y=205, w=330, h=180, title="Payload", body=["[]byte or UTF-8 string", "identifier bytes", "Redis key material"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="codec package", body=["Base58 Bitcoin alphabet", "Base62 / URL62", "Base64 URL", "hex helpers"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="Encoded text", body=["URL-safe tokens", "compact IDs", "leading zeros preserved", "decode validates alphabet"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=500, y=545, w=390, h=165, title="Compatibility", body=["bluetape4k-core vectors", "Kotlin numeric UUID caveat", "byte API is Go-owned"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="Boundary", body=["no serialization metadata", "no encryption", "malformed input returns error"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="encode/decode", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="text", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["purple"], marker="arrow-purple", label="vectors", lx=742, ly=484),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="limits", lx=950, ly=602),
        ],
        "Codec helpers preserve bytes; callers own serialization, signing, and encryption boundaries.",
    )
    static_diagram(
        "measure-unit-runtime-map",
        "measure Unit Runtime Map",
        "Typed dimensions, unit registries, compound units, and affine temperature helpers stay under one Go API.",
        [
            dict(x=92, y=205, w=330, h=180, title="Caller", body=["Measure[D]", "Parse user text", "Format output"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="Unit[D]", body=["name and suffix", "ratio to base unit", "spacing metadata", "immutable value"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="Registries", body=["Length / Time / Mass", "Storage / BinarySize", "Energy / Power", "Velocity / Acceleration"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=500, y=545, w=390, h=165, title="Operations", body=["In / As / Format", "Mul / Div compound", "sentinel errors"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="Temperature", body=["Kelvin absolute value", "Celsius/Fahrenheit affine", "delta is separate"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="construct", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="lookup", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["purple"], marker="arrow-purple", label="convert", lx=742, ly=484),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="affine", lx=950, ly=602),
        ],
        "Use measure for physical units; use money for currency and decimal exactness.",
    )
    static_diagram(
        "cache-contract-overview",
        "cache Package Contract Map",
        "Process-local TTL storage owns values; Redis wrappers add cross-process coordination when needed.",
        [
            dict(x=92, y=210, w=330, h=180, title="Caller", body=["Get / Set / Delete", "GetOrLoad(ctx, key)", "explicit TTL per value"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=520, y=180, w=390, h=230, title="Memory cache", body=["map guarded by mutex", "expired values miss", "same-key loader collapse", "no external invalidation"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1028, y=170, w=370, h=235, title="LoadingCache contract", body=["loader receives context", "loader errors are returned", "ErrCacheMiss is sentinel", "negative TTL rejected"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=320, y=545, w=360, h=165, title="redisnear", body=["local values stay local", "Pub/Sub invalidates peers", "no shared value store"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=865, y=545, w=380, h=165, title="rediscoord", body=["load token suppresses bursts", "short result envelope", "not a durable L2 cache"], fill="#fff0f0", stroke="#dc2626", icon=True),
        ],
        [
            dict(d="M422 300H520", color=COLORS["blue"], marker="arrow-blue", label="local ops", lx=470, ly=282),
            dict(d="M910 292H1028", color=COLORS["green"], marker="arrow-green", label="contract", lx=969, ly=274),
            dict(d="M590 410C560 475 500 520 430 545", color=COLORS["purple"], marker="arrow-purple", label="invalidation", lx=522, ly=486),
            dict(d="M745 410C785 475 855 520 955 545", color=COLORS["red"], marker="arrow-red", label="stampede guard", lx=828, ly=486),
        ],
        "Use the base cache when one process owns the value; add Redis wrappers only for cross-process coordination.",
    )
    static_diagram(
        "leader-contract-overview",
        "leader Package Contract Map",
        "The base package defines election contracts; Redis owns the concrete TTL-backed runtime state.",
        [
            dict(x=96, y=210, w=325, h=180, title="Member code", body=["Campaign(ctx)", "RunIfLeader style work", "Resign(ctx)"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=515, y=175, w=390, h=245, title="Elector contract", body=["one member per group", "IsLeader is local truth", "Leader reads backend token", "sentinel errors via errors.Is"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1020, y=175, w=380, h=245, title="Strategy layer", body=["GroupElector MaxLeaders", "candidate registry", "FIFO / random / scored", "weighted scorer compose"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=515, y=535, w=390, h=160, title="Backend boundary", body=["renew failure clears leader", "release only own token", "context cancellation exits"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=1020, y=535, w=380, h=160, title="leader/redis", body=["single key + TTL", "group ZSET slots", "Lua token checks"], fill="#fff0f0", stroke="#dc2626", icon=True),
        ],
        [
            dict(d="M421 302H515", color=COLORS["blue"], marker="arrow-blue", label="implements", lx=468, ly=284),
            dict(d="M905 298H1020", color=COLORS["purple"], marker="arrow-purple", label="extends", lx=962, ly=280),
            dict(d="M710 420V535", color=COLORS["amber"], marker="arrow-amber", label="runtime", lx=748, ly=482),
            dict(d="M905 615H1020", color=COLORS["red"], marker="arrow-red", label="Redis backend", lx=962, ly=598),
            dict(d="M1210 535C1210 470 1210 445 1210 420", color=COLORS["gray"], marker="arrow-gray", label="strategy state", lx=1262, ly=480, dashed=True),
        ],
        "Keep core leader code backend-neutral; choose leader/redis when ownership must survive process boundaries.",
    )
    static_diagram(
        "ratelimit-local-runtime-flow",
        "ratelimit Local Runtime Map",
        "A keyed in-process token bucket returns explicit Result values; HTTP middleware maps those results to responses.",
        [
            dict(x=92, y=210, w=320, h=175, title="Caller / HTTP request", body=["Allow(ctx, key, n)", "RemoteAddr default key", "custom KeyFunc for tenants"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=180, w=390, h=240, title="TokenBucket", body=["per-key bucket state", "mutex protects map", "refill by elapsed time", "IdleTTL prunes inactive keys"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=180, w=390, h=240, title="Result", body=["Allowed true/false", "Remaining tokens", "RetryAfter when rejected", "ResetAfter until full"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=500, y=545, w=390, h=165, title="HTTP middleware", body=["allowed delegates next", "429 when rejected", "503 for key/backend errors"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="Operational boundary", body=["state is process-local", "no FIFO fairness guarantee", "use ratelimit/redis to share buckets"], fill="#fff0f0", stroke="#dc2626", icon=True),
        ],
        [
            dict(d="M412 300H500", color=COLORS["blue"], marker="arrow-blue", label="request", lx=456, ly=282),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="result", lx=950, ly=282),
            dict(d="M695 420V545", color=COLORS["purple"], marker="arrow-purple", label="ServeHTTP", lx=750, ly=492),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="when shared", lx=950, ly=602),
            dict(d="M310 385C355 470 430 535 500 585", color=COLORS["gray"], marker="arrow-gray", label="middleware path", lx=382, ly=496, dashed=True),
        ],
        "Rejected attempts are normal Result values; errors mean invalid input, context cancellation, or backend failure.",
    )
    static_diagram(
        "resilience-policy-chain-flow",
        "resilience Policy Chain Map",
        "Typed operations are wrapped by ordered policies; events run synchronously on the protected call path.",
        [
            dict(x=90, y=215, w=320, h=170, title="Operation[T]", body=["func(context.Context)", "returns T, error", "nil context becomes Background"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=180, w=390, h=245, title="Compose / Run", body=["first policy is outermost", "applies wrappers in order", "Run executes wrapped op", "typed result preserved"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=170, w=390, h=250, title="Policy set", body=["Retry with backoff", "Timeout with own deadline", "Circuit breaker state", "Bulkhead admission limit"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=500, y=545, w=390, h=165, title="HTTP adapters", body=["RoundTripper for clients", "Handler for servers", "retry only replayable bodies"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=1010, y=545, w=390, h=165, title="Event boundary", body=["OnEvent is synchronous", "keep handlers non-blocking", "no exporter bundled"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M410 300H500", color=COLORS["blue"], marker="arrow-blue", label="wrap", lx=455, ly=282),
            dict(d="M890 300H1010", color=COLORS["purple"], marker="arrow-purple", label="ordered policies", lx=950, ly=282),
            dict(d="M695 425V545", color=COLORS["amber"], marker="arrow-amber", label="adapters", lx=742, ly=492),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="events", lx=950, ly=602),
            dict(d="M1205 545C1205 480 1205 455 1205 420", color=COLORS["gray"], marker="arrow-gray", label="call path", lx=1255, ly=486, dashed=True),
        ],
        "Composition protects one operation; observability hooks must not become the slowest policy.",
    )
    static_diagram(
        "batch-step-job-flow",
        "batch Step and Job Flow",
        "A Job runs Steps sequentially; each Step reads, processes, chunks, writes, and checkpoints progress.",
        [
            dict(x=92, y=205, w=330, h=180, title="Job", body=["ordered Runner list", "stops on failure/cancel", "children reports retained"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=520, y=175, w=390, h=240, title="Step[I,O]", body=["Reader -> Processor", "Writer receives chunks", "RetryPolicy and SkipPolicy", "CheckpointStore optional"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1030, y=180, w=370, h=230, title="Report", body=["completed / failed", "cancelled / partial", "read/write/skip counts", "first error preserved"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=312, y=545, w=370, h=165, title="Checkpoint", body=["restore before read", "save after writes", "reader owns position"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=865, y=545, w=390, h=165, title="Resource cleanup", body=["open reader/writer", "close with WithoutCancel", "close error can fail report"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H520", color=COLORS["blue"], marker="arrow-blue", label="Run(ctx)", lx=470, ly=282),
            dict(d="M910 300H1030", color=COLORS["green"], marker="arrow-green", label="finish", lx=970, ly=282),
            dict(d="M690 415C645 475 570 520 490 545", color=COLORS["purple"], marker="arrow-purple", label="restore/save", lx=595, ly=494),
            dict(d="M720 415C760 475 835 520 955 545", color=COLORS["red"], marker="arrow-red", label="defer close", lx=820, ly=494),
        ],
        "Use batch for deterministic chunk-oriented work; reports explain exactly where execution stopped.",
    )
    static_diagram(
        "concurrency-package-map",
        "concurrency Package Map",
        "Small helpers coordinate goroutines, parallel calls, worker pools, cancellation, and collected errors.",
        [
            dict(x=92, y=205, w=330, h=180, title="Caller", body=["context controls lifetime", "tasks return errors", "panic becomes failure"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="Group / Parallel", body=["run functions together", "collect first/all errors", "cancel sibling work", "bounded context support"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="WorkerPool", body=["fixed worker count", "submit tasks", "close waits for workers", "shared error channel"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=500, y=545, w=390, h=165, title="Error boundary", body=["ErrClosed / ErrInvalid", "errors.Is friendly", "caller decides retry"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=1010, y=545, w=390, h=165, title="Operational boundary", body=["no hidden scheduler", "no durable queue", "process-local coordination"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="spawn", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["purple"], marker="arrow-purple", label="pool path", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["amber"], marker="arrow-amber", label="errors", lx=744, ly=484),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="scope", lx=950, ly=602),
        ],
        "The package coordinates local goroutines; use a real queue when work must survive process exit.",
    )
    static_diagram(
        "testing-concurrency-harness-map",
        "testing/concurrency Harness Map",
        "Test helpers run repeated concurrent tasks and return a structured report instead of sleep-based guesses.",
        [
            dict(x=92, y=205, w=330, h=180, title="Test case", body=["Task functions", "rounds and workers", "timeout per run"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="Runner", body=["build round/task units", "run workers", "recover panics", "collect task errors"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="Report", body=["duration", "completed count", "errors collected", "RunError summarises"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=500, y=545, w=390, h=165, title="GoroutineStressTester", body=["stress same code path", "race detector friendly", "assert stable invariants"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="AsyncJobTester", body=["async completion checks", "context cancellation", "bounded waiting"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="configure", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="report", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["purple"], marker="arrow-purple", label="stress", lx=744, ly=484),
            dict(d="M890 620H1010", color=COLORS["red"], marker="arrow-red", label="async", lx=950, ly=602),
        ],
        "Prefer these harnesses over ad-hoc sleeps when proving concurrent Go behavior.",
    )
    static_diagram(
        "testcontainers-helper-flow",
        "testcontainers Helper Flow",
        "Small Start helpers launch service containers, register cleanup, and return connection endpoints for tests.",
        [
            dict(x=92, y=205, w=330, h=180, title="Go test", body=["context from test", "testing.TB cleanup", "integration code uses endpoint"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=175, w=390, h=235, title="Start helper", body=["module-specific image", "wait strategy", "endpoint extraction", "t.Cleanup terminate"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=175, w=390, h=235, title="Docker container", body=["Kafka brokers", "DB DSN", "NATS URL", "Redis address"], fill="#fff4de", stroke="#d59c3f", icon="docker"),
            dict(x=260, y=545, w=310, h=165, title="Queue services", body=["Kafka", "NATS", "broker URL returned"], fill="#f3efff", stroke="#9b7aec", icon="queue"),
            dict(x=640, y=545, w=310, h=165, title="Databases", body=["MySQL", "PostgreSQL", "DSN returned"], fill="#fff0f0", stroke="#dc2626", icon="database"),
            dict(x=1020, y=545, w=310, h=165, title="Redis", body=["redis:7-alpine", "host:port returned", "used by Redis modules"], fill="#fff0f0", stroke="#dc2626", icon="redis"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="Start(ctx,t)", lx=452, ly=274),
            dict(d="M890 300H1010", color=COLORS["green"], marker="arrow-green", label="launch", lx=950, ly=282),
            dict(d="M630 410C560 490 450 525 360 545", color=COLORS["purple"], marker="arrow-purple", label="mq", lx=500, ly=492),
            dict(d="M695 410V545", color=COLORS["red"], marker="arrow-red", label="sql", lx=742, ly=492),
            dict(d="M760 410C835 490 945 525 1105 545", color=COLORS["red"], marker="arrow-red", label="cache", lx=920, ly=492),
        ],
        "The helpers own container lifecycle; tests should consume returned endpoints and avoid global ports.",
    )
    static_diagram(
        "jwt-redis-facade-map",
        "jwt/redis Facade Map",
        "This subpackage exposes the Redis repository aliases while the jwt package owns rotation and parsing logic.",
        [
            dict(x=92, y=210, w=330, h=175, title="Application", body=["imports jwt/redis", "builds Options", "receives Repository"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=180, w=390, h=230, title="jwt/redis", body=["Options alias", "Repository alias", "New delegates to jwt", "small import boundary"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1010, y=170, w=390, h=250, title="jwt RedisRepository", body=["KeyChain storage", "current kid metadata", "retention order", "CAS-style scripts"], fill="#fff0f0", stroke="#dc2626", icon="redis"),
            dict(x=500, y=545, w=390, h=165, title="Provider layer", body=["DistributedProvider", "ComposeContext", "ParseContext"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=545, w=390, h=165, title="Operational boundary", body=["Redis stores signing keys", "reset is explicit", "retention protects readers"], fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="New", lx=460, ly=282),
            dict(d="M890 300H1010", color=COLORS["red"], marker="arrow-red", label="delegate", lx=950, ly=282),
            dict(d="M695 410V545", color=COLORS["purple"], marker="arrow-purple", label="provider uses", lx=750, ly=492),
            dict(d="M890 620H1010", color=COLORS["amber"], marker="arrow-amber", label="runtime rule", lx=950, ly=602),
        ],
        "Use jwt/redis when callers want a Redis-specific import path without depending on internal repository types.",
    )
    static_diagram(
        "probabilistic-redis-bloom-runtime",
        "probabilistic/redis Runtime Map",
        "Bloom filters and HyperLogLog use core Redis commands; Cuckoo remains RedisBloom-module follow-up scope.",
        [
            dict(x=92, y=235, w=330, h=185, title="Caller", body=["Bloom membership", "HLL distinct count", "context-owned calls"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=500, y=170, w=390, h=215, title="BloomFilter", body=["NewStringBloomFilter", "hashes values to offsets", "checks config fingerprint", "maps script errors"], fill="#edf9f4", stroke="#63b891"),
            dict(x=500, y=520, w=390, h=180, title="HyperLogLog", body=["NewStringHyperLogLog", "SHA-256 value digests", "Add / Count / Merge"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1010, y=160, w=390, h=220, title="Redis core commands", body=["Lua EVAL / EVALSHA", "PFADD / PFCOUNT / PFMERGE", "no RedisBloom module"], fill="#fff0f0", stroke="#dc2626", icon="redis"),
            dict(x=1010, y=455, w=390, h=165, title="Bloom keys", body=[":config hash metadata", ":bits bitmap string", "Cluster hash-tag slot"], fill="#fff4de", stroke="#d59c3f"),
            dict(x=1010, y=690, w=390, h=120, title="HLL key", body=["one HyperLogLog string", "approximate cardinality"], fill="#edf7ff", stroke="#3f7d9c"),
        ],
        [
            dict(d="M422 300H500", color=COLORS["blue"], marker="arrow-blue", label="Bloom API", lx=460, ly=282),
            dict(d="M422 360H462Q482 360 482 380V590Q482 610 492 610H500", color=COLORS["purple"], marker="arrow-purple", label="HLL API", lx=452, ly=538),
            dict(d="M890 280H1010", color=COLORS["red"], marker="arrow-red", label="scripts", lx=950, ly=262),
            dict(d="M890 610H950Q970 610 970 590V325Q970 305 990 305H1010", color=COLORS["green"], marker="arrow-green", label="PF*", lx=940, ly=520),
            dict(d="M890 315H950Q970 315 970 335V535Q970 555 990 555H1010", color=COLORS["amber"], marker="arrow-amber", label="metadata + bits", lx=830, ly=435),
            dict(d="M890 676H950Q970 676 970 696V742Q970 762 990 762H1010", color=COLORS["teal"], marker="arrow-teal", label="estimate key", lx=950, ly=724),
        ],
        "Bloom and HLL require only core Redis; RedisBloom CF* Cuckoo support is intentionally outside this scope.",
    )
    static_diagram(
        "redis-leader-election-lifecycle",
        "Redis Leader Election Runtime Map",
        "TTL ownership supports single, bounded group, and strategy-selected Go leaders.",
        [
            dict(x=92, y=198, w=310, h=165, title="Member instance", body=["Campaign(ctx)", "memberID:random token", "Resign releases owner"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=502, y=170, w=420, h=230, title="Redis ownership", body=["single key + TTL", "group ZSET slots", "strategy candidate index", "expired entries pruned"], fill="#fff8ed", stroke="#cf8428", icon=True),
            dict(x=1038, y=198, w=330, h=165, title="Renew loop", body=["PEXPIRE if token matches", "lost owner clears local state", "IsLeader becomes false"], fill="#edf9f4", stroke="#63b891"),
            dict(x=502, y=510, w=420, h=170, title="Strategic path", body=["list live candidates", "deterministic strategy", "run only selected member"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1038, y=510, w=330, h=170, title="Boundary", body=["Go-owned key format", "no JVM compatibility", "context controls waiting"], fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(d="M402 280 L502 280", color=COLORS["blue"], marker="arrow-blue", label="acquire", lx=452, ly=260),
            dict(d="M922 280 L1038 280", color=COLORS["green"], marker="arrow-green", label="renew", lx=980, ly=260),
            dict(d="M1038 340 L922 340", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="lost owner", lx=980, ly=368),
            dict(d="M712 400 L712 510", color=COLORS["blue"], marker="arrow-blue", label="rank", lx=745, ly=458),
            dict(d="M922 595 L1038 595", color=COLORS["purple"], marker="arrow-purple", label="runbook", lx=980, ly=575),
        ],
        "Release and renew mutate Redis only when the stored owner token still matches this member.",
    )
    static_diagram(
        "redis-lock-owner-token-lifecycle",
        "Redis Lock Owner-Token Runtime Map",
        "One SET NX key protects short coordination work; owner-safe Lua unlock prevents deleting another lease.",
        [
            dict(x=92, y=210, w=310, h=150, title="Caller", body=["TryLock(ctx)", "one acquire attempt", "context preserved"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=522, y=190, w=390, h=190, title="Mutex", body=["SET NX key token TTL", "random or custom token", "ErrNotAcquired on conflict"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1038, y=190, w=330, h=190, title="Redis key", body=["value = owner token", "TTL cleans abandoned work", "single instance only"], fill="#fff0f0", stroke="#dc2626", icon=True),
            dict(x=522, y=530, w=390, h=145, title="Lease", body=["Key() and Token()", "Unlock(ctx) returns bool"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1038, y=530, w=330, h=145, title="Unlock script", body=["GET key == token", "DEL only for owner", "mismatch leaves key intact"], fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(d="M402 285 L522 285", color=COLORS["blue"], marker="arrow-blue", label="try", lx=462, ly=265),
            dict(d="M912 285 L1038 285", color=COLORS["green"], marker="arrow-green", label="SET NX", lx=975, ly=265),
            dict(d="M1203 380 C1070 455 912 485 717 530", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="lease token", lx=900, ly=485),
            dict(d="M912 602 L1038 602", color=COLORS["blue"], marker="arrow-blue", label="Lua unlock", lx=975, ly=582),
            dict(d="M522 628 C360 690 238 540 247 360", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="released or not owner", lx=520, ly=730),
        ],
        "This is not Redlock and does not issue fencing tokens; choose TTL or compose renewal above the package.",
    )
    static_diagram(
        "redis-ratelimit-token-bucket-flow",
        "Redis Token Bucket Runtime Map",
        "Each Allow call runs one Lua script: server TIME, refill, consume, persist, and refresh IdleTTL.",
        [
            dict(x=92, y=205, w=300, h=170, title="Caller", body=["Allow(ctx, key, n)", "context deadline", "logical tenant key"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=512, y=180, w=360, h=235, title="Go limiter", body=["trim key", "MaxKeyBytes guard", "tokens <= Burst", "scale to microtokens"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1018, y=155, w=360, h=270, title="Redis Lua script", body=["TIME uses server clock", "HMGET tokens, updated_ms", "refill elapsed microtokens", "HSET state + PEXPIRE"], fill="#f3efff", stroke="#9b7aec", icon=True),
            dict(x=1018, y=560, w=360, h=150, title="Redis hash bucket", body=["tokens = remaining", "updated_ms = Redis timestamp", "IdleTTL bounds inactive key"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M392 290 L512 290", color=COLORS["blue"], marker="arrow-blue", label="request", lx=452, ly=270),
            dict(d="M872 290 L1018 290", color=COLORS["green"], marker="arrow-green", label="EVAL", lx=945, ly=270),
            dict(d="M1198 425 L1198 560", color=COLORS["green"], marker="arrow-green", label="HSET + PEXPIRE", lx=1270, ly=500),
            dict(d="M1018 635 C850 690 625 570 692 415", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="Result", lx=760, ly=585),
            dict(d="M512 370 C405 435 305 420 240 375", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="allowed or retry-after", lx=420, ly=455),
        ],
        "Rejected attempts are normal Result values; script failures return errors.",
    )
    static_diagram(
        "rediscoord-cold-burst-coordination",
        "rediscoord Cold Burst Runtime Map",
        "One owner loads a missing value while waiters reuse a token-bound result envelope.",
        [
            dict(x=92, y=190, w=300, h=175, title="Process A", body=["GetOrLoad sku:42", "local miss", "tries Redis lock"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=92, y=520, w=300, h=145, title="Process B", body=["same cold key", "waits for owner", "fills local cache"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=512, y=185, w=360, h=210, title="Wrapped cache", body=["Memory or redisnear", "owner uses GetOrLoad", "waiter fills via GetOrLoad"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1018, y=155, w=360, h=260, title="Redis coordination", body=["lock key owns token", "result key stores envelope", "short ResultTTL", "not a durable L2 cache"], fill="#fff0f0", stroke="#dc2626", icon=True),
            dict(x=1018, y=540, w=360, h=150, title="User loader", body=["runs once for winner", "payload encoded by Codec"], fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(d="M392 278 L512 278", color=COLORS["blue"], marker="arrow-blue", label="check", lx=452, ly=258),
            dict(d="M872 260 L1018 260", color=COLORS["green"], marker="arrow-green", label="lock", lx=945, ly=240),
            dict(d="M1198 415 L1198 540", color=COLORS["blue"], marker="arrow-blue", label="owner", lx=1260, ly=485),
            dict(d="M1018 610 C800 690 640 520 805 395", color=COLORS["blue"], marker="arrow-blue", label="store value", lx=820, ly=720),
            dict(d="M872 338 L1018 338", color=COLORS["blue"], marker="arrow-blue", label="publish envelope", lx=945, ly=318),
            dict(d="M392 592 C635 592 755 455 1018 338", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="poll token result", lx=610, ly=545),
            dict(d="M1018 300 C825 410 650 485 512 592", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="accept matching token", lx=760, ly=468),
        ],
        "If LockTTL expires before publication, another owner may run; waiters never trust mismatched tokens.",
    )
    static_diagram(
        "redis-jwt-distributed-key-rotation",
        "Redis JWT Distributed Key Runtime Map",
        "Instances converge on one current signing key through Redis repository scripts.",
        [
            dict(x=92, y=210, w=330, h=175, title="Service instance", body=["ComposeContext", "RotateContext", "ForcedRotateContext"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=512, y=180, w=380, h=240, title="DistributedProvider", body=["validates algorithm", "creates KeyChain", "uses explicit context", "retains old keys"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1018, y=160, w=380, h=260, title="Redis repository", body=["meta: version + alg", "current: current kid", "keys: kid payload hash", "order: retention zset"], fill="#fff0f0", stroke="#dc2626", icon=True),
            dict(x=512, y=535, w=380, h=165, title="Token validation", body=["ParseContext reads kid", "Find checks retained key", "expired keys rejected"], fill="#f3efff", stroke="#9b7aec"),
            dict(x=1018, y=560, w=380, h=140, title="Operator boundary", body=["KeyTTL >= lifetime + leeway", "Redis is trusted key storage"], fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(d="M422 295 L512 295", color=COLORS["blue"], marker="arrow-blue", label="request", lx=467, ly=275),
            dict(d="M892 292 L1018 292", color=COLORS["green"], marker="arrow-green", label="CAS/store", lx=955, ly=272),
            dict(d="M1018 365 L892 365", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="winner key", lx=955, ly=393),
            dict(d="M702 420 L702 535", color=COLORS["blue"], marker="arrow-blue", label="parse", lx=735, ly=485),
            dict(d="M892 620 L1018 620", color=COLORS["purple"], marker="arrow-purple", label="find kid", lx=955, ly=600),
            dict(d="M512 655 C360 625 270 505 257 385", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="signed token / Reader", lx=550, ly=744),
        ],
        "Reset is explicit only; DeleteKeyChainsContext is for tests/operator reset, not normal rollback.",
    )
    static_diagram(
        "redis-bloom-key-layout-01",
        "Redis Bloom Key Layout",
        "One hash-tagged namespace keeps bitmap bits and immutable config in the same Redis Cluster slot.",
        [
            dict(x=92, y=230, w=330, h=180, title="Bloom caller", body=["Put / MightContain", "BitCount / IsEmpty", "Clear is operator action"], fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=552, y=175, w=380, h=245, title="Lua script guard", body=["fingerprint required", "config mismatch fails", "config corrupt fails", "offsets from hasher"], fill="#edf9f4", stroke="#63b891"),
            dict(x=1058, y=155, w=350, h=230, title="{namespace}:config", body=["hash metadata", "version + family", "bit size + hash count", "hasher key + fingerprint"], fill="#f3efff", stroke="#9b7aec", icon=True),
            dict(x=1058, y=560, w=350, h=150, title="{namespace}:bits", body=["bitmap string", "GETBIT / SETBIT", "BITCOUNT / STRLEN"], fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(d="M422 322 L552 322", color=COLORS["blue"], marker="arrow-blue", label="operation", lx=487, ly=302),
            dict(d="M932 300 L1058 300", color=COLORS["blue"], marker="arrow-blue", label="check", lx=995, ly=280),
            dict(d="M932 360 C1010 445 1035 500 1058 600", color=COLORS["gray"], marker="arrow-gray", dashed=True, label="hash slot", lx=1000, ly=475),
            dict(d="M742 420 C825 525 945 605 1058 635", color=COLORS["blue"], marker="arrow-blue", label="read/write bits", lx=850, ly=550),
        ],
        "Namespaces are operational identifiers; do not put raw users, emails, secrets, or bearer tokens in them.",
    )


def build_sequences() -> None:
    sequence_diagram(
        "redis-leader-election-sequence",
        "Redis Leader Election Sequence",
        "Campaign, renewal, action ownership, release, and lazy group-slot cleanup happen in Redis time order.",
        [
            dict(x=150, title="Member", role="caller", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=360, title="Elector", role="Go API", fill="#fff4de", stroke="#d59c3f"),
            dict(x=590, title="Redis Lua", role="ownership", fill="#fff0f0", stroke="#dc2626"),
            dict(x=830, title="Redis keys", role="TTL/ZSET", fill="#f3efff", stroke="#9b7aec"),
            dict(x=1060, title="Action", role="user work", fill="#edf9f4", stroke="#63b891"),
            dict(x=1280, title="Contender", role="same group", fill="#f8fafc", stroke="#64748b"),
        ],
        [
            dict(y=310, **{"from":150, "to":360}, color=COLORS["blue"], marker="arrow-blue", text="1. Campaign(ctx)"),
            dict(y=370, **{"from":360, "to":590}, color=COLORS["amber"], marker="arrow-amber", text="2. SET NX PX or ZADD slot"),
            dict(y=430, **{"from":590, "to":830}, color=COLORS["green"], marker="arrow-green", text="3. store token + expiry"),
            dict(y=490, **{"from":590, "to":360}, color=COLORS["green"], marker="arrow-green", text="4a. acquired", **{"return": True}),
            dict(y=550, **{"from":360, "to":1060}, color=COLORS["green"], marker="arrow-green", text="5a. run elected work"),
            dict(y=650, **{"from":360, "to":590}, color=COLORS["amber"], marker="arrow-amber", text="6. renew/release if token matches"),
            dict(y=750, **{"from":1280, "to":590}, color=COLORS["purple"], marker="arrow-purple", text="7. next group acquire"),
            dict(y=810, **{"from":590, "to":830}, color=COLORS["purple"], marker="arrow-purple", text="8. prune expired slots first"),
        ],
        [dict(y=330, h=290, title="alt acquired", else_y=615, else_title="else conflict or waiting"), dict(y=720, h=140, title="lazy cleanup")],
        "Release and renew scripts mutate Redis only when the stored token still matches this member.",
    )
    sequence_diagram(
        "redis-lock-owner-token-sequence",
        "Redis Lock Owner-Token Sequence",
        "TryLock is one non-blocking SET NX attempt; Unlock deletes only the matching owner token.",
        [
            dict(x=180, title="Caller", role="protected work", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=430, title="Mutex", role="TryLock", fill="#edf9f4", stroke="#63b891"),
            dict(x=720, title="Redis key", role="token + TTL", fill="#fff0f0", stroke="#dc2626"),
            dict(x=1010, title="Lease", role="owner token", fill="#f3efff", stroke="#9b7aec"),
            dict(x=1260, title="Other owner", role="conflict", fill="#f8fafc", stroke="#64748b"),
        ],
        [
            dict(y=310, **{"from":180, "to":430}, color=COLORS["blue"], marker="arrow-blue", text="1. TryLock(ctx)"),
            dict(y=370, **{"from":430, "to":720}, color=COLORS["green"], marker="arrow-green", text="2. SET NX key token PX ttl"),
            dict(y=430, **{"from":720, "to":430}, color=COLORS["green"], marker="arrow-green", text="3a. OK", **{"return": True}),
            dict(y=490, **{"from":430, "to":1010}, color=COLORS["green"], marker="arrow-green", text="4a. return Lease"),
            dict(y=610, **{"from":1010, "to":720}, color=COLORS["amber"], marker="arrow-amber", text="5. Lua GET token then DEL"),
            dict(y=670, **{"from":720, "to":1010}, color=COLORS["amber"], marker="arrow-amber", text="6. deleted true/false", **{"return": True}),
            dict(y=790, **{"from":1260, "to":720}, color=COLORS["red"], marker="arrow-red", text="7. different token appears"),
            dict(y=850, **{"from":1010, "to":720}, color=COLORS["red"], marker="arrow-red", text="8. stale unlock leaves key", **{"return": True}),
        ],
        [dict(y=330, h=190, title="alt acquired", else_y=530, else_title="else ErrNotAcquired"), dict(y=760, h=130, title="mismatch safety")],
        "No renewal or blocking retry loop is hidden in this package.",
    )
    sequence_diagram(
        "redis-ratelimit-token-bucket-sequence",
        "Redis Token Bucket Sequence",
        "Allow validates input, evaluates one Lua script, and returns either allowed, rejected, or Redis error.",
        [
            dict(x=160, title="Caller", role="tenant key", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=390, title="Limiter", role="Go guard", fill="#edf9f4", stroke="#63b891"),
            dict(x=650, title="Redis Lua", role="atomic script", fill="#f3efff", stroke="#9b7aec"),
            dict(x=910, title="Redis TIME", role="server clock", fill="#fff4de", stroke="#d59c3f"),
            dict(x=1160, title="Hash bucket", role="tokens/updated_ms", fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(y=310, **{"from":160, "to":390}, color=COLORS["blue"], marker="arrow-blue", text="1. Allow(ctx, key, n)"),
            dict(y=370, **{"from":390, "to":650}, color=COLORS["purple"], marker="arrow-purple", text="2. EVAL allow script"),
            dict(y=430, **{"from":650, "to":910}, color=COLORS["amber"], marker="arrow-amber", text="3. TIME"),
            dict(y=490, **{"from":650, "to":1160}, color=COLORS["green"], marker="arrow-green", text="4. HMGET + refill + consume"),
            dict(y=550, **{"from":650, "to":1160}, color=COLORS["green"], marker="arrow-green", text="5. HSET + PEXPIRE"),
            dict(y=650, **{"from":650, "to":390}, color=COLORS["green"], marker="arrow-green", text="6a. allowed + remaining", **{"return": True}),
            dict(y=760, **{"from":650, "to":390}, color=COLORS["amber"], marker="arrow-amber", text="6b. rejected + retry-after", **{"return": True}),
            dict(y=820, **{"from":390, "to":160}, color=COLORS["blue"], marker="arrow-blue", text="7. ratelimit.Result", **{"return": True}),
        ],
        [dict(y=610, h=240, title="alt script result", else_y=720, else_title="else insufficient tokens")],
        "Concurrent clients for one key serialize inside Redis script execution.",
    )
    sequence_diagram(
        "rediscoord-cold-burst-sequence",
        "rediscoord Cold Burst Sequence",
        "The winner owns the load token; waiters accept only the matching result envelope.",
        [
            dict(x=135, title="Process A", role="winner", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=335, title="Process B", role="waiter", fill="#f3efff", stroke="#9b7aec"),
            dict(x=570, title="StampedeCache", role="wrapper", fill="#edf9f4", stroke="#63b891"),
            dict(x=820, title="Redis", role="lock/result", fill="#fff0f0", stroke="#dc2626"),
            dict(x=1080, title="Local cache", role="wrapped", fill="#fff4de", stroke="#d59c3f"),
            dict(x=1285, title="Loader", role="user func", fill="#f8fafc", stroke="#64748b"),
        ],
        [
            dict(y=310, **{"from":135, "to":570}, color=COLORS["blue"], marker="arrow-blue", text="1. GetOrLoad cold key"),
            dict(y=370, **{"from":570, "to":820}, color=COLORS["green"], marker="arrow-green", text="2. acquire load token"),
            dict(y=430, **{"from":820, "to":570}, color=COLORS["green"], marker="arrow-green", text="3a. winner token", **{"return": True}),
            dict(y=490, **{"from":570, "to":1080}, color=COLORS["blue"], marker="arrow-blue", text="4a. wrapped GetOrLoad"),
            dict(y=550, **{"from":1080, "to":1285}, color=COLORS["blue"], marker="arrow-blue", text="5a. run loader once"),
            dict(y=610, **{"from":570, "to":820}, color=COLORS["purple"], marker="arrow-purple", text="6a. publish envelope"),
            dict(y=730, **{"from":335, "to":570}, color=COLORS["amber"], marker="arrow-amber", text="1b. same cold key"),
            dict(y=790, **{"from":570, "to":820}, color=COLORS["amber"], marker="arrow-amber", text="2b. poll token result"),
            dict(y=850, **{"from":820, "to":570}, color=COLORS["amber"], marker="arrow-amber", text="3b. matching envelope", **{"return": True}),
            dict(y=910, **{"from":570, "to":1080}, color=COLORS["green"], marker="arrow-green", text="4b. fill local via GetOrLoad"),
        ],
        [dict(y=330, h=330, title="alt owner path", else_y=690, else_title="else waiter path")],
        "Redis envelope is transient coordination metadata, not a durable cache value.",
    )
    sequence_diagram(
        "redisnear-invalidation-sequence",
        "redisnear Invalidation Sequence",
        "Values stay local; Redis Pub/Sub carries only versioned invalidation commands.",
        [
            dict(x=150, title="Writer cache", role="process A", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=390, title="Local cache A", role="value store", fill="#edf9f4", stroke="#63b891"),
            dict(x=660, title="Redis Pub/Sub", role="invalidation bus", fill="#fff0f0", stroke="#dc2626"),
            dict(x=930, title="Subscriber", role="process B", fill="#f3efff", stroke="#9b7aec"),
            dict(x=1190, title="Local cache B", role="peer value", fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(y=310, **{"from":150, "to":390}, color=COLORS["blue"], marker="arrow-blue", text="1. Set/Delete/Clear"),
            dict(y=370, **{"from":390, "to":660}, color=COLORS["purple"], marker="arrow-purple", text="2. publish command"),
            dict(y=430, **{"from":660, "to":930}, color=COLORS["purple"], marker="arrow-purple", text="3. receive invalidation"),
            dict(y=490, **{"from":930, "to":1190}, color=COLORS["red"], marker="arrow-red", text="4. delete affected key"),
            dict(y=610, **{"from":1190, "to":930}, color=COLORS["gray"], marker="arrow-gray", text="5. next miss", **{"return": True}),
            dict(y=670, **{"from":930, "to":1190}, color=COLORS["green"], marker="arrow-green", text="6. local loader refills"),
        ],
        [dict(y=540, h=170, title="later read path")],
        "GetOrLoad fills only the local cache and does not publish invalidation.",
    )
    sequence_diagram(
        "redis-jwt-distributed-key-rotation-sequence",
        "Redis JWT Distributed Rotation Sequence",
        "Concurrent instances converge on one current kid, while retained keys remain available for validation.",
        [
            dict(x=150, title="Instance A", role="Compose", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=380, title="Provider", role="rotation logic", fill="#edf9f4", stroke="#63b891"),
            dict(x=650, title="Redis repo", role="CAS scripts", fill="#fff0f0", stroke="#dc2626"),
            dict(x=900, title="Key store", role="meta/current/keys", fill="#f3efff", stroke="#9b7aec"),
            dict(x=1160, title="Instance B", role="Parse", fill="#fff4de", stroke="#d59c3f"),
        ],
        [
            dict(y=310, **{"from":150, "to":380}, color=COLORS["blue"], marker="arrow-blue", text="1. ComposeContext"),
            dict(y=370, **{"from":380, "to":650}, color=COLORS["green"], marker="arrow-green", text="2. current missing/expired"),
            dict(y=430, **{"from":650, "to":900}, color=COLORS["green"], marker="arrow-green", text="3. CAS store fresh key"),
            dict(y=490, **{"from":900, "to":650}, color=COLORS["green"], marker="arrow-green", text="4. winner kid", **{"return": True}),
            dict(y=550, **{"from":380, "to":150}, color=COLORS["blue"], marker="arrow-blue", text="5. signed token with kid", **{"return": True}),
            dict(y=670, **{"from":1160, "to":650}, color=COLORS["purple"], marker="arrow-purple", text="6. FindKeyChainContext(kid)"),
            dict(y=730, **{"from":650, "to":900}, color=COLORS["purple"], marker="arrow-purple", text="7. read retained key"),
            dict(y=790, **{"from":650, "to":1160}, color=COLORS["purple"], marker="arrow-purple", text="8. verify key or ErrKeyNotFound", **{"return": True}),
        ],
        [dict(y=330, h=250, title="alt rotate if needed"), dict(y=640, h=190, title="validation path")],
        "DeleteKeyChainsContext is for test/operator reset, not normal rotation or rollback.",
    )
    sequence_diagram(
        "redis-bloom-operation-sequence",
        "Redis Bloom Operation Sequence",
        "Each operation validates immutable config before touching shared bitmap bits.",
        [
            dict(x=150, title="Caller", role="Put/MightContain", fill="#eaf2ff", stroke="#6d9df0"),
            dict(x=410, title="Bloom filter", role="hash offsets", fill="#edf9f4", stroke="#63b891"),
            dict(x=690, title="Lua script", role="guarded operation", fill="#f3efff", stroke="#9b7aec"),
            dict(x=960, title=":config hash", role="metadata", fill="#fff4de", stroke="#d59c3f"),
            dict(x=1230, title=":bits string", role="bitmap", fill="#fff0f0", stroke="#dc2626"),
        ],
        [
            dict(y=310, **{"from":150, "to":410}, color=COLORS["blue"], marker="arrow-blue", text="1. Put or MightContain"),
            dict(y=370, **{"from":410, "to":690}, color=COLORS["purple"], marker="arrow-purple", text="2. offsets + fingerprint"),
            dict(y=430, **{"from":690, "to":960}, color=COLORS["amber"], marker="arrow-amber", text="3. HGETALL config"),
            dict(y=490, **{"from":960, "to":690}, color=COLORS["amber"], marker="arrow-amber", text="4. metadata match", **{"return": True}),
            dict(y=570, **{"from":690, "to":1230}, color=COLORS["green"], marker="arrow-green", text="5a. SETBIT / GETBIT / BITCOUNT"),
            dict(y=650, **{"from":690, "to":410}, color=COLORS["green"], marker="arrow-green", text="6a. changed or maybe-present", **{"return": True}),
            dict(y=760, **{"from":690, "to":410}, color=COLORS["red"], marker="arrow-red", text="5b. ErrConfigMismatch/Corrupt", **{"return": True}),
        ],
        [dict(y=530, h=160, title="alt metadata valid", else_y=720, else_title="else config incident")],
        "Clear deletes shared bitmap state but preserves config metadata; treat it as operator action.",
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("target", choices=["static", "sequence", "all"])
    args = parser.parse_args()
    if args.target in {"static", "all"}:
        build_static()
    if args.target in {"sequence", "all"}:
        build_sequences()


if __name__ == "__main__":
    main()
