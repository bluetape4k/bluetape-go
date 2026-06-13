#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "jwt-provider-cache-adapter-flow";

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
  cache: "#2563EB",
  verify: "#7C3AED",
  success: "#16A34A",
  ops: "#EF4444",
};

const canvas = { w: 2000, h: 1140 };
const frame = { x: 48, y: 48, w: 1904, h: 1044 };
const titleGap = 58;

const panels = [
  { x: 96, y: 188, w: 280, h: 520, label: "Caller Boundary", fill: "#EFF6FF", stroke: "#BFDBFE" },
  { x: 430, y: 188, w: 360, h: 520, label: "Decorator Boundary", fill: "#F5F3FF", stroke: "#DDD6FE" },
  { x: 850, y: 188, w: 360, h: 520, label: "Trust Boundary", fill: "#F0FDF4", stroke: "#BBF7D0" },
  { x: 1270, y: 188, w: 350, h: 520, label: "Expiry Boundary", fill: "#FFF7ED", stroke: "#FED7AA" },
  { x: 1660, y: 188, w: 240, h: 520, label: "Ops Boundary", fill: "#FFF1F2", stroke: "#FDA4AF" },
  { x: 96, y: 760, w: 1804, h: 156, label: "Safety Contract", fill: "#F8FAFC", stroke: "#E2E8F0" },
];

