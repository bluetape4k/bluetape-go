#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const inputPath = join(
  repoRoot,
  "docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt",
);
const svgPath = join(
  repoRoot,
  "docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg",
);
const pngPath = join(
  repoRoot,
  "docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png",
);
const vegaPath = join(
  repoRoot,
  "docs/images/readme-charts/money-fastmoney-evaluation-benchmark.vl.json",
);

const labels = new Map([
  ["NewMinorUSD", "NewMinor USD"],
  ["NewMinorJPY", "NewMinor JPY"],
  ["MinorUnitsUSD", "MinorUnits USD"],
  ["AddUSD", "Add USD"],
  ["SumUSD10", "Sum USD x10"],
  ["ParseUSD", "Parse USD text"],
  ["MarshalJSON", "Marshal JSON"],
  ["DirectGovaluesNewAmountFromMinorUnits", "Direct govalues minor"],
]);

const raw = readFileSync(inputPath, "utf8");
const rows = parseBenchmarks(raw);

if (rows.length !== labels.size) {
  throw new Error(`expected ${labels.size} benchmark rows, got ${rows.length}`);
}

const metadata = {
  commit: raw.match(/^git_commit:\s+(.+)$/m)?.[1] ?? "unknown",
  goVersion: raw.match(/^go_version:\s+(.+)$/m)?.[1] ?? "unknown Go version",
  goos: raw.match(/^goos:\s+(.+)$/m)?.[1] ?? "unknown",
  goarch: raw.match(/^goarch:\s+(.+)$/m)?.[1] ?? "unknown",
  cpu: raw.match(/^cpu:\s+(.+)$/m)?.[1] ?? "unknown CPU",
  command: raw.match(/^command:\s+(.+)$/m)?.[1] ?? "unknown command",
};

const width = 1800;
const height = 1630;
const frame = { x: 40, y: 40, width: width - 80, height: height - 80 };
const panelX = 80;
const panelW = width - 160;
const panelH = 410;
const panelYs = [180, 620, 1060];
const bandY = 1510;
const bandH = 72;

