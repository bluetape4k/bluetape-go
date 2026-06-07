#!/usr/bin/env python3
"""
bluetape-go Architecture Overview SVG generator.
Fonts: Architects Daughter (titles/labels), Comic Mono (details/captions).
Layout: TB layered — Foundation -> Resilience -> Cache -> Workflow/Batch -> Portable -> Domain
"""

from html import escape
from pathlib import Path

ARCH_FONT = "Architects Daughter"
DETAIL_FONT = "Comic Mono"

W = 1320
H = 1100

PASTEL = {
    "foundation": "#EBF4FF",
    "foundation_border": "#93C5FD",
    "testing": "#F0FFF4",
    "testing_border": "#6EE7B7",
    "leader": "#FFF7ED",
    "leader_border": "#FCD34D",
    "resilience": "#FEF9C3",
    "resilience_border": "#FDE047",
    "cache": "#F0FDF4",
    "cache_border": "#86EFAC",
    "workflow": "#FAF5FF",
    "workflow_border": "#C4B5FD",
    "batch": "#FDF4FF",
    "batch_border": "#E879F9",
    "portable": "#FFF1F2",
    "portable_border": "#FDA4AF",
    "domain": "#F0F9FF",
    "domain_border": "#7DD3FC",
    "bg": "#F8FAFC",
    "frame": "#E2E8F0",
    "arrow": "#475569",
    "dashed": "#94A3B8",
    "section_label": "#1E293B",
    "text": "#1E293B",
    "subtext": "#334155",
}

def rect(x, y, w, h, fill, stroke, rx=10):
    return f'<rect x="{x}" y="{y}" width="{w}" height="{h}" fill="{fill}" stroke="{stroke}" stroke-width="1.5" rx="{rx}"/>'

def text_block(x, cy, lines, font, size, color, weight="normal", anchor="middle"):
    n = len(lines)
    lh = size * 1.35
    total = n * lh
    start_y = cy - total / 2 + lh / 2
    out = []
    for i, line in enumerate(lines):
        ty = start_y + i * lh
        out.append(f'<text x="{x}" y="{ty}" font-family="{font}" font-size="{size}" fill="{color}" font-weight="{weight}" text-anchor="{anchor}" dominant-baseline="middle">{escape(line, quote=False)}</text>')
    return "\n".join(out)

def section_label(x, y, label, color=PASTEL["section_label"]):
    return f'<text x="{x}" y="{y}" font-family="{ARCH_FONT}" font-size="13" fill="{color}" font-weight="bold" text-anchor="middle" dominant-baseline="middle">{escape(label, quote=False)}</text>'

def arrow(x1, y1, x2, y2, color=PASTEL["arrow"], dashed=False, marker="url(#arrowhead)"):
    dash = ' stroke-dasharray="6,4"' if dashed else ""
    return f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{color}" stroke-width="1.8"{dash} marker-end="{marker}"/>'

def ortho_arrow(x1, y1, x2, y2, color=PASTEL["arrow"], dashed=False):
    """Right-angle connector: vertical first, then horizontal, then vertical."""
    mid_y = (y1 + y2) / 2
    dash = ' stroke-dasharray="6,4"' if dashed else ""
    d = f"M {x1} {y1} L {x1} {mid_y} L {x2} {mid_y} L {x2} {y2}"
    marker = "url(#arrowhead-dashed)" if dashed else "url(#arrowhead)"
    return f'<path d="{d}" fill="none" stroke="{color}" stroke-width="1.8"{dash} marker-end="{marker}"/>'

DEFS = """
  <defs>
    <style>
      @font-face {
        font-family: 'Architects Daughter';
        src: url('/Users/debop/Library/Fonts/ArchitectsDaughter-Regular.ttf');
      }
      @font-face {
        font-family: 'Comic Mono';
        src: url('/Users/debop/Library/Fonts/ComicMono.ttf');
        font-weight: normal;
      }
      @font-face {
        font-family: 'Comic Mono';
        src: url('/Users/debop/Library/Fonts/ComicMono-Bold.ttf');
        font-weight: bold;
      }
    </style>
    <marker id="arrowhead" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
      <path d="M 1 1 L 7 4 L 1 7 Z" fill="%s"/>
    </marker>
    <marker id="arrowhead-dashed" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
      <path d="M 1 1 L 7 4 L 1 7 Z" fill="%s"/>
    </marker>
  </defs>
""" % (PASTEL["arrow"], PASTEL["dashed"])

