#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const inputPath = join(
  repoRoot,
  "docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt",
);
const svgPath = join(
  repoRoot,
  "docs/images/readme-charts/distributed-jwt-redis-benchmark.svg",
);
const vegaPath = join(
  repoRoot,
  "docs/images/readme-charts/distributed-jwt-redis-benchmark.vl.json",
);

const labels = {
  RepositoryFind: "Repository Find",
  RepositoryRotateCurrentHit: "Rotate current hit",
  RepositoryRotateExpired: "Rotate expired",
  RepositoryForcedRotate: "Forced rotate",
  DistributedProviderComposeContext: "Provider Compose",
  DistributedProviderParseContext: "Provider Parse",
};

const raw = readFileSync(inputPath, "utf8");
const rows = [];
const linePattern =
  /^BenchmarkRedis([A-Za-z]+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+B\/op\s+([\d.]+)\s+allocs\/op$/gm;

for (const match of raw.matchAll(linePattern)) {
  const [, name, nsPerOp, bytesPerOp, allocsPerOp] = match;
  if (!labels[name]) {
    throw new Error(`unknown benchmark row ${name}`);
  }
  rows.push({
    name,
    label: labels[name],
    group: name.startsWith("Distributed") ? "Provider" : "Repository",
    nsPerOp: Number(nsPerOp),
    bytesPerOp: Number(bytesPerOp),
    allocsPerOp: Number(allocsPerOp),
  });
}

if (rows.length !== Object.keys(labels).length) {
  throw new Error(`expected ${Object.keys(labels).length} benchmark rows, got ${rows.length}`);
}

const cpu = raw.match(/^cpu:\s+(.+)$/m)?.[1] ?? "unknown CPU";

function esc(text) {
  return String(text)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function fmt(value) {
  return new Intl.NumberFormat("en-US", {
    maximumFractionDigits: 0,
  }).format(value);
}

function text(x, y, content, cls, attrs = "") {
  return `<text x="${x}" y="${y}" class="${cls}" ${attrs}>${esc(content)}</text>`;
}

function niceMax(value) {
  if (value <= 100) return 100;
  if (value <= 250) return 250;
  if (value <= 500) return 500;
  if (value <= 1000) return 1000;
  if (value <= 2500) return 2500;
  if (value <= 5000) return 5000;
  if (value <= 7500) return 7500;
  if (value <= 10000) return 10000;
  if (value <= 250000) return 250000;
  if (value <= 500000) return 500000;
  return Math.ceil(value / 100000) * 100000;
}

function color(row) {
  if (row.group === "Provider") {
    return { fill: "#A7F3D0", stroke: "#059669" };
  }
  return { fill: "#BFDBFE", stroke: "#2563EB" };
}

function panel({ y, title, subtitle, metric, units }) {
  const x = 56;
  const width = 1168;
  const height = 320;
  const rowStartY = y + 96;
  const rowH = 26;
  const rowGap = 10;
  const labelX = x + 30;
  const chartX = x + 320;
  const chartW = 720;
  const valueX = chartX + chartW + 20;
  const max = niceMax(Math.max(...rows.map((row) => row[metric])));
  const parts = [
    `<g>`,
    `<rect x="${x}" y="${y}" width="${width}" height="${height}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="14"/>`,
    text(x + 28, y + 34, title, "section", 'dominant-baseline="middle"'),
    text(x + 28, y + 62, `${units}; lower is better`, "axis", 'dominant-baseline="middle"'),
  ];

  for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
    const tx = chartX + chartW * tick;
    parts.push(`<line x1="${tx.toFixed(1)}" y1="${rowStartY - 18}" x2="${tx.toFixed(1)}" y2="${rowStartY + rows.length * (rowH + rowGap) - 6}" stroke="#E2E8F0"/>`);
    parts.push(text(tx.toFixed(1), rowStartY - 28, fmt(max * tick), "axis", 'text-anchor="middle"'));
  }

  for (const [index, row] of rows.entries()) {
    const rowY = rowStartY + index * (rowH + rowGap);
    const barW = Math.max(2, (row[metric] / max) * chartW);
    const c = color(row);
    parts.push(text(labelX, rowY + rowH / 2, row.label, "bar-label", 'dominant-baseline="middle"'));
    parts.push(`<rect x="${chartX}" y="${rowY}" width="${barW.toFixed(1)}" height="${rowH}" fill="${c.fill}" stroke="${c.stroke}" stroke-width="1.2" rx="7"/>`);
    parts.push(text(valueX, rowY + rowH / 2, fmt(row[metric]), "value", 'dominant-baseline="middle"'));
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="1240" viewBox="0 0 1280 1240">
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
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #1E293B; font-weight: bold; }
      .subtitle { font-family: 'Comic Mono'; font-size: 14px; fill: #64748B; }
      .section { font-family: 'Architects Daughter'; font-size: 24px; fill: #1E293B; font-weight: bold; }
      .bar-label { font-family: 'Architects Daughter'; font-size: 17px; fill: #1E293B; }
      .axis { font-family: 'Comic Mono'; font-size: 11px; fill: #64748B; }
      .value { font-family: 'Comic Mono'; font-size: 13px; fill: #334155; font-weight: bold; }
      .legend { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; }
      .callout-title { font-family: 'Architects Daughter'; font-size: 20px; fill: #1E293B; font-weight: bold; }
      .callout { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; }
    </style>
  </defs>

  <rect x="0" y="0" width="1280" height="1240" fill="#F8FAFC" stroke="#E2E8F0" stroke-width="1.5" rx="18"/>
  <text x="640" y="42" text-anchor="middle" dominant-baseline="middle" class="title">Distributed JWT Redis Benchmark</text>
  <text x="640" y="74" text-anchor="middle" dominant-baseline="middle" class="subtitle">Issue #173 · local Testcontainers Redis snapshot · ${esc(cpu)} · benchtime 100ms</text>

  ${panel({
    y: 112,
    title: "Latency",
    subtitle: "Redis-backed operation cost",
    metric: "nsPerOp",
    units: "ns/op",
  })}

  ${panel({
    y: 456,
    title: "Heap bytes",
    subtitle: "Per-operation allocation volume",
    metric: "bytesPerOp",
    units: "B/op",
  })}

  ${panel({
    y: 800,
    title: "Allocations",
    subtitle: "Allocation count per operation",
    metric: "allocsPerOp",
    units: "allocs/op",
  })}

  <g>
    <rect x="56" y="1164" width="1168" height="44" fill="#FFFBEB" stroke="#F59E0B" stroke-width="1.4" rx="12"/>
    <text x="84" y="1186" dominant-baseline="middle" class="callout-title">Interpretation boundary</text>
    <text x="310" y="1186" dominant-baseline="middle" class="callout">One local smoke run. Use raw benchmark output as the numeric source; bar length is for scan-friendly comparison only.</text>
  </g>
</svg>
`;

const vegaLite = {
  $schema: "https://vega.github.io/schema/vega-lite/v5.json",
  title: "Distributed JWT Redis Benchmark",
  data: { values: rows },
  vconcat: [
    chartSpec("nsPerOp", "ns/op"),
    chartSpec("bytesPerOp", "B/op"),
    chartSpec("allocsPerOp", "allocs/op"),
  ],
};

function chartSpec(field, title) {
  return {
    width: 620,
    height: 170,
    mark: { type: "bar", cornerRadiusEnd: 4 },
    encoding: {
      y: { field: "label", type: "nominal", sort: null, title: null },
      x: { field, type: "quantitative", title: `${title}; lower is better` },
      color: { field: "group", type: "nominal", scale: { range: ["#2563EB", "#059669"] } },
      tooltip: [
        { field: "label", type: "nominal" },
        { field, type: "quantitative" },
      ],
    },
    title,
  };
}

mkdirSync(dirname(svgPath), { recursive: true });
writeFileSync(svgPath, svg);
writeFileSync(vegaPath, `${JSON.stringify(vegaLite, null, 2)}\n`);
console.log(`wrote ${svgPath}`);
console.log(`wrote ${vegaPath}`);
console.log(`chart gate: rows=${rows.length} panels=3 bars=${rows.length * 3} source=${inputPath}`);
