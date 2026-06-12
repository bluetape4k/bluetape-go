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
  "docs/images/readme-charts/compression-large-payload-benchmark-matrix.svg",
);

const payloads = ["json", "text", "binary", "random"];
const algorithms = ["gzip", "zlib", "deflate", "zstd", "lz4", "snappy"];
const algorithmLabels = {
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

function fmtNumber(value, digits = 0) {
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

function mixWithWhite(hex, amount) {
  const r = Number.parseInt(hex.slice(1, 3), 16);
  const g = Number.parseInt(hex.slice(3, 5), 16);
  const b = Number.parseInt(hex.slice(5, 7), 16);
  const mixed = [r, g, b].map((channel) =>
    Math.round(255 - (255 - channel) * amount)
      .toString(16)
      .padStart(2, "0"),
  );
  return `#${mixed.join("")}`;
}

function heatmapPanel({ title, subtitle, x, y, width, height, values, better }) {
  const labelW = 112;
  const chartX = x + labelW + 28;
  const chartY = y + 88;
  const cellGap = 10;
  const rowGap = 14;
  const cellW = (width - labelW - 72 - cellGap * (algorithms.length - 1)) / algorithms.length;
  const cellH = (height - 138 - rowGap * (payloads.length - 1)) / payloads.length;
  const rawValues = payloads.flatMap((payload) =>
    algorithms.map((algorithm) => values(row(values.phase, payload, algorithm)).score),
  );
  const min = Math.min(...rawValues);
  const max = Math.max(...rawValues);
  const parts = [
    `<g>`,
    `<rect x="${x}" y="${y}" width="${width}" height="${height}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="14"/>`,
    text(x + 28, y + 33, title, "section", 'dominant-baseline="middle"'),
    text(x + width - 28, y + 33, subtitle, "axis", 'text-anchor="end" dominant-baseline="middle"'),
  ];

  for (const [algorithmIndex, algorithm] of algorithms.entries()) {
    const cx = chartX + algorithmIndex * (cellW + cellGap) + cellW / 2;
    parts.push(text(cx.toFixed(1), y + 66, algorithmLabels[algorithm], "column-label", 'text-anchor="middle" dominant-baseline="middle"'));
  }

  for (const [payloadIndex, payload] of payloads.entries()) {
    const cy = chartY + payloadIndex * (cellH + rowGap);
    parts.push(text(x + 30, cy + cellH / 2, payload.toUpperCase(), "label", 'dominant-baseline="middle"'));
    for (const [algorithmIndex, algorithm] of algorithms.entries()) {
      const data = row(values.phase, payload, algorithm);
      const value = values(data);
      const normalized = max === min ? 1 : (value.score - min) / (max - min);
      const strength = better === "higher" ? normalized : 1 - normalized;
      const fill = mixWithWhite(colors[algorithm].fill, 0.2 + strength * 0.72);
      const stroke = colors[algorithm].stroke;
      const cx = chartX + algorithmIndex * (cellW + cellGap);
      const border = strength > 0.98 ? 2.4 : 1.1;
      parts.push(`<rect x="${cx.toFixed(1)}" y="${cy.toFixed(1)}" width="${cellW.toFixed(1)}" height="${cellH.toFixed(1)}" fill="${fill}" stroke="${stroke}" stroke-width="${border}" rx="9"/>`);
      parts.push(text((cx + cellW / 2).toFixed(1), (cy + cellH / 2 - 8).toFixed(1), value.primary, "cell-value", 'text-anchor="middle" dominant-baseline="middle"'));
      parts.push(text((cx + cellW / 2).toFixed(1), (cy + cellH / 2 + 12).toFixed(1), value.secondary, "cell-note", 'text-anchor="middle" dominant-baseline="middle"'));
    }
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="1180" viewBox="0 0 1280 1180">
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
      .title { font-family: 'Architects Daughter'; font-size: 34px; fill: #1E293B; font-weight: bold; }
      .subtitle { font-family: 'Comic Mono'; font-size: 14px; fill: #64748B; }
      .section { font-family: 'Architects Daughter'; font-size: 23px; fill: #1E293B; font-weight: bold; }
      .label { font-family: 'Comic Mono'; font-size: 13px; fill: #334155; font-weight: bold; }
      .axis { font-family: 'Comic Mono'; font-size: 11px; fill: #64748B; }
      .legend { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; }
      .note { font-family: 'Comic Mono'; font-size: 12px; fill: #475569; }
      .column-label { font-family: 'Comic Mono'; font-size: 13px; fill: #334155; font-weight: bold; }
      .cell-value { font-family: 'Comic Mono'; font-size: 15px; fill: #0F172A; font-weight: bold; }
      .cell-note { font-family: 'Comic Mono'; font-size: 10.5px; fill: #334155; }
      .callout-title { font-family: 'Architects Daughter'; font-size: 20px; fill: #1E293B; font-weight: bold; }
      .callout { font-family: 'Comic Mono'; font-size: 12px; fill: #334155; }
      .callout-strong { font-family: 'Comic Mono'; font-size: 12px; fill: #0F172A; font-weight: bold; }
    </style>
  </defs>

  <rect x="0" y="0" width="1280" height="1180" fill="#F8FAFC" stroke="#E2E8F0" stroke-width="1.5" rx="18"/>
  <text x="640" y="42" text-anchor="middle" dominant-baseline="middle" class="title">Compression Benchmark Matrix</text>
  <text x="640" y="72" text-anchor="middle" dominant-baseline="middle" class="subtitle">Issue #195 · large payload rows · Apple M4 Pro local snapshot · tables remain the numeric source of truth</text>

  ${heatmapPanel({
    title: "Compression throughput",
    subtitle: "MB/s · higher is better · strongest cells have darker fill and thicker border",
    x: 56,
    y: 112,
    width: 1168,
    height: 298,
    better: "higher",
    values: Object.assign((data) => ({
      score: data.mbPerSec,
      primary: fmtNumber(data.mbPerSec),
      secondary: `${fmtNumber(data.nsPerOp / 1000, 0)} us/op`,
    }), { phase: "Compress" }),
  })}

  ${heatmapPanel({
    title: "Decompression throughput",
    subtitle: "MB/s · higher is better · strongest cells have darker fill and thicker border",
    x: 56,
    y: 442,
    width: 1168,
    height: 298,
    better: "higher",
    values: Object.assign((data) => ({
      score: data.mbPerSec,
      primary: fmtNumber(data.mbPerSec),
      secondary: `${fmtNumber(data.nsPerOp / 1000, 0)} us/op`,
    }), { phase: "Decompress" }),
  })}

  ${heatmapPanel({
    title: "Compression density",
    subtitle: "compressed/original · lower is better · strongest cells have darker fill and thicker border",
    x: 56,
    y: 772,
    width: 1168,
    height: 298,
    better: "lower",
    values: Object.assign((data) => ({
      score: data.ratio,
      primary: fmtRatio(data.ratio),
      secondary: `${fmtNumber(data.compressedBytes / 1024, 1)} KiB`,
    }), { phase: "Compress" }),
  })}

  <g>
    <rect x="56" y="1092" width="1168" height="52" fill="#FFFBEB" stroke="#F59E0B" stroke-width="1.4" rx="12"/>
    <text x="84" y="1119" dominant-baseline="middle" class="callout-title">Interpretation boundary</text>
    <text x="330" y="1118" dominant-baseline="middle" class="callout">This is one local same-condition run. Use it for scan-friendly comparison, not as a production default recommendation.</text>
  </g>

  <g>
    ${algorithms
      .map((algorithm, index) => {
        const x = 78 + index * 132;
        return `<rect x="${x}" y="1158" width="12" height="12" fill="${colors[algorithm].fill}" stroke="${colors[algorithm].stroke}" rx="2"/>
    <text x="${x + 20}" y="1169" class="legend">${algorithmLabels[algorithm]}</text>`;
      })
      .join("\n    ")}
    <text x="910" y="1169" class="note">Random rows show incompressible-data overhead near 1.0 ratio.</text>
  </g>
</svg>
`;

mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, svg);
console.log(`wrote ${outputPath}`);
console.log(
  `chart gate: rows=${rows.length} panels=3 algorithms=${algorithms.length} payloads=${payloads.length} margins=L/R/T/B 56/56/112/36`,
);