def card(x, y, w, h, fill, border, title_lines, detail_lines=None, rx=10):
    out = [rect(x, y, w, h, fill, border, rx)]
    if detail_lines:
        cy_title = y + h * 0.36
        cy_detail = y + h * 0.68
        out.append(text_block(x + w/2, cy_title, title_lines, ARCH_FONT, 13, PASTEL["text"], "bold"))
        out.append(text_block(x + w/2, cy_detail, detail_lines, DETAIL_FONT, 10, PASTEL["subtext"]))
    else:
        out.append(text_block(x + w/2, y + h/2, title_lines, ARCH_FONT, 13, PASTEL["text"], "bold"))
    return "\n".join(out)

# ── Layout constants ──────────────────────────────────────────────────────────
PAD = 18
MARGIN = 30

# Row Y positions (top of each band)
Y_TITLE   = 22
Y_ROW1    = 85    # Foundation + Testing + Leader
Y_ROW2    = 310   # Resilience
Y_ROW3    = 430   # Cache & Coordination
Y_ROW4    = 545   # Workflow & Batch
Y_ROW5    = 670   # Portable Utilities
Y_ROW6    = 780   # Domain Packages

BAND_H1 = 200   # Foundation band height
BAND_H2 =  95
BAND_H3 =  95
BAND_H4 = 110
BAND_H5 =  90
BAND_H6 = 105

# ── Build SVG ─────────────────────────────────────────────────────────────────
parts = []
parts.append(f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}">')
parts.append(DEFS)

# Background + outer frame
parts.append(rect(0, 0, W, H, PASTEL["bg"], PASTEL["frame"], rx=16))

# ── Title ─────────────────────────────────────────────────────────────────────
parts.append(f'<text x="{W//2}" y="50" font-family="{ARCH_FONT}" font-size="28" fill="{PASTEL["text"]}" font-weight="bold" text-anchor="middle" dominant-baseline="middle">bluetape-go — Architecture Overview</text>')
parts.append(f'<text x="{W//2}" y="74" font-family="{DETAIL_FONT}" font-size="12" fill="#64748B" text-anchor="middle" dominant-baseline="middle">Go backend utilities · Roadmap v0.1 → v0.11</text>')

# ── ROW 1: Foundation | Testing | Leader ──────────────────────────────────────
# Foundation band: 6 cards * 105 + 5 gaps * 8 + 2 * 18 padding = 702
FX = MARGIN
FW = 706
parts.append(rect(FX, Y_ROW1, FW, BAND_H1, PASTEL["foundation"], PASTEL["foundation_border"], rx=12))
parts.append(section_label(FX + FW/2, Y_ROW1 + 18, "v0.1.0 — Foundation"))

# Foundation cards (row inside band)
card_h = 115
card_y = Y_ROW1 + 36
gap = 8

cards_f = [
    ("core", ["core"], ["validation", "zero/default", "pointers, strings"]),
    ("collections", ["collections"], ["chunk, group", "distinct", "error-aware"]),
    ("codec", ["codec"], ["binary", "encoding"]),
    ("compression", ["compression"], ["gzip/snappy", "/zstd"]),
    ("concurrency", ["concurrency"], ["goroutine", "helpers"]),
    ("serialization", ["serialization"], ["json/msgpack", "/gob"]),
]

cw = 105  # wider cards

total_f_w = len(cards_f) * cw + (len(cards_f) - 1) * gap
start_fx = FX + (FW - total_f_w) / 2
f_card_centers = {}
for i, (key, title, detail) in enumerate(cards_f):
    cx_card = start_fx + i * (cw + gap)
    parts.append(card(cx_card, card_y, cw, card_h, "white", PASTEL["foundation_border"], title, detail, rx=8))
    f_card_centers[key] = (cx_card + cw/2, card_y + card_h)

# Testing band
TX = MARGIN + FW + PAD
TW = 360
parts.append(rect(TX, Y_ROW1, TW, BAND_H1, PASTEL["testing"], PASTEL["testing_border"], rx=12))
parts.append(section_label(TX + TW/2, Y_ROW1 + 18, "Testing Infrastructure"))

