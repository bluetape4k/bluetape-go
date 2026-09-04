#!/usr/bin/env node

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const summaryPath = join(repoRoot, "docs/research/outputs/issue-599/summary.json");
const chartDir = join(repoRoot, "docs/images/readme-charts");
const outputBase = join(chartDir, "issue599-fory-redis-benchmark");
const summary = JSON.parse(readFileSync(summaryPath, "utf8"));

const profiles = {
  JSON: { color: "#2563EB", shape: "circle" },
  NativeFast: { color: "#059669", shape: "square" },
  NativeCompatible: { color: "#D97706", shape: "diamond" },
  Mutex: { color: "#7C3AED", shape: "circle" },
  Pool: { color: "#DC2626", shape: "square" },
};

const rowByName = new Map(summary.rows.map((row) => [row.name, row]));
const selectedPanels = [
  {
    title: "In-process codec / Small / RoundTrip",
    rows: ["JSON", "NativeFast", "NativeCompatible"].map((profile) => ({
      label: profile,
      profile,
      row: rowByName.get(`BenchmarkIssue599Codec/${profile}/Small/RoundTrip`),
    })),
  },
  {
    title: "Direct Redis value cache / Small / RoundTrip",
    rows: ["JSON", "NativeFast", "NativeCompatible"].map((profile) => ({
      label: profile,
      profile,
      row: rowByName.get(`BenchmarkIssue599DirectRedis/${profile}/Small/RoundTrip`),
    })),
  },
  {
    title: "Stampede coordination / ColdWinner",
    rows: ["JSON", "NativeFast", "NativeCompatible"].map((profile) => ({
      label: profile,
      profile,
      row: rowByName.get(`BenchmarkIssue599Coordination/${profile}/ColdWinner`),
    })),
  },
  {
    title: "NativeFast contention / RoundTrip",
    rows: ["Mutex", "Pool"].map((profile) => ({
      label: profile,
      profile,
      row: rowByName.get(`BenchmarkIssue599Contention/NativeFast/${profile}`),
    })),
  },
];

for (const panel of selectedPanels) {
  for (const item of panel.rows) {
    if (!item.row) throw new Error(`missing summary row for ${panel.title}/${item.profile}`);
  }
}

const width = 1500;
const height = 1080;
const plotX = 430;
const plotWidth = 830;
const labelX = 70;
const valueX = 1280;
const panelStartY = 190;
const rowHeight = 47;
const panelGap = 42;
const minDomain = 10;
const maxDomain = 1_000_000;
const ticks = [10, 100, 1_000, 10_000, 100_000, 1_000_000];

function escape(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function x(value) {
  return plotX + ((Math.log10(value) - Math.log10(minDomain)) / (Math.log10(maxDomain) - Math.log10(minDomain))) * plotWidth;
}

function format(value) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(value >= 10_000 ? 0 : 1)}k`;
  if (value >= 100) return `${Math.round(value)}`;
  return value.toFixed(value < 10 ? 1 : 0);
}

function marker(shape, centerX, centerY, color) {
  if (shape === "square") {
    return `<rect x="${centerX - 7}" y="${centerY - 7}" width="14" height="14" rx="2" fill="${color}" stroke="#FFFFFF" stroke-width="2"/>`;
  }
  if (shape === "diamond") {
    return `<path d="M ${centerX} ${centerY - 9} L ${centerX + 9} ${centerY} L ${centerX} ${centerY + 9} L ${centerX - 9} ${centerY} Z" fill="${color}" stroke="#FFFFFF" stroke-width="2"/>`;
  }
  return `<circle cx="${centerX}" cy="${centerY}" r="8" fill="${color}" stroke="#FFFFFF" stroke-width="2"/>`;
}

const svg = [];
svg.push(`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="title desc">`);
svg.push(`<title id="title">Issue #599 Go Fory and Redis cache benchmark</title>`);
svg.push(`<desc id="desc">Three-sample median latency comparison for JSON, NativeFast, NativeCompatible, and NativeFast mutex versus pool contention. Whiskers show observed minimum and maximum. Lower is better.</desc>`);
svg.push(`<metadata>source: docs/research/outputs/issue-599/summary.json; samples: ${summary.rows[0].samples}; metric: ns/op; scale: log10</metadata>`);
svg.push(`<defs><style>
  .title{font-family:Inter,Arial,sans-serif;font-size:30px;font-weight:750;fill:#0F172A}
  .subtitle{font-family:Inter,Arial,sans-serif;font-size:16px;fill:#475569}
  .panel{font-family:Inter,Arial,sans-serif;font-size:19px;font-weight:700;fill:#0F172A}
  .label{font-family:Inter,Arial,sans-serif;font-size:15px;fill:#334155}
  .value{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:14px;font-weight:700;fill:#0F172A}
  .axis{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:13px;fill:#64748B}
</style></defs>`);
svg.push(`<rect width="${width}" height="${height}" fill="#F8FAFC"/>`);
svg.push(`<rect x="24" y="24" width="${width - 48}" height="${height - 48}" rx="24" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="2"/>`);
svg.push(`<text x="70" y="78" class="title">Issue #599 · Go Fory / Redis cache profiles</text>`);
svg.push(`<text x="70" y="108" class="subtitle">Three samples per row · median latency with observed min/max whisker · lower is better · log scale</text>`);