function parseBenchmarks(textContent) {
  const benchmarkPattern =
    /^BenchmarkMoney([A-Za-z0-9]+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+B\/op\s+([\d.]+)\s+allocs\/op$/;

  return textContent
    .split(/\r?\n/)
    .map((line) => benchmarkPattern.exec(line.trim()))
    .filter(Boolean)
    .map((match) => {
      const id = match[1];
      const label = labels.get(id);
      if (!label) {
        throw new Error(`unknown benchmark row ${id}`);
      }
      return {
        id,
        label,
        group: id.startsWith("DirectGovalues") ? "Reference" : "Money",
        nsPerOp: Number(match[2]),
        bytesPerOp: Number(match[3]),
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

function fmt(value) {
  return new Intl.NumberFormat("en-US", {
    maximumFractionDigits: value < 10 && value !== 0 ? 3 : value < 100 ? 2 : 1,
  }).format(value);
}

function niceMax(value) {
  if (value <= 1) return 1;
  if (value <= 10) return 10;
  if (value <= 25) return 25;
  if (value <= 50) return 50;
  if (value <= 100) return 100;
  if (value <= 250) return 250;
  if (value <= 500) return 500;
  return Math.ceil(value / 250) * 250;
}

function text(x, y, content, cls, attrs = "") {
  return `<text x="${x}" y="${y}" class="${cls}" ${attrs}>${esc(content)}</text>`;
}

function color(row) {
  if (row.group === "Reference") {
    return { fill: "#FDE68A", stroke: "#D97706" };
  }
  return { fill: "#BFDBFE", stroke: "#2563EB" };
}

function panel({ y, title, metric, units }) {
  const rowStartY = y + 90;
  const rowH = 25;
  const rowGap = 12;
  const labelX = panelX + 36;
  const chartX = panelX + 370;
  const chartW = panelW - 560;
  const valueX = chartX + chartW + 28;
  const max = niceMax(Math.max(...rows.map((row) => row[metric])));
  const gridBottom = rowStartY + rows.length * (rowH + rowGap) - rowGap + 10;
  const parts = [
    `<g>`,
    `<rect x="${panelX}" y="${y}" width="${panelW}" height="${panelH}" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="1.5" rx="16"/>`,
    text(panelX + 34, y + 36, title, "section", 'dominant-baseline="middle"'),
    text(
      panelX + 34,
      y + 66,
      `${units} - lower is better`,
      "axis-title",
      'dominant-baseline="middle"',
    ),
  ];

  for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
    const tx = chartX + chartW * tick;
    parts.push(
      `<line x1="${tx.toFixed(1)}" y1="${rowStartY - 20}" x2="${tx.toFixed(1)}" y2="${gridBottom}" stroke="#E2E8F0"/>`,
    );
    parts.push(
      text(
        tx.toFixed(1),
        rowStartY - 32,
        fmt(max * tick),
        "axis",
        'text-anchor="middle"',
      ),
    );
  }

  for (const [index, row] of rows.entries()) {
    const rowY = rowStartY + index * (rowH + rowGap);
    const barW = Math.max(4, (row[metric] / max) * chartW);
    const c = color(row);
    parts.push(
      text(labelX, rowY + rowH / 2, row.label, "bar-label", 'dominant-baseline="middle"'),
    );
    parts.push(
      `<rect x="${chartX}" y="${rowY}" width="${barW.toFixed(1)}" height="${rowH}" fill="${c.fill}" stroke="${c.stroke}" stroke-width="1.2" rx="7"/>`,
    );
    parts.push(
      text(
        valueX,
        rowY + rowH / 2,
        `${fmt(row[metric])} ${units}`,
        "value",
        'dominant-baseline="middle"',
      ),
    );
  }

  parts.push(`</g>`);
  return parts.join("\n");
}

function legendItem(x, y, fill, stroke, label) {
  return [
    `<rect x="${x}" y="${y - 11}" width="24" height="16" fill="${fill}" stroke="${stroke}" stroke-width="1.1" rx="5"/>`,
    text(x + 34, y - 1, label, "legend", 'dominant-baseline="middle"'),
  ].join("\n");
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
      .bar-label { font-family: 'Architects Daughter'; font-size: 21px; fill: #1E293B; }
      .axis-title { font-family: 'Comic Mono'; font-size: 14px; fill: #475569; font-weight: bold; }
      .axis { font-family: 'Comic Mono'; font-size: 12px; fill: #64748B; }
      .value { font-family: 'Comic Mono'; font-size: 15px; fill: #334155; font-weight: bold; }
      .legend { font-family: 'Comic Mono'; font-size: 14px; fill: #334155; }
      .callout-title { font-family: 'Architects Daughter'; font-size: 23px; fill: #1E293B; font-weight: bold; }
      .callout { font-family: 'Comic Mono'; font-size: 13px; fill: #334155; }
      .footer { font-family: 'Comic Mono'; font-size: 12px; fill: #64748B; }
    </style>
  </defs>

  <rect x="0" y="0" width="${width}" height="${height}" fill="#F8FAFC"/>
  <rect x="${frame.x}" y="${frame.y}" width="${frame.width}" height="${frame.height}" fill="#F8FAFC" stroke="#CBD5E1" stroke-width="2" rx="24"/>
  <text x="${width / 2}" y="82" text-anchor="middle" dominant-baseline="middle" class="title">Money FastMoney Evaluation Benchmark</text>
  <text x="${width / 2}" y="122" text-anchor="middle" dominant-baseline="middle" class="subtitle">Issue #180 · ${esc(metadata.cpu)} · ${esc(metadata.goos)}/${esc(metadata.goarch)} · commit ${esc(metadata.commit)}</text>

  <g>
    ${legendItem(1180, 132, "#BFDBFE", "#2563EB", "Money public API")}
    ${legendItem(1418, 132, "#FDE68A", "#D97706", "Reference")}
  </g>

  ${panel({
    y: panelYs[0],
    title: "Latency",
    metric: "nsPerOp",
    units: "ns/op",
  })}

  ${panel({
    y: panelYs[1],
    title: "Heap bytes",
    metric: "bytesPerOp",
    units: "B/op",
  })}

  ${panel({
    y: panelYs[2],
    title: "Allocations",
    metric: "allocsPerOp",
    units: "allocs/op",
  })}

  <g>
    <rect x="${panelX}" y="${bandY}" width="${panelW}" height="${bandH}" fill="#FFFBEB" stroke="#F59E0B" stroke-width="1.5" rx="16"/>
    <text x="${panelX + 34}" y="${bandY + 28}" dominant-baseline="middle" class="callout-title">Interpretation boundary</text>
    <text x="${panelX + 320}" y="${bandY + 25}" dominant-baseline="middle" class="callout">One local benchmark snapshot; lower bars are better. Use raw benchmark output as the numeric source of truth.</text>
    <text x="${panelX + 320}" y="${bandY + 50}" dominant-baseline="middle" class="footer">${esc(metadata.command)} · ${esc(metadata.goVersion)}</text>
  </g>
</svg>
`;

const vegaLite = {
  $schema: "https://vega.github.io/schema/vega-lite/v5.json",
  title: "Money FastMoney Evaluation Benchmark",
  source: "docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt",
  metadata,
  rows,
  vconcat: [
    chartSpec("nsPerOp", "Latency (ns/op) - lower is better"),
    chartSpec("bytesPerOp", "Heap bytes (B/op) - lower is better"),
    chartSpec("allocsPerOp", "Allocations (allocs/op) - lower is better"),
  ],
};

function chartSpec(field, title) {
  return {
    width: 800,
    height: 240,
    mark: { type: "bar", cornerRadiusEnd: 5 },
    encoding: {
      y: { field: "label", type: "nominal", sort: null, title: null },
      x: { field, type: "quantitative", title },
      color: {
        field: "group",
        type: "nominal",
        scale: { range: ["#2563EB", "#D97706"] },
      },
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

const render = spawnSync("rsvg-convert", ["--output", pngPath, svgPath], {
  encoding: "utf8",
});
if (render.status !== 0) {
  throw new Error(`rsvg-convert failed: ${render.stderr || render.stdout}`);
}

console.log(`wrote ${svgPath}`);
console.log(`wrote ${pngPath}`);
console.log(`wrote ${vegaPath}`);
console.log(
  `chart gate: rows=${rows.length} panels=3 bars=${rows.length * 3} ` +
    `canvas=${width}x${height} bandClearance=${height - bandY - bandH}`,
);
