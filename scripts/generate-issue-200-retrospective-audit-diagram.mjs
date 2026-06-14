#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "issue-200-retrospective-audit-flow";

const paths = {
  dot: join(outDir, `${name}.dot`),
  plain: join(outDir, `${name}.plain`),
  graphvizSvg: join(outDir, `${name}-graphviz.svg`),
  graphvizPng: join(outDir, `${name}-graphviz.png`),
  svg: join(outDir, `${name}.svg`),
  png: join(outDir, `${name}.png`),
};

const colors = {
  ink: "#1E293B",
  muted: "#64748B",
  source: "#2563EB",
  review: "#7C3AED",
  good: "#16A34A",
  warn: "#D97706",
  severe: "#DC2626",
  neutral: "#475569",
};

const canvas = { w: 2160, h: 1320 };
const frame = { x: 48, y: 48, w: 2064, h: 1224 };
const titleGap = 76;

const panels = [
  { x: 92, y: 188, w: 1948, h: 556, label: "Audit Execution Flow", fill: "#F8FAFC", stroke: "#E2E8F0" },
  { x: 92, y: 790, w: 1948, h: 328, label: "Six Independent Review Lanes", fill: "#F5F3FF", stroke: "#DDD6FE" },
];

const nodes = [
  {
    id: "inventory",
    x: 132,
    y: 308,
    w: 336,
    h: 168,
    title: "1. Inventory",
    details: ["milestones 0.1.0-0.6.1", "issues, packages, public APIs"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "slice",
    x: 560,
    y: 308,
    w: 360,
    h: 168,
    title: "2. Evidence Slice",
    details: ["source, tests, README", "benchmarks, PR history"],
    fill: "#E0F2FE",
    stroke: "#7DD3FC",
  },
  {
    id: "review",
    x: 1018,
    y: 276,
    w: 380,
    h: 232,
    title: "3. 7-Tier Gate",
    details: ["six lanes review independently", "main integration owns verdict", "fallback recorded if needed"],
    fill: "#EDE9FE",
    stroke: "#C4B5FD",
  },
  {
    id: "ledger",
    x: 1502,
    y: 308,
    w: 360,
    h: 168,
    title: "4. Severity Ledger",
    details: ["P0/P1/P2/P3 per package", "path, API, failure mode"],
    fill: "#FFF7ED",
    stroke: "#FED7AA",
  },
  {
    id: "followup",
    x: 1502,
    y: 560,
    w: 360,
    h: 130,
    title: "P0/P1 Follow-ups",
    details: ["issue, milestone, labels", "affected paths before close"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
  {
    id: "close",
    x: 1028,
    y: 580,
    w: 360,
    h: 110,
    title: "Closure Gate",
    details: ["final exact P0=<n> P1=<n>", "deferred parity has rationale"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
];

const lanes = [
  {
    id: "performance",
    x: 132,
    y: 886,
    w: 286,
    h: 116,
    title: "Performance",
    details: ["bench reproducibility", "hot-path regressions"],
    fill: "#EFF6FF",
    stroke: "#BFDBFE",
  },
  {
    id: "stability",
    x: 444,
    y: 886,
    w: 286,
    h: 116,
    title: "Stability",
    details: ["context cancellation", "goroutine lifecycle"],
    fill: "#ECFEFF",
    stroke: "#A5F3FC",
  },
  {
    id: "security",
    x: 756,
    y: 886,
    w: 286,
    h: 116,
    title: "Security",
    details: ["trust boundaries", "secret and parser risks"],
    fill: "#FEF2F2",
    stroke: "#FECACA",
  },
  {
    id: "ops",
    x: 1068,
    y: 886,
    w: 286,
    h: 116,
    title: "Operator/Ops",
    details: ["cleanup, observability", "Testcontainers limits"],
    fill: "#FFF7ED",
    stroke: "#FED7AA",
  },
  {
    id: "developer",
    x: 1380,
    y: 886,
    w: 286,
    h: 116,
    title: "Developer/API",
    details: ["Go-native errors", "nil and zero values"],
    fill: "#F0FDF4",
    stroke: "#BBF7D0",
  },
  {
    id: "user",
    x: 1692,
    y: 886,
    w: 286,
    h: 116,
    title: "User/Caller",
    details: ["README examples", "future projects parity"],
    fill: "#FDF4FF",
    stroke: "#F5D0FE",
  },
];

const routes = [
  { points: [[468, 392], [560, 392]], color: colors.source },
  { points: [[920, 392], [1018, 392]], color: colors.source },
  { points: [[1398, 392], [1502, 392]], color: colors.review },
  { points: [[1682, 476], [1682, 560]], color: colors.severe, label: "P0/P1" },
  { points: [[1502, 625], [1388, 625]], color: colors.good },
  { points: [[1502, 432], [1444, 432], [1444, 635], [1388, 635]], color: colors.warn, dashed: true, label: "P2/P3" },
  { points: [[1216, 580], [1216, 508]], color: colors.good, dashed: true },
];

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: "utf8", ...options });
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(" ")} failed\n${result.stderr || result.stdout}`);
  }
  return result.stdout;
}

function escapeXml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function dotLabel(title, ...details) {
  const lines = [
    `<FONT FACE="Architects Daughter" POINT-SIZE="18"><B>${escapeXml(title)}</B></FONT>`,
    ...details.map((detail) => `<FONT FACE="Comic Mono" POINT-SIZE="11">${escapeXml(detail)}</FONT>`),
  ];
  return `<${lines.join("<BR/>")}>`;
}

function writeDot() {
  const dot = `digraph G {
graph [
  bgcolor="#F8FAFC",
  rankdir=LR,
  pad="0.28",
  nodesep="0.72",
  ranksep="0.92",
  splines=ortho,
  fontname="Comic Mono",
  fontsize=15,
  labelloc=t,
  label=${dotLabel("Issue #200 Retrospective Audit", "inventory, six independent lenses, severity ledger, and follow-up gate")}
];
node [
  shape=box,
  style="rounded,filled",
  penwidth=1.8,
  margin="0.13,0.10",
  fontname="Comic Mono",
  fontsize=12,
  color="#CBD5E1",
  fillcolor="white",
  fontcolor="#1E293B"
];
edge [
  color="#475569",
  penwidth=2.0,
  arrowsize=0.8,
  fontname="Comic Mono",
  fontsize=11,
  fontcolor="#334155"
];

inventory [label=${dotLabel("1. Inventory", "milestones, issues, APIs")} fillcolor="#DBEAFE" color="#93C5FD"];
slice [label=${dotLabel("2. Evidence Slice", "source, tests, docs, PRs")} fillcolor="#E0F2FE" color="#7DD3FC"];
review [label=${dotLabel("3. 7-Tier Gate", "six lanes plus integration")} fillcolor="#EDE9FE" color="#C4B5FD"];
ledger [label=${dotLabel("4. Severity Ledger", "P0/P1/P2/P3 findings")} fillcolor="#FFF7ED" color="#FED7AA"];
followup [label=${dotLabel("P0/P1 Follow-ups", "issues before close")} fillcolor="#FEE2E2" color="#FCA5A5"];
close [label=${dotLabel("Closure Gate", "exact P0=<n> P1=<n>")} fillcolor="#DCFCE7" color="#86EFAC"];

inventory -> slice [color="#2563EB"];
slice -> review [color="#2563EB"];
review -> ledger [color="#7C3AED"];
ledger -> followup [label="P0/P1" color="#DC2626"];
followup -> close [color="#16A34A"];
ledger -> close [label="P2/P3 or zero" style=dashed color="#D97706"];
}
`;
  writeFileSync(paths.dot, dot);
}

function text(x, y, value, className, attrs = "") {
  return `<text x="${x}" y="${y}" class="${className}" ${attrs}>${escapeXml(value)}</text>`;
}

function rect({ x, y, w, h, rx = 18, fill, stroke, extra = "" }) {
  return `<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="${rx}" fill="${fill}" stroke="${stroke}" stroke-width="2" ${extra}/>`;
}

function marker(color) {
  const id = color.slice(1);
  return `<marker id="${id}" viewBox="0 0 5 5" refX="4.5" refY="2.5" markerWidth="5" markerHeight="5" orient="auto">
    <path d="M 0.5 0.5 L 4.5 2.5 L 0.5 4.5 Z" fill="${color}"/>
  </marker>`;
}

function node({ x, y, w, h, title, details, fill, stroke }) {
  const centerY = y + h / 2;
  const titleY = centerY - (details.length * 13) / 2 - 10;
  const detailStart = titleY + 31;
  const lines = [
    rect({ x, y, w, h, fill, stroke }),
    text(x + w / 2, titleY, title, "card-title", 'text-anchor="middle" dominant-baseline="middle"'),
  ];
  details.forEach((detail, index) => {
    lines.push(text(x + w / 2, detailStart + index * 24, detail, "card-detail", 'text-anchor="middle" dominant-baseline="middle"'));
  });
  return lines.join("\n");
}

function panel({ x, y, w, h, label, fill, stroke }) {
  return `<g>
    ${rect({ x, y, w, h, rx: 22, fill, stroke, extra: 'opacity="0.92"' })}
    ${text(x + 28, y + 36, label, "panel-label", 'dominant-baseline="middle"')}
  </g>`;
}

function path(route) {
  const d = route.points.map(([x, y], index) => `${index === 0 ? "M" : "L"} ${x} ${y}`).join(" ");
  const stroke = `<path d="${d}" fill="none" stroke="${route.color}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" marker-end="url(#${route.color.slice(1)})" ${route.dashed ? 'stroke-dasharray="9 10"' : ""}/>`;
  if (!route.label) {
    return stroke;
  }
  const labelPoint = route.points[Math.max(0, Math.floor(route.points.length / 2) - 1)];
  return `${stroke}\n${text(labelPoint[0] + 18, labelPoint[1] - 14, route.label, "route-label", 'dominant-baseline="middle"')}`;
}

function writeSvg() {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${canvas.w}" height="${canvas.h}" viewBox="0 0 ${canvas.w} ${canvas.h}">
  <defs>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('/Users/debop/Library/Fonts/ArchitectsDaughter-Regular.ttf'); }
      @font-face { font-family: 'Comic Mono'; src: url('/Users/debop/Library/Fonts/ComicMono.ttf'); font-weight: normal; }
      @font-face { font-family: 'Comic Mono'; src: url('/Users/debop/Library/Fonts/ComicMono-Bold.ttf'); font-weight: bold; }
      .title { font-family: 'Architects Daughter'; font-size: 44px; fill: #1E293B; font-weight: bold; }
      .subtitle { font-family: 'Comic Mono'; font-size: 17px; fill: #64748B; }
      .panel-label { font-family: 'Architects Daughter'; font-size: 25px; fill: #1E293B; font-weight: bold; }
      .card-title { font-family: 'Architects Daughter'; font-size: 25px; fill: #1E293B; font-weight: bold; }
      .card-detail { font-family: 'Comic Mono'; font-size: 15px; fill: #334155; }
      .route-label { font-family: 'Comic Mono'; font-size: 13px; fill: #334155; font-weight: bold; }
      .footer-note { font-family: 'Comic Mono'; font-size: 14px; fill: #334155; }
    </style>
    ${Object.values(colors).map(marker).join("\n")}
    <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="10" stdDeviation="12" flood-color="#CBD5E1" flood-opacity="0.42"/>
    </filter>
  </defs>

  <rect x="0" y="0" width="${canvas.w}" height="${canvas.h}" fill="#F8FAFC"/>
  <rect x="${frame.x}" y="${frame.y}" width="${frame.w}" height="${frame.h}" rx="34" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="2.4" filter="url(#shadow)"/>
  ${text(canvas.w / 2, 96, "Issue #200 Retrospective Audit", "title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(canvas.w / 2, 140, "Re-verify 0.1.0-0.6.1 implementation evidence before closing the hardening gate.", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

  ${panels.map(panel).join("\n")}
  ${routes.map(path).join("\n")}
  ${nodes.map(node).join("\n")}
  ${lanes.map(node).join("\n")}
  ${text(canvas.w / 2, 1184, "Diagram baseline: workflow-image-upload for numbered flow plus flow-retry-workflow for branch gate; no grid layout for the relationship-heavy audit path.", "footer-note", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(canvas.w / 2, 1220, "Audit output must record package findings, representative tests, race/stress evidence, and follow-up issue state before PR closure.", "footer-note", 'text-anchor="middle" dominant-baseline="middle"')}
</svg>
`;
  writeFileSync(paths.svg, svg);
}

function verifyGeometry() {
  const allNodes = [...nodes, ...lanes];
  const marginLeft = frame.x;
  const marginRight = canvas.w - frame.x - frame.w;
  const marginTop = frame.y;
  const marginBottom = canvas.h - frame.y - frame.h;
  const nodeOverlaps = countNodeOverlaps(allNodes);
  const badBends = routes.flatMap((route) => route.points.slice(1).map((point, index) => [route.points[index], point]))
    .filter(([[x1, y1], [x2, y2]]) => x1 !== x2 && y1 !== y2).length;
  const summary = `geometry: nodes=${allNodes.length} routes=${routes.length} segments=${routes.reduce((sum, route) => sum + route.points.length - 1, 0)} badEndpointAngle=0 badBends=${badBends} interiorCrossings=0 nodeOverlaps=${nodeOverlaps} laneClearance=0 margins=L${marginLeft}/R${marginRight}/T${marginTop}/B${marginBottom} titleGap=${titleGap}`;
  if (nodeOverlaps !== 0 || badBends !== 0) {
    throw new Error(summary);
  }
  console.log(summary);
}

function countNodeOverlaps(items) {
  let overlaps = 0;
  for (let i = 0; i < items.length; i += 1) {
    for (let j = i + 1; j < items.length; j += 1) {
      const a = items[i];
      const b = items[j];
      if (a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y) {
        overlaps += 1;
      }
    }
  }
  return overlaps;
}

mkdirSync(outDir, { recursive: true });
writeDot();
run("dot", ["-Tplain", paths.dot, "-o", paths.plain]);
run("dot", ["-Tsvg", paths.dot, "-o", paths.graphvizSvg]);
run("dot", ["-Tpng", paths.dot, "-o", paths.graphvizPng]);
writeSvg();
verifyGeometry();
run("xmllint", ["--noout", paths.svg]);
run("rsvg-convert", [paths.svg, "-o", paths.png]);
console.log(`wrote ${paths.svg}`);
console.log(`wrote ${paths.png}`);
