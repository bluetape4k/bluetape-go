#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "..");
const outDir = join(repoRoot, "docs/images/readme-diagrams");
const name = "redis-bloom-key-layout-01";

const paths = {
  dot: join(outDir, `${name}.dot`),
  plain: join(outDir, `${name}.plain`),
  graphvizSvg: join(outDir, `${name}-graphviz.svg`),
  graphvizPng: join(outDir, `${name}-graphviz.png`),
  svg: join(outDir, `${name}.svg`),
  png: join(outDir, `${name}.png`),
};

const fonts = {
  title: "/Users/debop/Library/Fonts/ArchitectsDaughter-Regular.ttf",
  detail: "/Users/debop/Library/Fonts/ComicMono.ttf",
  detailBold: "/Users/debop/Library/Fonts/ComicMono-Bold.ttf",
};

const colors = {
  ink: "#1E293B",
  muted: "#64748B",
  neutral: "#475569",
  api: "#2563EB",
  script: "#7C3AED",
  redis: "#16A34A",
  ops: "#EF4444",
};

const canvas = { w: 1900, h: 1060 };
const frame = { x: 44, y: 44, w: 1812, h: 972 };
const titleGap = 54;

const panels = [
  { x: 90, y: 180, w: 250, h: 470, label: "Caller", fill: "#EFF6FF", stroke: "#BFDBFE" },
  { x: 400, y: 180, w: 420, h: 470, label: "Go Package", fill: "#F5F3FF", stroke: "#DDD6FE" },
  { x: 880, y: 180, w: 380, h: 470, label: "Atomic Lua", fill: "#FFF7ED", stroke: "#FED7AA" },
  { x: 1320, y: 180, w: 490, h: 470, label: "Redis Key Pair", fill: "#F0FDF4", stroke: "#BBF7D0" },
  { x: 90, y: 700, w: 1720, h: 190, label: "Operational Boundary", fill: "#F8FAFC", stroke: "#E2E8F0" },
];

const nodes = [
  {
    id: "client",
    x: 125,
    y: 342,
    w: 180,
    h: 128,
    title: "Client",
    details: ["context.Context", "caller deadlines"],
    fill: "#DBEAFE",
    stroke: "#93C5FD",
  },
  {
    id: "api",
    x: 440,
    y: 310,
    w: 300,
    h: 160,
    title: "Redis Bloom API",
    details: ["probabilistic/redis", "Put / MightContain / Clear", "BitCount diagnostics"],
    fill: "#F3E8FF",
    stroke: "#C4B5FD",
  },
  {
    id: "lua",
    x: 930,
    y: 302,
    w: 280,
    h: 176,
    title: "Static Lua Scripts",
    details: ["metadata fingerprint guard", "one EVALSHA round trip", "KEYS + ARGV only"],
    fill: "#FEF3C7",
    stroke: "#FCD34D",
  },
  {
    id: "bits",
    x: 1375,
    y: 254,
    w: 360,
    h: 132,
    title: "{namespace}:bits",
    details: ["Redis bitmap", "GETBIT / SETBIT / BITCOUNT", "no TTL by default"],
    fill: "#DCFCE7",
    stroke: "#86EFAC",
  },
  {
    id: "config",
    x: 1375,
    y: 440,
    w: 360,
    h: 132,
    title: "{namespace}:config",
    details: ["immutable metadata hash", "ErrConfigMismatch", "ErrConfigCorrupt"],
    fill: "#D1FAE5",
    stroke: "#6EE7B7",
  },
  {
    id: "namespace",
    x: 130,
    y: 752,
    w: 360,
    h: 100,
    title: "Namespace Guard",
    details: ["no raw user IDs, emails, tokens", "Redis Cluster hash tag"],
    fill: "#E0F2FE",
    stroke: "#7DD3FC",
  },
  {
    id: "operator",
    x: 585,
    y: 752,
    w: 360,
    h: 100,
    title: "Operator Runbook",
    details: ["Clear requires approval", "rebuild into new namespace"],
    fill: "#FFE4E6",
    stroke: "#FDA4AF",
  },
  {
    id: "security",
    x: 1040,
    y: 752,
    w: 320,
    h: 100,
    title: "Redis Security",
    details: ["TLS / AUTH / ACL", "minimum command set"],
    fill: "#FEE2E2",
    stroke: "#FCA5A5",
  },
  {
    id: "diagnostics",
    x: 1455,
    y: 752,
    w: 310,
    h: 100,
    title: "Diagnostics",
    details: ["HGETALL / STRLEN / BITCOUNT", "PTTL confirms no expiry"],
    fill: "#F0FDF4",
    stroke: "#BBF7D0",
  },
];