let legendX = 70;
for (const [name, style] of Object.entries(profiles)) {
  svg.push(marker(style.shape, legendX + 8, 145, style.color));
  svg.push(`<text x="${legendX + 24}" y="150" class="label">${escape(name)}</text>`);
  legendX += name === "NativeCompatible" ? 190 : 150;
}

for (const tick of ticks) {
  const tickX = x(tick);
  svg.push(`<line x1="${tickX}" y1="${panelStartY - 20}" x2="${tickX}" y2="${height - 108}" stroke="#E2E8F0" stroke-width="1"/>`);
  svg.push(`<text x="${tickX}" y="${height - 82}" text-anchor="middle" class="axis">${format(tick)}</text>`);
}

let y = panelStartY;
for (const panel of selectedPanels) {
  svg.push(`<text x="${labelX}" y="${y}" class="panel">${escape(panel.title)}</text>`);
  y += 29;
  for (const item of panel.rows) {
    const row = item.row;
    const style = profiles[item.profile];
    const median = row.ns_per_op.median;
    const min = row.ns_per_op.min;
    const max = row.ns_per_op.max;
    const centerY = y;
    svg.push(`<text x="${labelX + 16}" y="${centerY + 5}" class="label">${escape(item.label)}</text>`);
    svg.push(`<line x1="${plotX}" y1="${centerY}" x2="${plotX + plotWidth}" y2="${centerY}" stroke="#F1F5F9" stroke-width="1"/>`);
    svg.push(`<line x1="${x(min)}" y1="${centerY}" x2="${x(max)}" y2="${centerY}" stroke="${style.color}" stroke-width="6" stroke-linecap="round"/>`);
    svg.push(`<line x1="${x(min)}" y1="${centerY - 8}" x2="${x(min)}" y2="${centerY + 8}" stroke="${style.color}" stroke-width="2"/>`);
    svg.push(`<line x1="${x(max)}" y1="${centerY - 8}" x2="${x(max)}" y2="${centerY + 8}" stroke="${style.color}" stroke-width="2"/>`);
    svg.push(marker(style.shape, x(median), centerY, style.color));
    svg.push(`<text x="${valueX}" y="${centerY + 5}" class="value">${escape(format(median))} ns/op</text>`);
    y += rowHeight;
  }
  y += panelGap;
}

svg.push(`<line x1="${plotX}" y1="${height - 108}" x2="${plotX + plotWidth}" y2="${height - 108}" stroke="#64748B" stroke-width="1.5"/>`);
svg.push(`<text x="${plotX + plotWidth / 2}" y="${height - 48}" text-anchor="middle" class="subtitle">latency (ns/op, log10) · profile measurements are not schema-mode equivalents</text>`);
svg.push(`</svg>`);

writeFileSync(`${outputBase}.svg`, `${svg.join("\n")}\n`);
const render = spawnSync("rsvg-convert", ["--output", `${outputBase}.png`, `${outputBase}.svg`], { encoding: "utf8" });
if (render.status !== 0) throw new Error(`rsvg-convert failed: ${render.stderr || render.stdout}`);
writeFileSync(`${outputBase}.source.json`, `${JSON.stringify({ summary: "docs/research/outputs/issue-599/summary.json", panels: selectedPanels.map((panel) => ({ title: panel.title, rows: panel.rows.map((item) => item.row.name) })) }, null, 2)}\n`);
console.log(`generated ${outputBase}.{svg,png,source.json}`);