tc_names = ["redis", "postgres", "mysql", "nats", "kafka"]
tc_cw = 64; tc_ch = 50; tc_gap = 6
tc_total = len(tc_names) * tc_cw + (len(tc_names)-1) * tc_gap
tc_startx = TX + (TW - tc_total) / 2
for i, name in enumerate(tc_names):
    parts.append(card(tc_startx + i*(tc_cw+tc_gap), Y_ROW1+36, tc_cw, tc_ch, "white", PASTEL["testing_border"], [name], rx=6))

# testing card
parts.append(card(TX + (TW - 180)/2, Y_ROW1+100, 180, 70, "white", PASTEL["testing_border"],
                  ["testing"], ["Eventually/Consistently", "Gomega helpers"], rx=8))

# Leader band
LX = TX + TW + PAD
LW = W - LX - MARGIN
parts.append(rect(LX, Y_ROW1, LW, BAND_H1, PASTEL["leader"], PASTEL["leader_border"], rx=12))
parts.append(section_label(LX + LW/2, Y_ROW1 + 18, "Leader Election"))

l_cw = min(LW - 2*PAD, 180)
l_cx = LX + (LW - l_cw) / 2
leader_cy = Y_ROW1 + 42
parts.append(card(l_cx, leader_cy, l_cw, 65, "white", PASTEL["leader_border"],
                  ["leader"], ["API interface", "campaign / resign"], rx=8))
lr_cy = Y_ROW1 + 120
parts.append(card(l_cx, lr_cy, l_cw, 65, "white", PASTEL["leader_border"],
                  ["leader/redis"], ["SET NX PX", "+ TTL renewal"], rx=8))

# connector leader -> leader/redis
parts.append(arrow(l_cx + l_cw/2, leader_cy + 65, l_cx + l_cw/2, lr_cy, PASTEL["leader_border"]))

# ── ROW 2: Resilience ─────────────────────────────────────────────────────────
R2X = MARGIN; R2W = W - 2*MARGIN
parts.append(rect(R2X, Y_ROW2, R2W, BAND_H2, PASTEL["resilience"], PASTEL["resilience_border"], rx=12))
parts.append(section_label(R2X + R2W/2, Y_ROW2 + 16, "v0.2.0 — Resilience"))
res_cw = 320; res_ch = 60
res_cx = R2X + (R2W - res_cw) / 2
res_cy = Y_ROW2 + 30
parts.append(card(res_cx, res_cy, res_cw, res_ch, "white", PASTEL["resilience_border"],
                  ["resilience"], ["retry · timeout · circuit breaker · bulkhead · HTTP middleware"], rx=8))
res_center = (res_cx + res_cw/2, res_cy + res_ch)

# ── ROW 3: Cache ─────────────────────────────────────────────────────────────
R3X = MARGIN; R3W = W - 2*MARGIN
parts.append(rect(R3X, Y_ROW3, R3W, BAND_H3, PASTEL["cache"], PASTEL["cache_border"], rx=12))
parts.append(section_label(R3X + R3W/2, Y_ROW3 + 16, "v0.3.0 — Cache & Coordination"))
cac_cw = 340; cac_ch = 60
cac_cx = R3X + (R3W - cac_cw) / 2
cac_cy = Y_ROW3 + 30
parts.append(card(cac_cx, cac_cy, cac_cw, cac_ch, "white", PASTEL["cache_border"],
                  ["cache"], ["near cache · Redis locks · token-bucket rate limiting"], rx=8))
cac_center = (cac_cx + cac_cw/2, cac_cy + cac_ch)

# ── ROW 4: Workflow + Batch ───────────────────────────────────────────────────
R4X = MARGIN; R4W = W - 2*MARGIN
parts.append(rect(R4X, Y_ROW4, R4W, BAND_H4, PASTEL["workflow"], PASTEL["workflow_border"], rx=12))
parts.append(section_label(R4X + R4W/2, Y_ROW4 + 16, "v0.4-0.5 — Workflow & Batch Recovery"))

wf_cw = 240; wf_ch = 70
wf_cx = R4X + R4W/2 - wf_cw - 20
wf_cy = Y_ROW4 + 30
parts.append(card(wf_cx, wf_cy, wf_cw, wf_ch, "white", PASTEL["workflow_border"],
                  ["workflow"], ["state machine", "lightweight workflows"], rx=8))
