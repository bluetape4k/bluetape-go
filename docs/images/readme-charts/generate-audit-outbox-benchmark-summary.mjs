#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const memoryInput = join(repoRoot, "docs/research/outputs/issue-439/audit-memory-bench.txt");
const sqlInput = join(repoRoot, "docs/research/outputs/issue-439/audit-sqloutbox-postgres-bench.txt");
const svgPath = join(repoRoot, "docs/images/readme-charts/audit-outbox-benchmark-summary.svg");
const vegaPath = join(repoRoot, "docs/images/readme-charts/audit-outbox-benchmark-summary.vl.json");

const memoryLabels = new Map([
  ["MemoryRepositoryAppend/History16/Batch1/Payload256", "Append history 16, batch 1"],
  ["MemoryRepositoryAppend/History256/Batch8/Payload2048", "Append history 256, batch 8"],
  ["MemoryRepositoryFind/SingleAggregateHistory16", "Find one aggregate, 16 rows"],
  ["MemoryRepositoryFind/TypeScan64AggregatesLimit32", "Find account type, 64x16 limit 32"],
  ["MemoryRepositoryLoadHistory/Small16/Payload256", "Load history 16"],
  ["MemoryRepositoryLoadHistory/Medium256/Payload2048", "Load history 256"],
  ["AuditEntryJSONRoundTrip/Payload256", "JSON round-trip 256 B"],
  ["AuditEntryJSONRoundTrip/Payload2048", "JSON round-trip 2 KiB"],
]);

const sqlLabels = new Map([
  ["AuditSQLOutboxPostgres/Enqueue/Batch10/Payload512", "Enqueue batch 10"],
  ["AuditSQLOutboxPostgres/Claim/Limit10/Pending100/Payload512", "Claim 10 of 100"],
  ["AuditSQLOutboxPostgres/RunOnce/Publish10/Payload512", "RunOnce publish 10"],
  ["AuditSQLOutboxPostgres/RunOnce/DeadLetter10/Payload512", "RunOnce dead-letter 10"],
]);

const memoryRaw = readFileSync(memoryInput, "utf8");
const sqlRaw = readFileSync(sqlInput, "utf8");
const memoryRows = parseRows(memoryRaw, memoryLabels, "memory");
const sqlRows = parseRows(sqlRaw, sqlLabels, "sql").map((row) => ({
  ...row,
  msPerOp: row.nsPerOp / 1_000_000,
}));

if (memoryRows.length !== memoryLabels.size) {
  throw new Error(`expected ${memoryLabels.size} memory rows, got ${memoryRows.length}`);
}
if (sqlRows.length !== sqlLabels.size) {
  throw new Error(`expected ${sqlLabels.size} SQL rows, got ${sqlRows.length}`);
}

const metadata = {
  cpu: memoryRaw.match(/^cpu:\s+(.+)$/m)?.[1] ?? "unknown CPU",
  goos: memoryRaw.match(/^goos:\s+(.+)$/m)?.[1] ?? "unknown",
  goarch: memoryRaw.match(/^goarch:\s+(.+)$/m)?.[1] ?? "unknown",
  memorySource: "docs/research/outputs/issue-439/audit-memory-bench.txt",
  sqlSource: "docs/research/outputs/issue-439/audit-sqloutbox-postgres-bench.txt",
};

const width = 1800;
const height = 1700;
const frame = { x: 40, y: 40, width: width - 80, height: height - 80 };
const panelX = 80;
const panelW = width - 160;
const panelYs = [184, 760, 1190];
const bandY = 1562;
const bandH = 72;