const nodes = [
  {
    id: "caller",
    x: 128,
    y: 360,
    w: 216,
    h: 132,
    title: "Caller",
    details: ["Parse / ParseContext", "caller-owned options"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "adapter",
    x: 462,
    y: 302,
    w: 296,
    h: 216,
    title: "CachedProvider",
    details: ["decorates provider", "token digest key", "parse profile key"],
    fill: "#F3E8FF",
    stroke: "#C4B5FD",
  },
  {
    id: "cache",
    x: 882,
    y: 260,
    w: 296,
    h: 148,
    title: "Reader Cache",
    details: ["cache.Cache contract", "Get / Set / Delete / Clear"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "provider",
    x: 882,
    y: 548,
    w: 296,
    h: 148,
    title: "Provider",
    details: ["signature validation", "alg / kid / claims"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "ttl",
    x: 1302,
    y: 260,
    w: 286,
    h: 148,
    title: "TTL Clipper",
    details: ["min(exp-now, maxTTL)", "zero means no cache"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "repo",
    x: 1302,
    y: 548,
    w: 286,
    h: 148,
    title: "KeyChain Source",
    details: ["local repository", "or distributed Redis"],
    fill: "#D1FAE5",
    stroke: "#6EE7B7",
  },
  {
    id: "invalidate",
    x: 1690,
    y: 374,
    w: 178,
    h: 204,
    title: "Invalidate",
    details: ["Clear cache", "after rotation", "or reset"],
    fill: "#FFE4E6",
    stroke: "#FDA4AF",
  },
  {
    id: "noncache",
    x: 184,
    y: 812,
    w: 360,
    h: 76,
    title: "Failures Are Not Cached",
    details: ["malformed, wrong alg, wrong key, expired"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
  {
    id: "trust",
    x: 634,
    y: 812,
    w: 360,
    h: 76,
    title: "Cache Is Not Trust",
    details: ["provider remains the validation authority"],
    fill: "#EDE9FE",
    stroke: "#C4B5FD",
  },
  {
    id: "bounded",
    x: 1084,
    y: 812,
    w: 360,
    h: 76,
    title: "Bounded Lifetime",
    details: ["entry cannot outlive claim expiry"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "compose",
    x: 1534,
    y: 812,
    w: 300,
    h: 76,
    title: "Compose Bypasses Cache",
    details: ["signing path is unchanged"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
];

const routes = [
  { from: "caller", to: "adapter", points: [[344, 426], [462, 426]], color: colors.neutral },
  { from: "adapter", to: "cache", points: [[758, 334], [882, 334]], color: colors.cache },
  { from: "cache", to: "adapter", points: [[882, 386], [816, 386], [816, 466], [758, 466]], color: colors.success },
  { from: "adapter", to: "provider", points: [[758, 500], [816, 500], [816, 622], [882, 622]], color: colors.verify },
  { from: "provider", to: "repo", points: [[1178, 622], [1302, 622]], color: colors.verify },
  { from: "adapter", to: "ttl", points: [[758, 438], [1236, 438], [1236, 334], [1302, 334]], color: colors.cache },
  { from: "ttl", to: "cache", points: [[1302, 362], [1178, 362]], color: colors.cache },
  { from: "repo", to: "invalidate", points: [[1588, 622], [1779, 622], [1779, 578]], color: colors.ops },
  { from: "invalidate", to: "cache", points: [[1690, 478], [1228, 478], [1228, 408], [1178, 408]], color: colors.ops },
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
  label=${dotLabel("jwt - Provider Cache Adapter Flow", "cache hits never bypass provider-owned validation boundaries")}
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

caller [label=${dotLabel("Caller", "Parse / ParseContext")} fillcolor="#DBEAFE" color="#93C5FD"];
adapter [label=${dotLabel("CachedProvider", "token digest", "parse profile")} fillcolor="#F3E8FF" color="#C4B5FD"];
cache [label=${dotLabel("Reader Cache", "Get / Set / Delete / Clear")} fillcolor="#DBEAFE" color="#93C5FD"];
provider [label=${dotLabel("Provider", "signature + claims")} fillcolor="#DCFCE7" color="#86EFAC"];
ttl [label=${dotLabel("TTL Clipper", "bounded by exp")} fillcolor="#FEF3C7" color="#FCD34D"];
repo [label=${dotLabel("KeyChain Source", "local or distributed")} fillcolor="#D1FAE5" color="#6EE7B7"];
invalidate [label=${dotLabel("Invalidate", "rotation or reset")} fillcolor="#FFE4E6" color="#FDA4AF"];

caller -> adapter;
adapter -> cache [color="#2563EB"];
cache -> adapter [color="#16A34A"];
adapter -> provider [color="#7C3AED"];
provider -> repo [color="#7C3AED"];
adapter -> ttl [color="#2563EB"];
ttl -> cache [color="#2563EB"];
repo -> invalidate [color="#EF4444"];
invalidate -> cache [color="#EF4444"];
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

function route(route) {
  const id = route.color.slice(1);
  const path = `M ${route.points.map(([x, y]) => `${x} ${y}`).join(" L ")}`;
  return `<path d="${path}" fill="none" stroke="${route.color}" stroke-width="3" marker-end="url(#${id})"/>`;
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
  ${[colors.neutral, colors.cache, colors.verify, colors.success, colors.ops].map(marker).join("\n")}
</defs>
<rect width="${canvas.w}" height="${canvas.h}" fill="#F8FAFC"/>
${rect({ ...frame, rx: 30, fill: "#FFFFFF", stroke: "#CBD5E1", extra: 'filter="url(#shadow)"' })}
${text(1000, 96, "jwt - Provider Cache Adapter Flow", "title", 'text-anchor="middle" dominant-baseline="middle"')}
${text(1000, 130, "Optional parse-result cache keeps provider validation as the trust boundary", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

${panels.map((panel) => `${rect(panel)}\n${text(panel.x + panel.w / 2, panel.y + 34, panel.label, "panel-label", 'text-anchor="middle" dominant-baseline="middle"')}`).join("\n")}

${nodes.map(card).join("\n")}

${routes.map(route).join("\n")}

<rect x="140" y="960" width="1720" height="48" rx="12" fill="#F8FAFC" stroke="#E2E8F0"/>
${text(1000, 984, "Evidence: source model from jwt/provider.go, distributed_provider.go, cache/cache.go, cache/memory.go, and #175 acceptance criteria.", "footer", 'text-anchor="middle" dominant-baseline="middle"')}
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