wf_center = (wf_cx + wf_cw/2, wf_cy + wf_ch)

bt_cw = 240; bt_ch = 70
bt_cx = R4X + R4W/2 + 20
bt_cy = wf_cy
parts.append(card(bt_cx, bt_cy, bt_cw, bt_ch, "white", PASTEL["batch_border"],
                  ["batch"], ["retry / skip policies", "checkpoint restart"], rx=8))
bt_center = (bt_cx + bt_cw/2, bt_cy + bt_ch)

# ── ROW 5: Portable ───────────────────────────────────────────────────────────
R5X = MARGIN; R5W = W - 2*MARGIN
parts.append(rect(R5X, Y_ROW5, R5W, BAND_H5, PASTEL["portable"], PASTEL["portable_border"], rx=12))
parts.append(section_label(R5X + R5W/2, Y_ROW5 + 16, "v0.6 — Portable Utilities"))
port_cw = 400; port_ch = 55
port_cx = R5X + (R5W - port_cw) / 2
port_cy = Y_ROW5 + 26
parts.append(card(port_cx, port_cy, port_cw, port_ch, "white", PASTEL["portable_border"],
                  ["id · jwt · money · probabilistic · rule engine"], rx=8))
port_center = (port_cx + port_cw/2, port_cy + port_ch)

# ── ROW 6: Domain ─────────────────────────────────────────────────────────────
R6X = MARGIN; R6W = W - 2*MARGIN
parts.append(rect(R6X, Y_ROW6, R6W, BAND_H6, PASTEL["domain"], PASTEL["domain_border"], rx=12))
parts.append(section_label(R6X + R6W/2, Y_ROW6 + 17, "v0.8-0.11 — Domain Packages"))

domain_cards = [
    ("graph", ["graph"], ["Neo4j / Memgraph"]),
    ("text", ["text"], ["search · blockword", "tokenizer"]),
    ("audit", ["audit"], ["event packages"]),
    ("aws", ["aws"], ["helpers", "LocalStack examples"]),
]
d_cw = 240; d_ch = 68; d_gap = 16
d_total = len(domain_cards)*d_cw + (len(domain_cards)-1)*d_gap
d_startx = R6X + (R6W - d_total) / 2
d_cy = Y_ROW6 + 28
domain_centers = []
for i, (key, title, detail) in enumerate(domain_cards):
    dx = d_startx + i*(d_cw+d_gap)
    parts.append(card(dx, d_cy, d_cw, d_ch, "white", PASTEL["domain_border"], title, detail, rx=8))
    domain_centers.append((dx + d_cw/2, d_cy))

# ── Connectors ────────────────────────────────────────────────────────────────
def vert_arrow(x, y1, y2, color, dashed=False):
    dash = ' stroke-dasharray="6,4"' if dashed else ""
    marker = "url(#arrowhead-dashed)" if dashed else "url(#arrowhead)"
    return f'<line x1="{x}" y1="{y1}" x2="{x}" y2="{y2}" stroke="{color}" stroke-width="1.8"{dash} marker-end="{marker}"/>'

# Foundation -> Resilience (core center)
core_cx, core_bot = f_card_centers["core"]
parts.append(vert_arrow(core_cx, core_bot, res_cy, PASTEL["foundation_border"]))

# concurrency -> resilience
conc_cx, conc_bot = f_card_centers["concurrency"]
parts.append(ortho_arrow(conc_cx, conc_bot, res_cx + 60, res_cy, PASTEL["foundation_border"]))

# collections -> batch
coll_cx, coll_bot = f_card_centers["collections"]
parts.append(ortho_arrow(coll_cx, coll_bot, bt_cx + bt_cw/2, bt_cy, PASTEL["foundation_border"], dashed=True))

# serialization -> cache
ser_cx, ser_bot = f_card_centers["serialization"]
parts.append(ortho_arrow(ser_cx, ser_bot, cac_cx + cac_cw - 60, cac_cy, PASTEL["foundation_border"]))

# leader/redis -> cache (dashed)
lr_cx = l_cx + l_cw/2
lr_bot = lr_cy + 65
parts.append(ortho_arrow(lr_cx, lr_bot, cac_cx + cac_cw + 30, cac_cy + cac_ch/2, PASTEL["leader_border"], dashed=True))

