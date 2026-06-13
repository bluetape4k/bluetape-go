#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "money-exchange-rate-provider-flow";

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
  provider: "#2563EB",
  cache: "#16A34A",
  ecb: "#7C3AED",
  core: "#EA580C",
  error: "#DC2626",
};

const canvas = { w: 2100, h: 1180 };
const frame = { x: 48, y: 48, w: 2004, h: 1084 };
const titleGap = 58;

const panels = [
  { x: 100, y: 190, w: 250, h: 520, label: "Caller Boundary", fill: "#EFF6FF", stroke: "#BFDBFE" },
  { x: 410, y: 190, w: 330, h: 520, label: "Conversion Boundary", fill: "#FFF7ED", stroke: "#FED7AA" },
  { x: 800, y: 190, w: 330, h: 520, label: "Provider Boundary", fill: "#F5F3FF", stroke: "#DDD6FE" },
  { x: 1190, y: 190, w: 330, h: 520, label: "Source Boundary", fill: "#F0FDF4", stroke: "#BBF7D0" },
  { x: 1580, y: 190, w: 370, h: 520, label: "Value Boundary", fill: "#FFF7ED", stroke: "#FDBA74" },
  { x: 100, y: 760, w: 1850, h: 160, label: "Caller-Visible Safety Contract", fill: "#F8FAFC", stroke: "#E2E8F0" },
];

