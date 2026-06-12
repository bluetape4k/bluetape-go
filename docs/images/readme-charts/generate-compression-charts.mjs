#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const inputPath = join(
  repoRoot,
  "docs/research/outputs/issue-195/go-compression-bench.txt",
);
const outputPath = join(
  repoRoot,
  "docs/images/readme-charts/compression-large-payload-benchmark-bars.svg",
);

const payloads = ["json", "text", "binary", "random"];
const algorithms = ["gzip", "zlib", "deflate", "zstd", "lz4", "snappy"];
const labels = {
  gzip: "gzip",
  zlib: "zlib",
  deflate: "deflate",
  zstd: "zstd",
  lz4: "lz4",
  snappy: "snappy",
};
const colors = {
  gzip: { fill: "#FDBA74", stroke: "#F97316" },
  zlib: { fill: "#FDE68A", stroke: "#D97706" },
  deflate: { fill: "#C4B5FD", stroke: "#7C3AED" },
  zstd: { fill: "#93C5FD", stroke: "#2563EB" },
  lz4: { fill: "#86EFAC", stroke: "#16A34A" },
  snappy: { fill: "#F9A8D4", stroke: "#DB2777" },
};

const raw = readFileSync(inputPath, "utf8");
const rows = [];
const linePattern =
  /^BenchmarkCompressors(Compress|Decompress)\/([a-z]+)\/large\/([a-z0-9]+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+MB\/s\s+([\d.]+)\s+compressed\/original\s+([\d.]+)\s+compressed_bytes/mg;

for (const match of raw.matchAll(linePattern)) {
  const [, phase, payload, algorithm, nsPerOp, mbPerSec, ratio, compressedBytes] =
    match;
  rows.push({
    phase,
    payload,
    algorithm,
    nsPerOp: Number(nsPerOp),
    mbPerSec: Number(mbPerSec),
    ratio: Number(ratio),
    compressedBytes: Number(compressedBytes),
  });
}

const expectedRows = 2 * payloads.length * algorithms.length;
if (rows.length !== expectedRows) {
  throw new Error(`expected ${expectedRows} large-payload rows, got ${rows.length}`);
}

function row(phase, payload, algorithm) {
  const found = rows.find(
    (candidate) =>
      candidate.phase === phase &&
      candidate.payload === payload &&
      candidate.algorithm === algorithm,
  );
  if (!found) {
    throw new Error(`missing row for ${phase}/${payload}/${algorithm}`);
  }
  return found;
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
  }).format(value);
}

function fmtRatio(value) {
  return value >= 0.1 ? value.toFixed(3) : value.toFixed(4);
}

function text(x, y, content, cls, attrs = "") {
  return `<text x="${x}" y="${y}" class="${cls}" ${attrs}>${esc(content)}</text>`;
}

function payloadTitle(payload) {
  return payload.toUpperCase();
}

function niceMax(value, metric) {
  if (metric === "ratio") {
    if (value <= 0.25) return 0.25;
    if (value <= 0.5) return 0.5;
    return 1;
  }
  if (value <= 1500) return 1500;
  if (value <= 2500) return 2500;
  if (value <= 4000) return 4000;
  if (value <= 6000) return 6000;
  return 8000;
}

function fmtTick(value, metric) {
  if (metric === "ratio") {
    return value === 0 || value === 1 ? value.toFixed(0) : value.toFixed(2);
  }
  return fmt(value);
}

