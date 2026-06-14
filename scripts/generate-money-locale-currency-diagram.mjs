#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "money-locale-currency-resolution-flow";

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
  parse: "#2563EB",
  policy: "#7C3AED",
  success: "#16A34A",
  warning: "#D97706",
  error: "#DC2626",
};

const canvas = { w: 2050, h: 1120 };
const frame = { x: 48, y: 48, w: 1954, h: 1024 };
const titleGap = 58;

const panels = [
  { x: 92, y: 190, w: 330, h: 740, label: "Caller Input", fill: "#EFF6FF", stroke: "#BFDBFE" },
  { x: 462, y: 190, w: 700, h: 740, label: "BCP47 Region Policy", fill: "#F5F3FF", stroke: "#DDD6FE" },
  { x: 1202, y: 190, w: 390, h: 740, label: "CLDR Tender Query", fill: "#ECFDF5", stroke: "#A7F3D0" },
  { x: 1632, y: 190, w: 326, h: 740, label: "Caller Outcome", fill: "#FFF7ED", stroke: "#FED7AA" },
];

const nodes = [
  {
    id: "tag",
    x: 120,
    y: 350,
    w: 260,
    h: 130,
    title: "Locale Tag",
    details: ["ko-KR, en_US", "must name region"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "parse",
    x: 500,
    y: 270,
    w: 270,
    h: 150,
    title: "language.Parse",
    details: ["BCP47 syntax", "canonical tag"],
    fill: "#EDE9FE",
    stroke: "#C4B5FD",
  },
  {
    id: "region",
    x: 850,
    y: 255,
    w: 280,
    h: 170,
    title: "Explicit Region?",
    details: ["use tag region only", "no likely-region guess"],
    fill: "#F3E8FF",
    stroke: "#C4B5FD",
  },
  {
    id: "unsupported",
    x: 850,
    y: 560,
    w: 280,
    h: 132,
    title: "Unsupported",
    details: ["missing or invalid", "ErrInvalidCurrency"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
  {
    id: "query",
    x: 1240,
    y: 270,
    w: 310,
    h: 150,
    title: "currency.Query",
    details: ["Region(region)", "current legal tender"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "count",
    x: 1240,
    y: 520,
    w: 310,
    h: 170,
    title: "Tender Count",
    details: ["1 = deterministic", "0 or many = reject"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "success",
    x: 1660,
    y: 330,
    w: 270,
    h: 150,
    title: "Currency",
    details: ["ParseCurrency", "ISO 4217 wrapper"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "ambiguous",
    x: 1660,
    y: 560,
    w: 270,
    h: 132,
    title: "Ambiguous",
    details: ["multiple tender units", "caller must choose"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "notender",
    x: 1240,
    y: 790,
    w: 310,
    h: 112,
    title: "No Tender",
    details: ["no current legal tender"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
];

const routes = [
  { from: "tag", to: "parse", points: [[380, 415], [500, 415]], color: colors.parse, label: "parse" },
  { from: "parse", to: "region", points: [[770, 345], [850, 345]], color: colors.policy, label: "inspect" },
  { from: "region", to: "query", points: [[1130, 345], [1240, 345]], color: colors.success, label: "yes" },
  { from: "region", to: "unsupported", points: [[990, 425], [990, 560]], color: colors.error, label: "no" },
  { from: "query", to: "count", points: [[1395, 420], [1395, 520]], color: colors.success, label: "iterate" },
  { from: "count", to: "success", points: [[1550, 575], [1600, 575], [1600, 405], [1660, 405]], color: colors.success, label: "count = 1" },
  { from: "count", to: "ambiguous", points: [[1550, 625], [1660, 625]], color: colors.warning, label: "count > 1" },
  { from: "count", to: "notender", points: [[1395, 690], [1395, 790]], color: colors.error, label: "count = 0" },
];

const labels = [
  { x: 425, y: 384, text: "parse", color: colors.parse },
  { x: 787, y: 314, text: "extract", color: colors.policy },
  { x: 1142, y: 314, text: "region", color: colors.success },
  { x: 1006, y: 492, text: "missing", color: colors.error },
  { x: 1412, y: 470, text: "current tender", color: colors.success },
  { x: 1574, y: 498, text: "single", color: colors.success },
  { x: 1588, y: 594, text: "ambiguous", color: colors.warning },
  { x: 1412, y: 740, text: "none", color: colors.error },
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
  nodesep="0.64",
  ranksep="0.90",
  splines=ortho,
  fontname="Comic Mono",
  fontsize=15,
  labelloc=t,
  label=${dotLabel("money - Locale Currency Resolution", "explicit region plus CLDR tender data prevents hidden guesses")}
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

tag [label=${dotLabel("Locale Tag", "BCP47-like input")} fillcolor="#DBEAFE" color="#93C5FD"];
parse [label=${dotLabel("language.Parse", "canonical BCP47")} fillcolor="#EDE9FE" color="#C4B5FD"];
region [label=${dotLabel("Explicit Region?", "no likely-region guess")} fillcolor="#F3E8FF" color="#C4B5FD"];
unsupported [label=${dotLabel("Unsupported", "ErrInvalidCurrency")} fillcolor="#FEE2E2" color="#FCA5A5"];
query [label=${dotLabel("currency.Query", "current legal tender")} fillcolor="#DCFCE7" color="#86EFAC"];
count [label=${dotLabel("Tender Count", "1 / 0 / many")} fillcolor="#FEF3C7" color="#FCD34D"];
success [label=${dotLabel("Currency", "ISO 4217 wrapper")} fillcolor="#DCFCE7" color="#86EFAC"];
ambiguous [label=${dotLabel("Ambiguous", "caller must choose")} fillcolor="#FEF3C7" color="#FCD34D"];
notender [label=${dotLabel("No Tender", "reject")} fillcolor="#FEE2E2" color="#FCA5A5"];

tag -> parse [color="#2563EB"];
parse -> region [color="#7C3AED"];
region -> query [label="yes" color="#16A34A"];
region -> unsupported [label="no" color="#DC2626"];
query -> count [color="#16A34A"];
count -> success [label="1" color="#16A34A"];
count -> ambiguous [label="many" color="#D97706"];
count -> notender [label="0" color="#DC2626"];
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

function card(node) {
  const cx = node.x + node.w / 2;
  const blockHeight = 30 + node.details.length * 22;
  const startY = node.y + node.h / 2 - blockHeight / 2 + 16;
  const parts = [
    rect({ x: node.x, y: node.y, w: node.w, h: node.h, fill: node.fill, stroke: node.stroke }),
    text(cx, startY, node.title, "card-title", 'text-anchor="middle" dominant-baseline="middle"'),
  ];
  node.details.forEach((detail, index) => {
    parts.push(text(cx, startY + 30 + index * 22, detail, "detail", 'text-anchor="middle" dominant-baseline="middle"'));
  });
  return parts.join("\n");
}

function route(routeDef) {
  const id = routeDef.color.slice(1);
  const path = `M ${routeDef.points.map(([x, y]) => `${x} ${y}`).join(" L ")}`;
  return `<path d="${path}" fill="none" stroke="${routeDef.color}" stroke-width="3" marker-end="url(#${id})"/>`;
}

function labelPill({ x, y, text: value, color }) {
  const width = Math.max(68, value.length * 9 + 28);
  return [
    rect({ x, y, w: width, h: 30, rx: 10, fill: "#FFFFFF", stroke: color }),
    text(x + width / 2, y + 15, value, "route-label", `fill="${color}" text-anchor="middle" dominant-baseline="middle"`),
  ].join("\n");
}

function writeSvg() {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${canvas.w}" height="${canvas.h}" viewBox="0 0 ${canvas.w} ${canvas.h}">
<defs>
  <style>
    .title { font-family: 'Architects Daughter'; font-size: 34px; font-weight: 700; fill: ${colors.ink}; }
    .subtitle { font-family: 'Comic Mono'; font-size: 16px; fill: ${colors.muted}; }
    .panel-label { font-family: 'Architects Daughter'; font-size: 20px; font-weight: 700; fill: ${colors.ink}; }
    .card-title { font-family: 'Architects Daughter'; font-size: 22px; font-weight: 700; fill: ${colors.ink}; }
    .detail { font-family: 'Comic Mono'; font-size: 14px; fill: ${colors.ink}; }
    .route-label { font-family: 'Comic Mono'; font-size: 13px; font-weight: 700; }
    .footer { font-family: 'Comic Mono'; font-size: 14px; fill: ${colors.ink}; }
  </style>
  <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
    <feDropShadow dx="0" dy="8" stdDeviation="8" flood-color="#94A3B8" flood-opacity="0.22"/>
  </filter>
  ${[colors.parse, colors.policy, colors.success, colors.warning, colors.error].map(marker).join("\n")}
</defs>
<rect width="${canvas.w}" height="${canvas.h}" fill="#F8FAFC"/>
${rect({ ...frame, rx: 30, fill: "#FFFFFF", stroke: "#CBD5E1", extra: 'filter="url(#shadow)"' })}
${text(1025, 96, "money - Locale Currency Resolution", "title", 'text-anchor="middle" dominant-baseline="middle"')}
${text(1025, 130, "Explicit region tags plus CLDR tender data prevent hidden currency guesses", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

${panels.map((panel) => `${rect(panel)}\n${text(panel.x + panel.w / 2, panel.y + 34, panel.label, "panel-label", 'text-anchor="middle" dominant-baseline="middle"')}`).join("\n")}

${nodes.map(card).join("\n")}

${routes.map(route).join("\n")}
${labels.map(labelPill).join("\n")}

<rect x="150" y="978" width="1750" height="52" rx="12" fill="#F8FAFC" stroke="#E2E8F0"/>
${text(1025, 1004, "Locale mapping is a current-region convenience, not an accounting or legal-tender authority.", "footer", 'text-anchor="middle" dominant-baseline="middle"')}
</svg>
`;
  writeFileSync(paths.svg, svg);
}

function rectsOverlap(a, b) {
  return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
}

function pointOnBoundary(point, box) {
  const [x, y] = point;
  const onHorizontal = (y === box.y || y === box.y + box.h) && x >= box.x && x <= box.x + box.w;
  const onVertical = (x === box.x || x === box.x + box.w) && y >= box.y && y <= box.y + box.h;
  return onHorizontal || onVertical;
}

function segmentIntersectsRect(a, b, box) {
  const [x1, y1] = a;
  const [x2, y2] = b;
  if (x1 === x2) {
    const minY = Math.min(y1, y2);
    const maxY = Math.max(y1, y2);
    return x1 > box.x && x1 < box.x + box.w && maxY > box.y && minY < box.y + box.h;
  }
  if (y1 === y2) {
    const minX = Math.min(x1, x2);
    const maxX = Math.max(x1, x2);
    return y1 > box.y && y1 < box.y + box.h && maxX > box.x && minX < box.x + box.w;
  }
  return true;
}

function validateGeometry() {
  let badEndpointAngle = 0;
  let badBends = 0;
  let interiorCrossings = 0;
  let nodeOverlaps = 0;
  let laneClearance = 0;

  const byID = new Map(nodes.map((node) => [node.id, node]));
  for (let i = 0; i < nodes.length; i++) {
    for (let j = i + 1; j < nodes.length; j++) {
      if (rectsOverlap(nodes[i], nodes[j])) nodeOverlaps++;
    }
  }

  for (const r of routes) {
    const from = byID.get(r.from);
    const to = byID.get(r.to);
    if (!pointOnBoundary(r.points[0], from) || !pointOnBoundary(r.points.at(-1), to)) {
      badEndpointAngle++;
    }
    for (let i = 0; i < r.points.length - 1; i++) {
      const a = r.points[i];
      const b = r.points[i + 1];
      if (a[0] !== b[0] && a[1] !== b[1]) badBends++;
      for (const node of nodes) {
        if (node.id === r.from || node.id === r.to) continue;
        if (segmentIntersectsRect(a, b, node)) interiorCrossings++;
      }
    }
  }

  const margins = { left: frame.x, right: canvas.w - frame.x - frame.w, top: frame.y, bottom: canvas.h - frame.y - frame.h };
  const marginValues = Object.values(margins);
  const marginImbalance = Math.max(...marginValues) - Math.min(...marginValues);
  const segments = routes.reduce((sum, r) => sum + r.points.length - 1, 0);
  const summary = `money-locale-currency-resolution-flow: nodes=${nodes.length} routes=${routes.length} segments=${segments} badEndpointAngle=${badEndpointAngle} badBends=${badBends} interiorCrossings=${interiorCrossings} nodeOverlaps=${nodeOverlaps} laneClearance=${laneClearance} marginImbalance=${marginImbalance} margins=L${margins.left}/R${margins.right}/T${margins.top}/B${margins.bottom} titleGap=${titleGap}`;
  console.log(summary);
  if (badEndpointAngle || badBends || interiorCrossings || nodeOverlaps || laneClearance || marginImbalance > 0) {
    throw new Error(summary);
  }
}

mkdirSync(outDir, { recursive: true });
writeDot();
writeFileSync(paths.plain, run("dot", ["-Tplain", paths.dot]));
run("dot", ["-Tsvg", paths.dot, "-o", paths.graphvizSvg]);
run("rsvg-convert", [paths.graphvizSvg, "-o", paths.graphvizPng]);
writeSvg();
validateGeometry();
run("xmllint", ["--noout", paths.svg]);
run("rsvg-convert", [paths.svg, "-o", paths.png]);
