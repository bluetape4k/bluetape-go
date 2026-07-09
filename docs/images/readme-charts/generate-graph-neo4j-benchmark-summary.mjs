#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const mappingInput = join(
  repoRoot,
  "docs/research/outputs/issue-438/graph-neo4j-mapping-bench.txt",
);
const containerInput = join(
  repoRoot,
  "docs/research/outputs/issue-438/graph-neo4j-containers-bench.txt",
);
const svgPath = join(
  repoRoot,
  "docs/images/readme-charts/graph-neo4j-benchmark-summary.svg",
);
const vegaPath = join(
  repoRoot,
  "docs/images/readme-charts/graph-neo4j-benchmark-summary.vl.json",
);

const mappingLabels = new Map([
  ["VertexFromNode", "Vertex from node"],
  ["EdgeFromRelationship", "Edge from relationship"],
  ["VerticesFromRecords", "Vertices from 100 records"],
  ["EdgesFromRecords", "Edges from 100 records"],
]);

const containerLabels = new Map([
  ["WriteNode", "Write node"],
  ["WriteRelationship", "Write relationship"],
  ["ReadVertices/Small10", "Read vertices 10"],
  ["ReadVertices/Medium100", "Read vertices 100"],
  ["ReadEdges/Small10", "Read edges 10"],
  ["ReadEdges/Medium100", "Read edges 100"],
  ["ReadEmptyResult", "Read empty"],
  ["WriteSyntaxError", "Write syntax error"],
]);

const mappingRaw = readFileSync(mappingInput, "utf8");
const containerRaw = readFileSync(containerInput, "utf8");
const mappingRows = parseMapping(mappingRaw);
const containerRows = parseContainers(containerRaw);

if (mappingRows.length !== mappingLabels.size) {
  throw new Error(`expected ${mappingLabels.size} mapping rows, got ${mappingRows.length}`);
}
if (containerRows.length !== containerLabels.size * 2) {
  throw new Error(`expected ${containerLabels.size * 2} container rows, got ${containerRows.length}`);
}

const metadata = {
  cpu: mappingRaw.match(/^cpu:\s+(.+)$/m)?.[1] ?? "unknown CPU",
  goos: mappingRaw.match(/^goos:\s+(.+)$/m)?.[1] ?? "unknown",
  goarch: mappingRaw.match(/^goarch:\s+(.+)$/m)?.[1] ?? "unknown",
  mappingSource: "docs/research/outputs/issue-438/graph-neo4j-mapping-bench.txt",
  containerSource: "docs/research/outputs/issue-438/graph-neo4j-containers-bench.txt",
};

const width = 1800;
const height = 1840;
const frame = { x: 40, y: 40, width: width - 80, height: height - 80 };
const panelX = 80;
const panelW = width - 160;
const panelYs = [184, 526, 1130];
const bandY = 1720;
const bandH = 74;