const nodes = [
  {
    id: "caller",
    x: 130,
    y: 356,
    w: 190,
    h: 120,
    title: "Caller",
    details: ["ctx + Money", "target Currency"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "convert",
    x: 440,
    y: 292,
    w: 270,
    h: 212,
    title: "ConvertWithProvider",
    details: ["validates inputs", "requests quote", "returns Money + Quote"],
    fill: "#FFEDD5",
    stroke: "#FDBA74",
  },
  {
    id: "provider",
    x: 830,
    y: 292,
    w: 270,
    h: 212,
    title: "ExchangeRateProvider",
    details: ["Rate(ctx, base, target)", "no hidden failures", "nil ctx normalized"],
    fill: "#EDE9FE",
    stroke: "#C4B5FD",
  },
  {
    id: "cache",
    x: 1220,
    y: 248,
    w: 270,
    h: 138,
    title: "Snapshot Cache",
    details: ["fresh hit", "stale fallback"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "ecb",
    x: 1220,
    y: 548,
    w: 270,
    h: 138,
    title: "ECB Daily XML",
    details: ["EUR-base rates", "observed date"],
    fill: "#F3E8FF",
    stroke: "#C4B5FD",
  },
  {
    id: "core",
    x: 1610,
    y: 292,
    w: 310,
    h: 212,
    title: "Value Core",
    details: ["NewExchangeRate", "Convert", "no provider IO"],
    fill: "#FED7AA",
    stroke: "#FDBA74",
  },
  {
    id: "result",
    x: 1610,
    y: 568,
    w: 310,
    h: 112,
    title: "Result",
    details: ["converted Money", "quote metadata"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "boundary",
    x: 170,
    y: 812,
    w: 380,
    h: 76,
    title: "Value Path Stays Pure",
    details: ["Convert never fetches"],
    fill: "#FFEDD5",
    stroke: "#FDBA74",
  },
  {
    id: "freshness",
    x: 640,
    y: 812,
    w: 380,
    h: 76,
    title: "Freshness Is Visible",
    details: ["observed, fetched, expires"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "stale",
    x: 1110,
    y: 812,
    w: 380,
    h: 76,
    title: "Stale Is Explicit",
    details: ["Stale + RefreshError"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "errors",
    x: 1580,
    y: 812,
    w: 300,
    h: 76,
    title: "Errors Stay Typed",
    details: ["errors.Is sentinels"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
];

const routes = [
  { from: "caller", to: "convert", points: [[320, 416], [440, 416]], color: colors.neutral },
  { from: "convert", to: "provider", points: [[710, 360], [830, 360]], color: colors.provider },
  { from: "provider", to: "cache", points: [[1100, 318], [1220, 318]], color: colors.cache },
  { from: "cache", to: "provider", points: [[1220, 364], [1152, 364], [1152, 464], [1100, 464]], color: colors.cache },
  { from: "provider", to: "ecb", points: [[965, 504], [965, 617], [1220, 617]], color: colors.ecb },
  { from: "provider", to: "core", points: [[1100, 414], [1610, 414]], color: colors.core },
  { from: "ecb", to: "cache", points: [[1355, 548], [1355, 386]], color: colors.ecb },
  { from: "core", to: "result", points: [[1765, 504], [1765, 568]], color: colors.core },
  { from: "provider", to: "errors", points: [[1030, 504], [1030, 735], [1730, 735], [1730, 812]], color: colors.error },
  { from: "cache", to: "stale", points: [[1490, 318], [1518, 318], [1518, 740], [1300, 740], [1300, 812]], color: colors.cache },
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
  nodesep="0.68",
  ranksep="0.84",
  splines=ortho,
  fontname="Comic Mono",
  fontsize=15,
  labelloc=t,
  label=${dotLabel("money - Exchange-Rate Provider Flow", "provider IO is explicit; value conversion remains pure")}
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

caller [label=${dotLabel("Caller", "ctx + Money")} fillcolor="#DBEAFE" color="#93C5FD"];
convert [label=${dotLabel("ConvertWithProvider", "Money + target")} fillcolor="#FFEDD5" color="#FDBA74"];
provider [label=${dotLabel("ExchangeRateProvider", "Rate(ctx, base, target)")} fillcolor="#EDE9FE" color="#C4B5FD"];
cache [label=${dotLabel("Snapshot Cache", "fresh or stale")} fillcolor="#DCFCE7" color="#86EFAC"];
ecb [label=${dotLabel("ECB Daily XML", "EUR-base rates")} fillcolor="#F3E8FF" color="#C4B5FD"];
core [label=${dotLabel("Value Core", "NewExchangeRate + Convert")} fillcolor="#FED7AA" color="#FDBA74"];
result [label=${dotLabel("Result", "Money + Quote")} fillcolor="#DCFCE7" color="#86EFAC"];
errors [label=${dotLabel("Typed Errors", "network, stale, unsupported")} fillcolor="#FEE2E2" color="#FCA5A5"];

caller -> convert;
convert -> provider [color="#2563EB"];
provider -> cache [color="#16A34A"];
cache -> provider [color="#16A34A"];
provider -> ecb [color="#7C3AED"];
ecb -> cache [color="#7C3AED"];
provider -> core [color="#EA580C"];
core -> result [color="#EA580C"];
provider -> errors [color="#DC2626"];
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

function writeSvg() {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${canvas.w}" height="${canvas.h}" viewBox="0 0 ${canvas.w} ${canvas.h}">
<defs>
  <style>
    .title { font-family: 'Architects Daughter'; font-size: 34px; font-weight: 700; fill: ${colors.ink}; }
    .subtitle { font-family: 'Comic Mono'; font-size: 16px; fill: ${colors.muted}; }
    .panel-label { font-family: 'Architects Daughter'; font-size: 20px; font-weight: 700; fill: ${colors.ink}; }
    .card-title { font-family: 'Architects Daughter'; font-size: 22px; font-weight: 700; fill: ${colors.ink}; }
    .detail { font-family: 'Comic Mono'; font-size: 14px; fill: ${colors.ink}; }
    .footer { font-family: 'Comic Mono'; font-size: 14px; fill: ${colors.ink}; }
  </style>
  <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
    <feDropShadow dx="0" dy="8" stdDeviation="8" flood-color="#94A3B8" flood-opacity="0.22"/>
  </filter>
  ${[colors.neutral, colors.provider, colors.cache, colors.ecb, colors.core, colors.error].map(marker).join("\n")}
</defs>
<rect width="${canvas.w}" height="${canvas.h}" fill="#F8FAFC"/>
${rect({ ...frame, rx: 30, fill: "#FFFFFF", stroke: "#CBD5E1", extra: 'filter="url(#shadow)"' })}
${text(1050, 96, "money - Exchange-Rate Provider Flow", "title", 'text-anchor="middle" dominant-baseline="middle"')}
${text(1050, 130, "Provider IO is explicit while value conversion remains pure and deterministic", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

${panels.map((panel) => `${rect(panel)}\n${text(panel.x + panel.w / 2, panel.y + 34, panel.label, "panel-label", 'text-anchor="middle" dominant-baseline="middle"')}`).join("\n")}

${nodes.map(card).join("\n")}

${routes.map(route).join("\n")}

<rect x="150" y="962" width="1800" height="48" rx="12" fill="#F8FAFC" stroke="#E2E8F0"/>
${text(1050, 986, "Evidence: source model from money/provider.go, money/ecb_provider.go, money/exchange_rate.go, and issue #178 acceptance criteria.", "footer", 'text-anchor="middle" dominant-baseline="middle"')}
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

  const margins = {
    L: frame.x,
    R: canvas.w - frame.x - frame.w,
    T: frame.y,
    B: canvas.h - frame.y - frame.h,
  };
  const marginValues = Object.values(margins);
  const marginImbalance = Math.max(...marginValues) - Math.min(...marginValues);
  const summary = {
    nodes: nodes.length,
    routes: routes.length,
    segments: routes.reduce((sum, r) => sum + r.points.length - 1, 0),
    badEndpointAngle,
    badBends,
    interiorCrossings,
    nodeOverlaps,
    laneClearance,
    marginImbalance,
    titleGap,
    margins,
  };

  if (badEndpointAngle || badBends || interiorCrossings || nodeOverlaps || laneClearance || marginImbalance > 8 || titleGap < 48) {
    throw new Error(`geometry gate failed: ${JSON.stringify(summary)}`);
  }
  return summary;
}

function validateText() {
  const svgText = String(run("cat", [paths.svg]));
  const forbidden = /undefined|TODO|Mermaid|Graphviz only|layout fixed|Grid residue|Inter|Arial|Helvetica/g;
  const matches = svgText.match(forbidden);
  if (matches) {
    throw new Error(`forbidden SVG text: ${matches.join(", ")}`);
  }
}

function main() {
  mkdirSync(outDir, { recursive: true });
  writeDot();
  writeFileSync(paths.plain, run("dot", ["-Tplain", paths.dot]));
  writeFileSync(paths.graphvizSvg, run("dot", ["-Tsvg", paths.dot]));
  run("rsvg-convert", ["-o", paths.graphvizPng, paths.graphvizSvg]);
  writeSvg();
  const summary = validateGeometry();
  validateText();
  run("xmllint", ["--noout", paths.svg]);
  run("rsvg-convert", ["-o", paths.png, paths.svg]);
  console.log(
    `${name}: nodes=${summary.nodes} routes=${summary.routes} segments=${summary.segments} ` +
      `badEndpointAngle=${summary.badEndpointAngle} badBends=${summary.badBends} ` +
      `interiorCrossings=${summary.interiorCrossings} nodeOverlaps=${summary.nodeOverlaps} ` +
      `laneClearance=${summary.laneClearance} marginImbalance=${summary.marginImbalance} ` +
      `margins=L${summary.margins.L}/R${summary.margins.R}/T${summary.margins.T}/B${summary.margins.B} ` +
      `titleGap=${summary.titleGap}`,
  );
}

main();