function panel({ title, subtitle, y, phase, measure, units, better, metric }) {
  const x = 56;
  const width = 1168;
  const height = 500;
  const cardW = 532;
  const cardH = 176;
  const cardGapX = 48;
  const cardGapY = 34;
  const cardStartX = x + 28;
  const cardStartY = y + 82;
  const parts = [
    `<g>`,
    `<rect x="${x}" y="${y}" width="${width}" height="${height}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="14"/>`,
    text(x + 28, y + 34, title, "section", 'dominant-baseline="middle"'),
    text(x + width - 28, y + 34, subtitle, "axis", 'text-anchor="end" dominant-baseline="middle"'),
    text(x + width - 28, y + 58, better, "axis", 'text-anchor="end" dominant-baseline="middle"'),
  ];

  for (const [payloadIndex, payload] of payloads.entries()) {
    const col = payloadIndex % 2;
    const rowIndex = Math.floor(payloadIndex / 2);
    const cardX = cardStartX + col * (cardW + cardGapX);
    const cardY = cardStartY + rowIndex * (cardH + cardGapY);
    const cardValues = algorithms.map((algorithm) => measure(row(phase, payload, algorithm)));
    const cardMax = niceMax(Math.max(...cardValues), metric);
    const chartX = cardX + 96;
    const chartY = cardY + 46;
    const chartW = cardW - 150;
    const barH = 12;
    const barGap = 8;
    const axisY = cardY + cardH - 12;

    parts.push(`<rect x="${cardX}" y="${cardY}" width="${cardW}" height="${cardH}" fill="#F8FAFC" stroke="#E2E8F0" stroke-width="1" rx="12"/>`);
    parts.push(text(cardX + 18, cardY + 24, payloadTitle(payload), "payload", 'dominant-baseline="middle"'));
    for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
      const tx = chartX + chartW * tick;
      parts.push(`<line x1="${tx.toFixed(1)}" y1="${chartY - 10}" x2="${tx.toFixed(1)}" y2="${axisY - 22}" stroke="#E2E8F0"/>`);
      parts.push(text(tx.toFixed(1), axisY, fmtTick(cardMax * tick, metric), "axis", 'text-anchor="middle"'));
    }
    parts.push(text(chartX + chartW, cardY + 24, `${units} axis max ${fmtTick(cardMax, metric)}`, "axis", 'text-anchor="end" dominant-baseline="middle"'));

    for (const [algorithmIndex, algorithm] of algorithms.entries()) {
      const data = row(phase, payload, algorithm);
      const value = measure(data);
      const barY = chartY + algorithmIndex * (barH + barGap);
      const barW = Math.max(2, (value / cardMax) * chartW);
      const color = colors[algorithm];

      parts.push(text(cardX + 18, barY + barH / 2, labels[algorithm], "bar-label", 'dominant-baseline="middle"'));
      parts.push(`<rect x="${chartX}" y="${barY.toFixed(1)}" width="${barW.toFixed(1)}" height="${barH}" fill="${color.fill}" stroke="${color.stroke}" stroke-width="1" rx="4"/>`);
    }
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="1760" viewBox="0 0 1280 1760">
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
      .payload { font-family: 'Architects Daughter'; font-size: 20px; fill: #1E293B; font-weight: bold; }
      .bar-label { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; font-weight: bold; }
      .axis { font-family: 'Comic Mono'; font-size: 10.5px; fill: #64748B; }
      .legend { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; }
      .note { font-family: 'Comic Mono'; font-size: 12px; fill: #475569; }
      .callout-title { font-family: 'Architects Daughter'; font-size: 20px; fill: #1E293B; font-weight: bold; }
      .callout { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; }
    </style>
  </defs>

  <rect x="0" y="0" width="1280" height="1760" fill="#F8FAFC" stroke="#E2E8F0" stroke-width="1.5" rx="18"/>
  <text x="640" y="42" text-anchor="middle" dominant-baseline="middle" class="title">Compression Benchmark Bar Charts</text>
  <text x="640" y="74" text-anchor="middle" dominant-baseline="middle" class="subtitle">Issue #195 · large payload rows · Apple M4 Pro local snapshot · bar length is the comparison signal</text>

  ${panel({
    title: "Compression throughput",
    subtitle: "Each payload card uses its own x-axis so bar length stays readable",
    y: 112,
    phase: "Compress",
    units: "MB/s",
    metric: "throughput",
    better: "Higher throughput is better",
    measure: (data) => data.mbPerSec,
  })}

  ${panel({
    title: "Decompression throughput",
    subtitle: "Each payload card uses its own x-axis so bar length stays readable",
    y: 636,
    phase: "Decompress",
    units: "MB/s",
    metric: "throughput",
    better: "Higher throughput is better",
    measure: (data) => data.mbPerSec,
  })}

  ${panel({
    title: "Compression density",
    subtitle: "Shared ratio axis · bar length is compressed/original size",
    y: 1160,
    phase: "Compress",
    units: "ratio",
    metric: "ratio",
    better: "Shorter bars are better; random payloads stay near 1.0",
    measure: (data) => Math.min(data.ratio, 1),
  })}

  <g>
    <rect x="56" y="1688" width="1168" height="44" fill="#FFFBEB" stroke="#F59E0B" stroke-width="1.4" rx="12"/>
    <text x="84" y="1710" dominant-baseline="middle" class="callout-title">Interpretation boundary</text>
    <text x="330" y="1709" dominant-baseline="middle" class="callout">One local same-condition run. Use the chart to scan patterns; keep raw output and tables as the numeric source of truth.</text>
  </g>

  <g>
    ${algorithms
      .map((algorithm, index) => {
        const x = 80 + index * 134;
        return `<rect x="${x}" y="1744" width="12" height="12" fill="${colors[algorithm].fill}" stroke="${colors[algorithm].stroke}" rx="2"/>
    <text x="${x + 20}" y="1755" class="legend">${labels[algorithm]}</text>`;
      })
      .join("\n    ")}
    <text x="914" y="1755" class="note">Density panel intentionally rewards shorter bars.</text>
  </g>
</svg>
`;

mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, svg);
console.log(`wrote ${outputPath}`);
console.log(
  `chart gate: rows=${rows.length} panels=3 smallMultiples=12 bars=72 payloads=${payloads.length} algorithms=${algorithms.length}`,
);