# leader/redis -> batch (dashed)
parts.append(ortho_arrow(lr_cx, lr_bot, bt_cx + bt_cw - 40, bt_cy, PASTEL["leader_border"], dashed=True))

# Resilience -> Cache
parts.append(vert_arrow(res_center[0], res_center[1], cac_cy, PASTEL["resilience_border"]))

# Resilience -> Workflow
parts.append(ortho_arrow(res_cx + 60, res_center[1], wf_cx + wf_cw/2, wf_cy, PASTEL["resilience_border"]))

# Cache -> Workflow
parts.append(ortho_arrow(cac_center[0], cac_center[1], wf_cx + wf_cw/2, wf_cy, PASTEL["cache_border"]))

# Workflow -> Batch
mid_wf_bt_y = wf_cy + wf_ch/2
parts.append(arrow(wf_cx + wf_cw, mid_wf_bt_y, bt_cx, mid_wf_bt_y, PASTEL["workflow_border"]))

# Workflow -> Portable
parts.append(ortho_arrow(wf_center[0], wf_center[1], port_cx + 80, port_cy, PASTEL["workflow_border"], dashed=True))

# Batch -> Portable
parts.append(ortho_arrow(bt_center[0], bt_center[1], port_cx + port_cw - 80, port_cy, PASTEL["batch_border"], dashed=True))

# Portable -> Domain
port_bot = port_cy + port_ch
for i, (dcx, dcy) in enumerate(domain_centers):
    parts.append(ortho_arrow(port_cx + (i+1)*port_cw/(len(domain_centers)+1), port_bot, dcx, dcy, PASTEL["portable_border"], dashed=True))

# ── Legend ────────────────────────────────────────────────────────────────────
leg_x = W - 220; leg_y = H - 110
parts.append(rect(leg_x, leg_y, 200, 90, "white", PASTEL["frame"], rx=8))
parts.append(f'<text x="{leg_x+100}" y="{leg_y+16}" font-family="{ARCH_FONT}" font-size="11" fill="{PASTEL["text"]}" text-anchor="middle" dominant-baseline="middle">Legend</text>')
# solid arrow
parts.append(f'<line x1="{leg_x+16}" y1="{leg_y+36}" x2="{leg_x+60}" y2="{leg_y+36}" stroke="{PASTEL["arrow"]}" stroke-width="1.8" marker-end="url(#arrowhead)"/>')
parts.append(f'<text x="{leg_x+68}" y="{leg_y+36}" font-family="{DETAIL_FONT}" font-size="10" fill="{PASTEL["subtext"]}" dominant-baseline="middle">depends on</text>')
# dashed arrow
parts.append(f'<line x1="{leg_x+16}" y1="{leg_y+60}" x2="{leg_x+60}" y2="{leg_y+60}" stroke="{PASTEL["dashed"]}" stroke-width="1.8" stroke-dasharray="6,4" marker-end="url(#arrowhead-dashed)"/>')
parts.append(f'<text x="{leg_x+68}" y="{leg_y+60}" font-family="{DETAIL_FONT}" font-size="10" fill="{PASTEL["subtext"]}" dominant-baseline="middle">used by / planned</text>')
# active badge
parts.append(rect(leg_x+16, leg_y+76, 44, 16, PASTEL["foundation"], PASTEL["foundation_border"], rx=4))
parts.append(f'<text x="{leg_x+38}" y="{leg_y+84}" font-family="{DETAIL_FONT}" font-size="9" fill="{PASTEL["text"]}" text-anchor="middle" dominant-baseline="middle">active</text>')
parts.append(rect(leg_x+70, leg_y+76, 44, 16, PASTEL["resilience"], PASTEL["resilience_border"], rx=4))
parts.append(f'<text x="{leg_x+92}" y="{leg_y+84}" font-family="{DETAIL_FONT}" font-size="9" fill="{PASTEL["text"]}" text-anchor="middle" dominant-baseline="middle">roadmap</text>')

parts.append('</svg>')

svg_out = "\n".join(parts)
out_path = Path(__file__).resolve().parent / "bluetape-go-architecture-overview.svg"
out_path.write_text(svg_out)
print(f"Written: {out_path}")