function parseMapping(raw) {
  const pattern =
    /^Benchmark([A-Za-z0-9]+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+B\/op\s+([\d.]+)\s+allocs\/op$/;
  return raw
    .split(/\r?\n/)
    .map((line) => pattern.exec(line.trim()))
    .filter(Boolean)
    .map((match) => {
      const id = match[1];
      const label = mappingLabels.get(id);
      if (!label) {
        throw new Error(`unknown mapping benchmark row ${id}`);
      }
      return {
        id,
        label,
        nsPerOp: Number(match[2]),
        bytesPerOp: Number(match[3]),
        allocsPerOp: Number(match[4]),
      };
    });
}

function parseContainers(raw) {
  const pattern =
    /^BenchmarkGraphNeo4jContainers\/(Neo4j|Memgraph)\/(?:neo4j:5\.26\.0|memgraph:3\.5\.0)\/(.+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+B\/op\s+([\d.]+)\s+allocs\/op$/;
  return raw
    .split(/\r?\n/)
    .map((line) => pattern.exec(line.trim()))
    .filter(Boolean)
    .map((match) => {
      const runtime = match[1];
      const id = match[2];
      const label = containerLabels.get(id);
      if (!label) {
        throw new Error(`unknown container benchmark row ${id}`);
      }
      return {
        id,
        label,
        runtime,
        nsPerOp: Number(match[3]),
        msPerOp: Number(match[3]) / 1_000_000,
        bytesPerOp: Number(match[4]),
        kibPerOp: Number(match[4]) / 1024,
        allocsPerOp: Number(match[5]),
      };
    });
}

function esc(text) {
  return String(text)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function fmt(value, digits = 0) {
  return new Intl.NumberFormat("en-US", {
    maximumFractionDigits: digits,
    minimumFractionDigits: digits,
  }).format(value);
}

function niceMax(value) {
  if (value <= 1) return 1;
  if (value <= 2.5) return 2.5;
  if (value <= 5) return 5;
  if (value <= 10) return 10;
  if (value <= 25) return 25;
  if (value <= 50) return 50;
  if (value <= 100) return 100;
  if (value <= 250) return 250;
  if (value <= 500) return 500;
  return Math.ceil(value / 100) * 100;
}

function text(x, y, content, cls, attrs = "") {
  return `<text x="${x}" y="${y}" class="${cls}" ${attrs}>${esc(content)}</text>`;
}

function legendItem(x, y, fill, stroke, label) {
  return [
    `<rect x="${x}" y="${y - 13}" width="28" height="18" fill="${fill}" stroke="${stroke}" stroke-width="1.2" rx="5"/>`,
    text(x + 40, y - 3, label, "legend", 'dominant-baseline="middle"'),
  ].join("\n");
}

function colorForRuntime(runtime) {
  if (runtime === "Neo4j") {
    return { fill: "#BFDBFE", stroke: "#2563EB" };
  }
  return { fill: "#A7F3D0", stroke: "#059669" };
}

function mappingPanel() {
  const y = panelYs[0];
  const panelH = 278;
  const rowStartY = y + 94;
  const rowH = 28;
  const rowGap = 20;
  const labelX = panelX + 38;
  const chartX = panelX + 430;
  const chartW = panelW - 670;
  const valueX = chartX + chartW + 30;
  const max = niceMax(Math.max(...mappingRows.map((row) => row.nsPerOp)));
  const gridBottom = rowStartY + mappingRows.length * (rowH + rowGap) - rowGap + 12;
  const parts = [
    `<g>`,
    `<rect x="${panelX}" y="${y}" width="${panelW}" height="${panelH}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="16"/>`,
    text(panelX + 34, y + 36, "Pure mapping latency", "section", 'dominant-baseline="middle"'),
    text(panelX + 34, y + 66, "ns/op - lower is better; no Docker startup", "axis-title", 'dominant-baseline="middle"'),
  ];

  for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
    const tx = chartX + chartW * tick;
    parts.push(`<line x1="${tx.toFixed(1)}" y1="${rowStartY - 22}" x2="${tx.toFixed(1)}" y2="${gridBottom}" stroke="#E2E8F0"/>`);
    parts.push(text(tx.toFixed(1), rowStartY - 36, fmt(max * tick, 0), "axis", 'text-anchor="middle"'));
  }

  for (const [index, row] of mappingRows.entries()) {
    const rowY = rowStartY + index * (rowH + rowGap);
    const barW = Math.max(4, (row.nsPerOp / max) * chartW);
    parts.push(text(labelX, rowY + rowH / 2, row.label, "bar-label", 'dominant-baseline="middle"'));
    parts.push(`<rect x="${chartX}" y="${rowY}" width="${barW.toFixed(1)}" height="${rowH}" fill="#FDE68A" stroke="#D97706" stroke-width="1.2" rx="7"/>`);
    parts.push(text(valueX, rowY + rowH / 2, `${fmt(row.nsPerOp, row.nsPerOp < 200 ? 1 : 0)} ns/op`, "value", 'dominant-baseline="middle"'));
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

function groupedPanel({ y, title, metric, units, maxDigits }) {
  const panelH = 560;
  const rowStartY = y + 98;
  const groupGap = 20;
  const barH = 17;
  const barGap = 6;
  const labelX = panelX + 36;
  const chartX = panelX + 432;
  const chartW = panelW - 690;
  const valueX = chartX + chartW + 30;
  const orderedIds = Array.from(containerLabels.keys());
  const max = niceMax(Math.max(...containerRows.map((row) => row[metric])));
  const gridBottom = rowStartY + orderedIds.length * (barH * 2 + barGap + groupGap) - groupGap + 12;
  const parts = [
    `<g>`,
    `<rect x="${panelX}" y="${y}" width="${panelW}" height="${panelH}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="16"/>`,
    text(panelX + 34, y + 38, title, "section", 'dominant-baseline="middle"'),
    text(panelX + 34, y + 70, `${units} - lower is better; local Testcontainers snapshot`, "axis-title", 'dominant-baseline="middle"'),
    legendItem(panelX + panelW - 390, y + 46, "#BFDBFE", "#2563EB", "Neo4j 5.26.0"),
    legendItem(panelX + panelW - 204, y + 46, "#A7F3D0", "#059669", "Memgraph 3.5.0"),
  ];

  for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
    const tx = chartX + chartW * tick;
    parts.push(`<line x1="${tx.toFixed(1)}" y1="${rowStartY - 22}" x2="${tx.toFixed(1)}" y2="${gridBottom}" stroke="#E2E8F0"/>`);
    parts.push(text(tx.toFixed(1), rowStartY - 36, fmt(max * tick, maxDigits), "axis", 'text-anchor="middle"'));
  }

  for (const [index, id] of orderedIds.entries()) {
    const label = containerLabels.get(id);
    const groupY = rowStartY + index * (barH * 2 + barGap + groupGap);
    parts.push(text(labelX, groupY + barH + barGap / 2, label, "bar-label", 'dominant-baseline="middle"'));

    for (const [runtimeIndex, runtime] of ["Neo4j", "Memgraph"].entries()) {
      const row = containerRows.find((candidate) => candidate.id === id && candidate.runtime === runtime);
      if (!row) {
        throw new Error(`missing ${runtime} row for ${id}`);
      }
      const rowY = groupY + runtimeIndex * (barH + barGap);
      const barW = Math.max(4, (row[metric] / max) * chartW);
      const c = colorForRuntime(runtime);
      parts.push(`<rect x="${chartX}" y="${rowY}" width="${barW.toFixed(1)}" height="${barH}" fill="${c.fill}" stroke="${c.stroke}" stroke-width="1.1" rx="5"/>`);
      parts.push(text(valueX, rowY + barH / 2, `${fmt(row[metric], maxDigits)} ${units}`, "value", 'dominant-baseline="middle"'));
    }
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">
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
      .title { font-family: 'Architects Daughter'; font-size: 44px; fill: #1E293B; font-weight: bold; }
      .subtitle { font-family: 'Comic Mono'; font-size: 16px; fill: #475569; }
      .section { font-family: 'Architects Daughter'; font-size: 28px; fill: #1E293B; font-weight: bold; }
      .bar-label { font-family: 'Architects Daughter'; font-size: 20px; fill: #1E293B; }
      .axis-title { font-family: 'Comic Mono'; font-size: 14px; fill: #475569; font-weight: bold; }
      .axis { font-family: 'Comic Mono'; font-size: 12px; fill: #64748B; }
      .value { font-family: 'Comic Mono'; font-size: 14px; fill: #334155; font-weight: bold; }
      .legend { font-family: 'Comic Mono'; font-size: 14px; fill: #334155; }
      .callout-title { font-family: 'Architects Daughter'; font-size: 23px; fill: #1E293B; font-weight: bold; }
      .callout { font-family: 'Comic Mono'; font-size: 13px; fill: #334155; }
      .footer { font-family: 'Comic Mono'; font-size: 12px; fill: #64748B; }
    </style>
  </defs>

  <rect x="0" y="0" width="${width}" height="${height}" fill="#F8FAFC"/>
  <rect x="${frame.x}" y="${frame.y}" width="${frame.width}" height="${frame.height}" fill="#F8FAFC" stroke="#CBD5E1" stroke-width="2" rx="24"/>
  <text x="${width / 2}" y="82" text-anchor="middle" dominant-baseline="middle" class="title">graph/neo4j Benchmark Summary</text>
  <text x="${width / 2}" y="122" text-anchor="middle" dominant-baseline="middle" class="subtitle">Issue #438 · ${esc(metadata.cpu)} · ${esc(metadata.goos)}/${esc(metadata.goarch)} · Neo4j/Memgraph rows are opt-in Testcontainers evidence</text>

  ${mappingPanel()}

  ${groupedPanel({
    y: panelYs[1],
    title: "Container adapter latency",
    metric: "msPerOp",
    units: "ms/op",
    maxDigits: 2,
  })}

  ${groupedPanel({
    y: panelYs[2],
    title: "Container heap bytes",
    metric: "kibPerOp",
    units: "KiB/op",
    maxDigits: 1,
  })}

  <g>
    <rect x="${panelX}" y="${bandY}" width="${panelW}" height="${bandH}" fill="#FFFBEB" stroke="#F59E0B" stroke-width="1.5" rx="16"/>
    <text x="${panelX + 34}" y="${bandY + 28}" dominant-baseline="middle" class="callout-title">Interpretation boundary</text>
    <text x="${panelX + 320}" y="${bandY + 25}" dominant-baseline="middle" class="callout">Raw benchmark output is the numeric source of truth. Bars compare one local snapshot, not production database ranking.</text>
    <text x="${panelX + 320}" y="${bandY + 50}" dominant-baseline="middle" class="footer">Container rows include caller-side params and 10s bounded operation contexts.</text>
  </g>
</svg>
`;

const vegaLite = {
  $schema: "https://vega.github.io/schema/vega-lite/v5.json",
  title: "graph/neo4j Benchmark Summary",
  source: [metadata.mappingSource, metadata.containerSource],
  metadata,
  mappingRows,
  containerRows,
};

mkdirSync(dirname(svgPath), { recursive: true });
writeFileSync(svgPath, svg);
writeFileSync(vegaPath, `${JSON.stringify(vegaLite, null, 2)}\n`);

console.log(`wrote ${svgPath}`);
console.log(`wrote ${vegaPath}`);
console.log(
  `chart gate: mappingRows=${mappingRows.length} containerRows=${containerRows.length} ` +
    `panels=3 bars=${mappingRows.length + containerRows.length * 2} canvas=${width}x${height}`,
);