const routes = [
  { from: "client", to: "api", points: [[305, 406], [440, 406]], color: colors.neutral },
  { from: "api", to: "lua", points: [[740, 390], [930, 390]], color: colors.script },
  { from: "lua", to: "bits", points: [[1210, 350], [1375, 350]], color: colors.redis },
  { from: "lua", to: "config", points: [[1210, 430], [1290, 430], [1290, 506], [1375, 506]], color: colors.redis },
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
  label=${dotLabel("probabilistic/redis - Redis Bloom Key Layout", "static Lua scripts keep bitmap and config checks atomic")}
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

client [label=${dotLabel("Client", "context.Context", "caller deadlines")} fillcolor="#DBEAFE" color="#93C5FD"];
api [label=${dotLabel("Redis Bloom API", "Put / MightContain / Clear", "BitCount diagnostics")} fillcolor="#F3E8FF" color="#C4B5FD"];
lua [label=${dotLabel("Static Lua Scripts", "fingerprint guard", "one EVALSHA")} fillcolor="#FEF3C7" color="#FCD34D"];
bits [label=${dotLabel("{namespace}:bits", "Redis bitmap", "GETBIT / SETBIT")} fillcolor="#DCFCE7" color="#86EFAC"];
config [label=${dotLabel("{namespace}:config", "metadata hash", "immutable config")} fillcolor="#D1FAE5" color="#6EE7B7"];
operator [label=${dotLabel("Operator Runbook", "admin Clear only", "rebuild namespace")} fillcolor="#FFE4E6" color="#FDA4AF"];

client -> api;
api -> lua [color="#7C3AED" fontcolor="#5B21B6"];
lua -> bits [color="#16A34A" fontcolor="#166534"];
lua -> config [color="#16A34A" fontcolor="#166534"];
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
    @font-face { font-family: 'Architects Daughter'; src: url('${fonts.title}'); }
    @font-face { font-family: 'Comic Mono'; src: url('${fonts.detail}'); font-weight: 400; }
    @font-face { font-family: 'Comic Mono'; src: url('${fonts.detailBold}'); font-weight: 700; }
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
  ${[colors.neutral, colors.script, colors.redis, colors.ops].map(marker).join("\n")}
</defs>
<rect width="${canvas.w}" height="${canvas.h}" fill="#F8FAFC"/>
${rect({ ...frame, rx: 30, fill: "#FFFFFF", stroke: "#CBD5E1", extra: 'filter="url(#shadow)"' })}
${text(950, 94, "probabilistic/redis - Redis Bloom Key Layout", "title", 'text-anchor="middle" dominant-baseline="middle"')}
${text(950, 128, "Cluster-safe key pair with static Lua checks for shared Bloom filter state", "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}

${panels.map((panel) => `${rect(panel)}\n${text(panel.x + panel.w / 2, panel.y + 34, panel.label, "panel-label", 'text-anchor="middle" dominant-baseline="middle"')}`).join("\n")}

${nodes.map(card).join("\n")}

${routes.map(route).join("\n")}

<rect x="125" y="930" width="1650" height="42" rx="12" fill="#F8FAFC" stroke="#E2E8F0"/>
${text(950, 951, "Evidence: DOT/plain/sketch PNG retained; final PNG uses decorated frame, layer bands, 5x5 arrows, and source-grounded labels.", "footer", 'text-anchor="middle" dominant-baseline="middle"')}
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
