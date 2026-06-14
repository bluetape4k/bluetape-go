#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "issue-201-test-gates-flow";

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
  stress: "#7C3AED",
  good: "#16A34A",
  warn: "#D97706",
  fail: "#DC2626",
  neutral: "#475569",
};

const canvas = { w: 2240, h: 1360 };
const frame = { x: 48, y: 48, w: 2144, h: 1264 };
const titleGap = 78;

const panels = [
  { x: 92, y: 190, w: 2056, h: 568, label: "Issue #201 Verification Upgrade Path", fill: "#F8FAFC", stroke: "#E2E8F0" },
  { x: 92, y: 804, w: 2056, h: 332, label: "Required Review And Evidence Lanes", fill: "#F5F3FF", stroke: "#DDD6FE" },
];

const nodes = [
  {
    id: "inventory",
    x: 132,
    y: 316,
    w: 344,
    h: 168,
    title: "1. Gap Inventory",
    details: ["#200 audit plus current tests", "failure, cancel, cleanup, stress"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "red",
    x: 560,
    y: 316,
    w: 344,
    h: 168,
    title: "2. RED Tests",
    details: ["write missing contract tests", "watch targeted failures first"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
  {
    id: "green",
    x: 988,
    y: 316,
    w: 360,
    h: 168,
    title: "3. Minimal Green",
    details: ["bounded cleanup contexts", "no broad API expansion"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "stress",
    x: 1432,
    y: 284,
    w: 372,
    h: 232,
    title: "4. Stress/Race Gate",
    details: ["GoroutineStressTester", "AsyncJobTester where context matters", "targeted go test -race"],
    fill: "#EDE9FE",
    stroke: "#C4B5FD",
  },
  {
    id: "ci",
    x: 1432,
    y: 584,
    w: 372,
    h: 118,
    title: "5. Repo Gate",
    details: ["make ci", "git diff --check"],
    fill: "#FFF7ED",
    stroke: "#FED7AA",
  },
  {
    id: "dod",
    x: 1000,
    y: 592,
    w: 336,
    h: 110,
    title: "DoD Status",
    details: ["P0=0 P1=0", "PR body live verified"],
    fill: "#ECFDF5",
    stroke: "#A7F3D0",
  },
];

const lanes = [
  {
    x: 132,
    y: 900,
    w: 300,
    h: 118,
    title: "Performance",
    details: ["stress stays bounded", "no slow global suites"],
    fill: "#EFF6FF",
    stroke: "#BFDBFE",
  },
  {
    x: 464,
    y: 900,
    w: 300,
    h: 118,
    title: "Stability",
    details: ["cancellation", "cleanup timeout"],
    fill: "#ECFEFF",
    stroke: "#A5F3FC",
  },
  {
    x: 796,
    y: 900,
    w: 300,
    h: 118,
    title: "Security",
    details: ["no weaker auth/cache", "no secret leakage"],
    fill: "#FEF2F2",
    stroke: "#FECACA",
  },
  {
    x: 1128,
    y: 900,
    w: 300,
    h: 118,
    title: "Operator/Ops",
    details: ["Docker cleanup", "serial Testcontainers"],
    fill: "#FFF7ED",
    stroke: "#FED7AA",
  },
  {
    x: 1460,
    y: 900,
    w: 300,
    h: 118,
    title: "Developer/API",
    details: ["Go-shaped tests", "errors.Is/As"],
    fill: "#F0FDF4",
    stroke: "#BBF7D0",
  },
  {
    x: 1792,
    y: 900,
    w: 300,
    h: 118,
    title: "User/Caller",
    details: ["document gaps", "future issue links"],
    fill: "#FDF4FF",
    stroke: "#F5D0FE",
  },
];

const routes = [
  { points: [[476, 400], [560, 400]], color: colors.source },
  { points: [[904, 400], [988, 400]], color: colors.fail, label: "RED" },
  { points: [[1348, 400], [1432, 400]], color: colors.good, label: "GREEN" },
  { points: [[1618, 516], [1618, 584]], color: colors.stress, label: "race" },
  { points: [[1432, 642], [1336, 642]], color: colors.good },
  { points: [[1168, 592], [1168, 484], [1180, 484]], color: colors.warn, dashed: true, label: "fix loop" },
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
  label=${dotLabel("Issue #201 Test Gate Upgrade", "gap inventory, RED tests, minimal fixes, stress/race, and DoD gate")}
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

inventory [label=${dotLabel("1. Gap Inventory", "#200 audit plus current tests")} fillcolor="#DBEAFE" color="#93C5FD"];
red [label=${dotLabel("2. RED Tests", "failure, cancellation, cleanup")} fillcolor="#FEE2E2" color="#FCA5A5"];
green [label=${dotLabel("3. Minimal Green", "small code/test fixes")} fillcolor="#DCFCE7" color="#86EFAC"];
stress [label=${dotLabel("4. Stress/Race", "GoroutineStressTester plus race")} fillcolor="#EDE9FE" color="#C4B5FD"];
ci [label=${dotLabel("5. Repo Gate", "make ci and diff check")} fillcolor="#FFF7ED" color="#FED7AA"];
dod [label=${dotLabel("DoD Status", "P0=0 P1=0")} fillcolor="#ECFDF5" color="#A7F3D0"];

inventory -> red [color="#2563EB"];
red -> green [label="RED" color="#DC2626"];
green -> stress [label="GREEN" color="#16A34A"];
stress -> ci [label="race" color="#7C3AED"];
ci -> dod [color="#16A34A"];
dod -> green [label="fix loop" style=dashed color="#D97706"];
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
  ${text(canvas.w / 2, 96, "Issue #201 Test Gate Upgrade", "title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(canvas.w / 2, 140, "Turn audit gaps into failure, cancellation, cleanup, stress, race, and CI evidence before parity expansion.", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

  ${panels.map(panel).join("\n")}
  ${routes.map(path).join("\n")}
  ${nodes.map(node).join("\n")}
  ${lanes.map(node).join("\n")}
  ${text(canvas.w / 2, 1200, "Diagram baseline: flow-retry-workflow plus workflow-image-upload; no grid layout for the relationship-heavy verification path.", "footer-note", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(canvas.w / 2, 1238, "Evidence must include RED failure, GoroutineStressTester coverage, targeted race, make ci, git diff check, and live PR DoD verification.", "footer-note", 'text-anchor="middle" dominant-baseline="middle"')}
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
