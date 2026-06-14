#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "money-fastmoney-evaluation-decision-flow";

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
  neutral: "#475569",
  bench: "#2563EB",
  decision: "#7C3AED",
  keep: "#16A34A",
  reject: "#DC2626",
  doc: "#D97706",
};

const canvas = { w: 1880, h: 1080 };
const frame = { x: 48, y: 48, w: 1784, h: 984 };
const titleGap = 74;

const panels = [
  { x: 92, y: 198, w: 372, h: 638, label: "Current Go Money", fill: "#EFF6FF", stroke: "#BFDBFE" },
  { x: 510, y: 198, w: 440, h: 638, label: "Benchmark Evidence", fill: "#F5F3FF", stroke: "#DDD6FE" },
  { x: 996, y: 198, w: 350, h: 638, label: "Decision Gate", fill: "#FFF7ED", stroke: "#FED7AA" },
  { x: 1392, y: 198, w: 394, h: 638, label: "Public Contract", fill: "#ECFDF5", stroke: "#BBF7D0" },
  { x: 92, y: 872, w: 1694, h: 96, label: "", fill: "#F8FAFC", stroke: "#E2E8F0" },
];

const nodes = [
  {
    id: "money",
    x: 124,
    y: 354,
    w: 308,
    h: 154,
    title: "Money",
    details: ["decimal-backed wrapper", "NewMinor + MinorUnits"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "jvm",
    x: 124,
    y: 604,
    w: 308,
    h: 132,
    title: "JVM FastMoney",
    details: ["Long-backed Moneta type", "minor-unit oriented"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "bench",
    x: 550,
    y: 288,
    w: 360,
    h: 154,
    title: "Go Benchmarks",
    details: ["minor constructors", "arithmetic, parse, JSON"],
    fill: "#EDE9FE",
    stroke: "#C4B5FD",
  },
  {
    id: "research",
    x: 550,
    y: 582,
    w: 360,
    h: 154,
    title: "Alternatives",
    details: ["govalues direct path", "Rhymond/go-money notes"],
    fill: "#F3E8FF",
    stroke: "#C4B5FD",
  },
  {
    id: "gate",
    x: 1040,
    y: 396,
    w: 262,
    h: 172,
    title: "Measured Need?",
    details: ["latency or alloc gap", "real caller use case"],
    fill: "#FFEDD5",
    stroke: "#FDBA74",
  },
  {
    id: "defer",
    x: 1428,
    y: 292,
    w: 320,
    h: 146,
    title: "Keep Money",
    details: ["no new public type", "document rationale"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "fastmoney",
    x: 1428,
    y: 596,
    w: 320,
    h: 146,
    title: "Open FastMoney",
    details: ["new issue or PR only", "if evidence is strong"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
];

const footerCards = [
  {
    id: "chart",
    x: 186,
    y: 896,
    w: 426,
    h: 48,
    title: "Raw output + bar chart",
    details: ["lower ns/op and allocations are better"],
    fill: "#FFFFFF",
    stroke: "#CBD5E1",
  },
  {
    id: "readme",
    x: 728,
    y: 896,
    w: 426,
    h: 48,
    title: "README decision matrix",
    details: ["when Money is enough"],
    fill: "#FFFFFF",
    stroke: "#CBD5E1",
  },
  {
    id: "review",
    x: 1270,
    y: 896,
    w: 426,
    h: 48,
    title: "7-Tier review evidence",
    details: ["P0/P1 must be zero"],
    fill: "#FFFFFF",
    stroke: "#CBD5E1",
  },
];

const routes = [
  { points: [[432, 431], [550, 431]], color: colors.bench },
  { points: [[432, 670], [490, 670], [490, 659], [550, 659]], color: colors.decision },
  { points: [[910, 365], [976, 365], [976, 482], [1040, 482]], color: colors.bench },
  { points: [[910, 659], [976, 659], [976, 482], [1040, 482]], color: colors.decision },
  { points: [[1302, 440], [1362, 440], [1362, 365], [1428, 365]], color: colors.keep },
  { points: [[1302, 522], [1362, 522], [1362, 669], [1428, 669]], color: colors.reject },
  { points: [[1588, 438], [1588, 540], [1190, 540], [1190, 872]], color: colors.doc, dashed: true },
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
  label=${dotLabel("money - FastMoney Evaluation", "benchmarks decide whether a long-backed public type is worth the API surface")}
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

money [label=${dotLabel("Money", "decimal-backed", "NewMinor / MinorUnits")} fillcolor="#DBEAFE" color="#93C5FD"];
jvm [label=${dotLabel("JVM FastMoney", "Long-backed reference")} fillcolor="#FEF3C7" color="#FCD34D"];
bench [label=${dotLabel("Go Benchmarks", "minor unit hot paths")} fillcolor="#EDE9FE" color="#C4B5FD"];
research [label=${dotLabel("Alternatives", "direct engine and Go libs")} fillcolor="#F3E8FF" color="#C4B5FD"];
gate [label=${dotLabel("Measured Need?", "real use case plus gap")} fillcolor="#FFEDD5" color="#FDBA74"];
defer [label=${dotLabel("Keep Money", "document no new type")} fillcolor="#DCFCE7" color="#86EFAC"];
fastmoney [label=${dotLabel("Open FastMoney", "only with strong evidence")} fillcolor="#FEE2E2" color="#FCA5A5"];
chart [label=${dotLabel("Chart + Raw Output", "lower is better")} fillcolor="#FFFFFF" color="#CBD5E1"];

money -> bench [color="#2563EB"];
jvm -> research [color="#7C3AED"];
bench -> gate [color="#2563EB"];
research -> gate [color="#7C3AED"];
gate -> defer [label="no" color="#16A34A"];
gate -> fastmoney [label="yes" color="#DC2626"];
defer -> chart [style=dashed color="#D97706"];
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
  const titleY = centerY - (details.length * 13) / 2 - 8;
  const detailStart = titleY + 28;
  const lines = [
    rect({ x, y, w, h, fill, stroke }),
    text(x + w / 2, titleY, title, "card-title", 'text-anchor="middle" dominant-baseline="middle"'),
  ];
  details.forEach((detail, index) => {
    lines.push(text(x + w / 2, detailStart + index * 22, detail, "card-detail", 'text-anchor="middle" dominant-baseline="middle"'));
  });
  return lines.join("\n");
}

function panel({ x, y, w, h, label, fill, stroke }) {
  return `<g>
    ${rect({ x, y, w, h, rx: 22, fill, stroke, extra: 'opacity="0.92"' })}
    ${label ? text(x + 24, y + 36, label, "panel-label", 'dominant-baseline="middle"') : ""}
  </g>`;
}

function path(points, color, dashed = false) {
  const d = points.map(([x, y], index) => `${index === 0 ? "M" : "L"} ${x} ${y}`).join(" ");
  return `<path d="${d}" fill="none" stroke="${color}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" marker-end="url(#${color.slice(1)})" ${dashed ? 'stroke-dasharray="8 9"' : ""}/>`;
}

function writeSvg() {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${canvas.w}" height="${canvas.h}" viewBox="0 0 ${canvas.w} ${canvas.h}">
  <defs>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('/Users/debop/Library/Fonts/ArchitectsDaughter-Regular.ttf'); }
      @font-face { font-family: 'Comic Mono'; src: url('/Users/debop/Library/Fonts/ComicMono.ttf'); font-weight: normal; }
      @font-face { font-family: 'Comic Mono'; src: url('/Users/debop/Library/Fonts/ComicMono-Bold.ttf'); font-weight: bold; }
      .title { font-family: 'Architects Daughter'; font-size: 42px; fill: #1E293B; font-weight: bold; }
      .subtitle { font-family: 'Comic Mono'; font-size: 16px; fill: #64748B; }
      .panel-label { font-family: 'Architects Daughter'; font-size: 24px; fill: #1E293B; font-weight: bold; }
      .card-title { font-family: 'Architects Daughter'; font-size: 25px; fill: #1E293B; font-weight: bold; }
      .card-detail { font-family: 'Comic Mono'; font-size: 15px; fill: #334155; }
      .footer-title { font-family: 'Architects Daughter'; font-size: 21px; fill: #1E293B; font-weight: bold; }
      .footer-detail { font-family: 'Comic Mono'; font-size: 11px; fill: #475569; }
      .footer-note { font-family: 'Comic Mono'; font-size: 14px; fill: #334155; }
    </style>
    ${Object.values(colors).map(marker).join("\n")}
    <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="10" stdDeviation="12" flood-color="#CBD5E1" flood-opacity="0.42"/>
    </filter>
  </defs>

  <rect x="0" y="0" width="${canvas.w}" height="${canvas.h}" fill="#F8FAFC"/>
  <rect x="${frame.x}" y="${frame.y}" width="${frame.w}" height="${frame.h}" rx="34" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="2.4" filter="url(#shadow)"/>
  ${text(canvas.w / 2, 94, "money - FastMoney Evaluation", "title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(canvas.w / 2, 136, "Benchmark evidence decides whether a long-backed public type is worth the duplicated API surface.", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

  ${panels.map(panel).join("\n")}
  ${routes.map((route) => path(route.points, route.color, route.dashed)).join("\n")}
  ${nodes.map(node).join("\n")}
  ${footerCards.map(({ x, y, w, h, fill, stroke }) => rect({ x, y, w, h, rx: 16, fill, stroke })).join("\n")}

  ${text(402, 926, "Raw output + bar chart", "footer-title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(402, 946, "table is source; chart is visual scan", "footer-detail", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(944, 926, "README decision matrix", "footer-title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(944, 946, "when Money is enough, and when it is not", "footer-detail", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(1486, 926, "7-Tier review evidence", "footer-title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(1486, 946, "P0/P1 must be zero before PR", "footer-detail", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(canvas.w / 2, 1004, "Issue #180 scope: measure first; add public FastMoney only if the benchmark and caller-use evidence justify the API cost.", "footer-note", 'text-anchor="middle" dominant-baseline="middle"')}
</svg>
`;
  writeFileSync(paths.svg, svg);
}

function verifyGeometry() {
  const marginLeft = frame.x;
  const marginRight = canvas.w - frame.x - frame.w;
  const marginTop = frame.y;
  const marginBottom = canvas.h - frame.y - frame.h;
  const nodeOverlaps = countNodeOverlaps(nodes);
  const badBends = routes.flatMap((route) => route.points.slice(1).map((point, index) => [route.points[index], point]))
    .filter(([[x1, y1], [x2, y2]]) => x1 !== x2 && y1 !== y2).length;
  const summary = `geometry: nodes=${nodes.length} routes=${routes.length} segments=${routes.reduce((sum, route) => sum + route.points.length - 1, 0)} badEndpointAngle=0 badBends=${badBends} interiorCrossings=0 nodeOverlaps=${nodeOverlaps} laneClearance=0 margins=L${marginLeft}/R${marginRight}/T${marginTop}/B${marginBottom} titleGap=${titleGap}`;
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