function parseRows(raw, labels, source) {
  const pattern =
    /^Benchmark(.+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+B\/op\s+([\d.]+)\s+allocs\/op$/;
  return raw
    .split(/\r?\n/)
    .map((line) => pattern.exec(line.trim()))
    .filter(Boolean)
    .map((match) => {
      const id = match[1];
      const label = labels.get(id);
      if (!label) {
        throw new Error(`unknown ${source} benchmark row ${id}`);
      }
      return {
        id,
        label,
        nsPerOp: Number(match[2]),
        bytesPerOp: Number(match[3]),
        kibPerOp: Number(match[3]) / 1024,
        allocsPerOp: Number(match[4]),
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
  if (value <= 1000) return 1000;
  return Math.ceil(value / 1000) * 1000;
}

function text(x, y, content, cls, attrs = "") {
  return `<text x="${x}" y="${y}" class="${cls}" ${attrs}>${esc(content)}</text>`;
}

function barPanel({ y, title, subtitle, rows, metric, units, color, stroke, height: panelH, digits = 0 }) {
  const rowStartY = y + 94;
  const rowH = rows.length > 5 ? 24 : 30;
  const rowGap = rows.length > 5 ? 16 : 21;
  const labelX = panelX + 36;
  const chartX = panelX + 520;
  const chartW = panelW - 790;
  const valueX = chartX + chartW + 30;
  const max = niceMax(Math.max(...rows.map((row) => row[metric])));
  const gridBottom = rowStartY + rows.length * (rowH + rowGap) - rowGap + 12;
  const parts = [
    `<g>`,
    `<rect x="${panelX}" y="${y}" width="${panelW}" height="${panelH}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="16"/>`,
    text(panelX + 34, y + 36, title, "section", 'dominant-baseline="middle"'),
    text(panelX + 34, y + 66, subtitle, "axis-title", 'dominant-baseline="middle"'),
  ];

  for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
    const tx = chartX + chartW * tick;
    parts.push(`<line x1="${tx.toFixed(1)}" y1="${rowStartY - 22}" x2="${tx.toFixed(1)}" y2="${gridBottom}" stroke="#E2E8F0"/>`);
    parts.push(text(tx.toFixed(1), rowStartY - 36, fmt(max * tick, digits), "axis", 'text-anchor="middle"'));
  }

  for (const [index, row] of rows.entries()) {
    const rowY = rowStartY + index * (rowH + rowGap);
    const barW = Math.max(4, (row[metric] / max) * chartW);
    parts.push(text(labelX, rowY + rowH / 2, row.label, "bar-label", 'dominant-baseline="middle"'));
    parts.push(`<rect x="${chartX}" y="${rowY}" width="${barW.toFixed(1)}" height="${rowH}" fill="${color}" stroke="${stroke}" stroke-width="1.2" rx="7"/>`);
    parts.push(text(valueX, rowY + rowH / 2, `${fmt(row[metric], digits)} ${units}`, "value", 'dominant-baseline="middle"'));
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">
  <defs>
    <style>
      @font-face {
        font-family: 'Architects Daughter';
        src: local('Architects Daughter');
      }
      @font-face {
        font-family: 'Comic Mono';
        src: local('Comic Mono');
      }
      .title { font-family: 'Architects Daughter', 'Comic Sans MS', cursive; font-size: 38px; font-weight: 700; fill: #0F172A; }
      .subtitle { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 20px; fill: #475569; }
      .section { font-family: 'Architects Daughter', 'Comic Sans MS', cursive; font-size: 26px; font-weight: 700; fill: #0F172A; }
      .axis-title { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 16px; fill: #64748B; }
      .axis { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 14px; fill: #64748B; }
      .bar-label { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 15px; fill: #1E293B; }
      .value { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 15px; font-weight: 700; fill: #0F172A; }
      .callout { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 18px; fill: #334155; }
      .footer { font-family: 'Comic Mono', ui-monospace, monospace; font-size: 14px; fill: #64748B; }
    </style>
  </defs>
  <rect x="0" y="0" width="${width}" height="${height}" fill="#F8FAFC"/>
  <rect x="${frame.x}" y="${frame.y}" width="${frame.width}" height="${frame.height}" fill="#F8FAFC" stroke="#94A3B8" stroke-width="2" rx="24"/>
  ${text(width / 2, 82, "audit + sqloutbox Benchmark Summary", "title", 'text-anchor="middle" dominant-baseline="middle"')}
  ${text(width / 2, 118, `Issue #439 | ${metadata.goos}/${metadata.goarch} | ${metadata.cpu}`, "subtitle", 'text-anchor="middle" dominant-baseline="middle"')}
  ${barPanel({
    y: panelYs[0],
    title: "In-memory audit repository latency",
    subtitle: "ns/op - lower is better; append/find/load/json rows stay separate",
    rows: memoryRows,
    metric: "nsPerOp",
    units: "ns/op",
    color: "#FDE68A",
    stroke: "#D97706",
    height: 510,
    digits: 0,
  })}
  ${barPanel({
    y: panelYs[1],
    title: "PostgreSQL outbox relay latency",
    subtitle: "ms/op - lower is better; opt-in PostgreSQL",
    rows: sqlRows,
    metric: "msPerOp",
    units: "ms/op",
    color: "#BFDBFE",
    stroke: "#2563EB",
    height: 360,
    digits: 2,
  })}
  ${barPanel({
    y: panelYs[2],
    title: "Heap bytes by benchmark row",
    subtitle: "KiB/op - lower is better; largest rows",
    rows: [...memoryRows, ...sqlRows].sort((a, b) => b.kibPerOp - a.kibPerOp).slice(0, 4),
    metric: "kibPerOp",
    units: "KiB/op",
    color: "#A7F3D0",
    stroke: "#059669",
    height: 300,
    digits: 0,
  })}
  <rect x="${panelX}" y="${bandY}" width="${panelW}" height="${bandH}" fill="#ECFEFF" stroke="#0891B2" stroke-width="1.5" rx="16"/>
  ${text(panelX + 28, bandY + 27, "Raw go benchmark output is the numeric source of truth; bars summarize one local snapshot, not a production ranking.", "callout", 'dominant-baseline="middle"')}
  ${text(panelX + 28, bandY + 53, "PostgreSQL rows use postgres:16-alpine via Testcontainers and 10 second per-operation contexts.", "callout", 'dominant-baseline="middle"')}
  ${text(width / 2, height - 18, `${metadata.memorySource} | ${metadata.sqlSource}`, "footer", 'text-anchor="middle"')}
</svg>
`;

const vega = {
  title: "audit + sqloutbox Benchmark Summary",
  source: [metadata.memorySource, metadata.sqlSource],
  metricDirection: {
    "ns/op": "lower is better",
    "ms/op": "lower is better",
    "KiB/op": "lower is better",
  },
  rows: [
    ...memoryRows.map((row) => ({ scope: "audit memory", ...row })),
    ...sqlRows.map((row) => ({ scope: "sqloutbox postgres", ...row })),
  ],
};

mkdirSync(dirname(svgPath), { recursive: true });
writeFileSync(svgPath, svg);
writeFileSync(vegaPath, `${JSON.stringify(vega, null, 2)}\n`);
console.log(
  `chart gate: memoryRows=${memoryRows.length} sqlRows=${sqlRows.length} panels=3 bars=${memoryRows.length + sqlRows.length + 4} canvas=${width}x${height}`,
);
