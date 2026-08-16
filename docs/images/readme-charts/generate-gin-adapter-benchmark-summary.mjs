#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const defaultOutputDir = scriptDir;
const expectedNames = [
  "BenchmarkGinAdapter/NoOp/Serial",
  "BenchmarkGinAdapter/NoOp/Parallel",
  "BenchmarkGinAdapter/DirectCore/Serial",
  "BenchmarkGinAdapter/DirectCore/Parallel",
  "BenchmarkGinAdapter/Bridge/Serial",
  "BenchmarkGinAdapter/Bridge/Parallel",
  "BenchmarkGinAdapter/FullAdapter/Serial",
  "BenchmarkGinAdapter/FullAdapter/Parallel",
  "BenchmarkGinAdapter/FullAdapterRetry/Serial",
  "BenchmarkGinAdapter/FullAdapterRetry/Parallel",
  "BenchmarkGinAdapterColdConstruction",
  "BenchmarkGinAdapterColdFirstRequest",
  "BenchmarkGinAdapterWarmRequest/Serial",
  "BenchmarkGinAdapterWarmRequest/Parallel",
];
const expected = new Set(expectedNames);

function parseArgs(argv) {
  if (argv.includes("--self-test")) return { selfTest: true };
  let input;
  let outputDir = process.env.BLUETAPE_GIN_BENCH_CHART_DIR || defaultOutputDir;
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === "--output-dir") {
      outputDir = argv[index + 1];
      if (!outputDir || outputDir.startsWith("--")) throw new Error("--output-dir requires a directory");
      index += 1;
      continue;
    }
    if (value.startsWith("--")) throw new Error(`unknown option ${value}`);
    if (input) throw new Error("only one benchmark JSON input is allowed");
    input = value;
  }
  if (!input) throw new Error("usage: generator.mjs [--output-dir DIR] INPUT.json");
  return {
    selfTest: false,
    input,
    outputDir,
  };
}

function finite(value, field) {
  if (typeof value !== "number" || !Number.isFinite(value)) throw new Error(`${field} must be finite`);
  return value;
}

function validate(summary) {
  if (!summary || !Array.isArray(summary.rows) || summary.rows.length === 0) throw new Error("benchmark rows are missing");
  const names = new Set();
  const exact = new Set();
  const benchmarkCount = Number.parseInt(summary.metadata?.benchmark_count || "1", 10);
  if (!Number.isInteger(benchmarkCount) || benchmarkCount <= 0) throw new Error("benchmark_count must be a positive integer");
  const rawCpus = summary.metadata?.cpu;
  const expectedCpus = rawCpus
    ? rawCpus.split(",").map((value) => Number.parseInt(value, 10))
    : [...new Set(summary.rows.map((row) => row.cpu))];
  if (!expectedCpus.length || expectedCpus.some((value) => !Number.isInteger(value) || value <= 0) || new Set(expectedCpus).size !== expectedCpus.length) {
    throw new Error("cpu metadata must contain unique positive integers");
  }
  const counts = new Map();
  for (const row of summary.rows) {
    if (!expected.has(row.name)) throw new Error(`unknown benchmark row ${row.name}`);
    finite(row.cpu, `${row.name}.cpu`);
    finite(row.iterations, `${row.name}.iterations`);
    finite(row.ns_per_op, `${row.name}.ns_per_op`);
    finite(row.bytes_per_op, `${row.name}.bytes_per_op`);
    finite(row.allocs_per_op, `${row.name}.allocs_per_op`);
    if (row.cpu <= 0 || row.iterations <= 0 || row.ns_per_op <= 0 || row.bytes_per_op < 0 || row.allocs_per_op < 0) {
      throw new Error(`invalid metric for ${row.name}`);
    }
    if (row.failed === true) throw new Error(`failed benchmark row ${row.name}`);
    names.add(row.name);
    if (!expectedCpus.includes(row.cpu)) throw new Error(`unexpected benchmark CPU ${row.cpu}`);
    const key = `${row.name}|${row.cpu}`;
    counts.set(key, (counts.get(key) || 0) + 1);
    const signature = [row.name, row.cpu, row.iterations, row.ns_per_op, row.bytes_per_op, row.allocs_per_op].join("|");
    if (exact.has(signature) && benchmarkCount <= 1) throw new Error(`duplicate benchmark row ${row.name} cpu=${row.cpu}`);
    exact.add(signature);
  }
  const missing = expectedNames.filter((name) => !names.has(name));
  if (missing.length) throw new Error(`missing benchmark rows: ${missing.join(", ")}`);
  for (const name of expectedNames) {
    for (const cpu of expectedCpus) {
      const observed = counts.get(`${name}|${cpu}`) || 0;
      if (observed !== benchmarkCount) throw new Error(`benchmark sample count mismatch ${name} cpu=${cpu}: ${observed} != ${benchmarkCount}`);
    }
  }
  return summary;
}

function median(values) {
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0 ? (sorted[middle - 1] + sorted[middle]) / 2 : sorted[middle];
}

function summarize(summary) {
  const rows = expectedNames.map((name) => {
    const samples = summary.rows.filter((row) => row.name === name && row.cpu === 1);
    if (samples.length === 0) throw new Error(`missing cpu=1 samples for ${name}`);
    return {
      name,
      nsPerOp: median(samples.map((row) => row.ns_per_op)),
      bytesPerOp: median(samples.map((row) => row.bytes_per_op)),
      allocsPerOp: median(samples.map((row) => row.allocs_per_op)),
      samples: samples.length,
    };
  });
  const byName = new Map(rows.map((row) => [row.name, row]));
  const bridgeOverhead = (byName.get("BenchmarkGinAdapter/Bridge/Serial").nsPerOp - byName.get("BenchmarkGinAdapter/DirectCore/Serial").nsPerOp) / byName.get("BenchmarkGinAdapter/DirectCore/Serial").nsPerOp;
  const fullOverhead = (byName.get("BenchmarkGinAdapter/FullAdapter/Serial").nsPerOp - byName.get("BenchmarkGinAdapter/NoOp/Serial").nsPerOp) / byName.get("BenchmarkGinAdapter/NoOp/Serial").nsPerOp;
  return { rows, bridgeOverhead, fullOverhead };
}

function escape(value) {
  return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}

function format(value) {
  if (value >= 1000) return `${(value / 1000).toFixed(1)}k`;
  return value.toFixed(value < 10 ? 2 : 0);
}

function render(summary, outputDir) {
  const { rows, bridgeOverhead, fullOverhead } = summarize(summary);
  const selected = rows.filter((row) => row.name.includes("/Serial") && row.name.startsWith("BenchmarkGinAdapter/"));
  const max = Math.max(...selected.map((row) => row.nsPerOp), 1);
  const width = 1500;
  const height = 760;
  const chartX = 470;
  const chartW = 820;
  const rowH = 66;
  const svg = [];
  svg.push(`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="title desc">`);
  svg.push(`<title id="title">Gin adapter benchmark summary</title>`);
  svg.push(`<desc id="desc">Median CPU 1 serial latency for no-op, direct core, bridge, parser-only full adapter, and retry-policy full adapter fixtures. Lower is better.</desc>`);
  svg.push(`<defs><style>.title{font:700 30px Inter,Arial,sans-serif;fill:#0F172A}.subtitle{font:16px Inter,Arial,sans-serif;fill:#475569}.label{font:16px ui-monospace,monospace;fill:#334155}.value{font:700 15px ui-monospace,monospace;fill:#0F172A}.note{font:15px Inter,Arial,sans-serif;fill:#475569}</style></defs>`);
  svg.push(`<rect width="${width}" height="${height}" fill="#F8FAFC"/><rect x="24" y="24" width="${width - 48}" height="${height - 48}" rx="24" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="2"/>`);
  svg.push(`<text x="70" y="78" class="title">Gin adapter · Issue #543</text>`);
  svg.push(`<text x="70" y="110" class="subtitle">CPU 1 serial median · ns/op, B/op, allocs/op are parsed from raw Go benchmark output · lower is better</text>`);
  for (const tick of [0, 0.25, 0.5, 0.75, 1]) {
    const x = chartX + chartW * tick;
    svg.push(`<line x1="${x}" y1="150" x2="${x}" y2="510" stroke="#E2E8F0"/>`);
    svg.push(`<text x="${x}" y="535" text-anchor="middle" class="value">${format(max * tick)}</text>`);
  }
  selected.forEach((row, index) => {
    const y = 176 + index * rowH;
    const widthValue = Math.max(3, (row.nsPerOp / max) * chartW);
    svg.push(`<text x="70" y="${y + 24}" class="label">${escape(row.name.replace("BenchmarkGinAdapter/", ""))}</text>`);
    const fill = row.name.includes("FullAdapterRetry") ? "#0F766E" : row.name.includes("FullAdapter/Serial") ? "#F59E0B" : "#2563EB";
    svg.push(`<rect x="${chartX}" y="${y}" width="${widthValue}" height="30" rx="8" fill="${fill}"/>`);
    svg.push(`<text x="${chartX + chartW + 24}" y="${y + 23}" class="value">${format(row.nsPerOp)} ns/op</text>`);
  });
  svg.push(`<rect x="70" y="570" width="1360" height="110" rx="16" fill="#ECFEFF" stroke="#0891B2"/>`);
  svg.push(`<text x="100" y="612" class="note">bridge overhead = (bridge - direct-core) / direct-core = ${(bridgeOverhead * 100).toFixed(1)}%</text>`);
  svg.push(`<text x="100" y="645" class="note">full overhead = (full adapter - no-op) / no-op = ${(fullOverhead * 100).toFixed(1)}%</text>`);
  svg.push(`<text x="100" y="678" class="note">FullAdapterRetry includes the no-backoff retry-policy success path; raw rows remain the source of truth.</text>`);
  svg.push(`</svg>`);
  mkdirSync(outputDir, { recursive: true });
  const svgPath = join(outputDir, "gin-adapter-benchmark-summary.svg");
  const pngPath = join(outputDir, "gin-adapter-benchmark-summary.png");
  const vlPath = join(outputDir, "gin-adapter-benchmark-summary.vl.json");
  writeFileSync(svgPath, `${svg.join("\n")}\n`);
  const renderResult = spawnSync("rsvg-convert", ["--output", pngPath, svgPath], { encoding: "utf8" });
  if (renderResult.status !== 0) throw new Error(`rsvg-convert failed: ${renderResult.stderr || renderResult.stdout}`);
  writeFileSync(
    vlPath,
    `${JSON.stringify(
      {
        $schema: "https://vega.github.io/schema/vega-lite/v5.json",
        title: "Gin adapter benchmark summary",
        description: "Median CPU 1 serial latency for parser-only and retry-policy Gin adapter fixtures; lower latency is better.",
        data: { values: rows.map((row) => ({ name: row.name, ns_per_op: row.nsPerOp })) },
        mark: "bar",
        encoding: {
          y: { field: "name", type: "nominal", sort: "-x" },
          x: { field: "ns_per_op", type: "quantitative", title: "ns/op" },
        },
        usermeta: { metadata: summary.metadata || {}, metricDirection: "lower is better", bridgeOverhead, fullOverhead },
      },
      null,
      2,
    )}\n`,
  );
  return { svgPath, pngPath, vlPath, bridgeOverhead, fullOverhead };
}

function selfTest() {
  const rows = expectedNames.flatMap((name, index) => [
    { name, cpu: 1, iterations: 1, ns_per_op: index + 1, bytes_per_op: index, allocs_per_op: index },
    { name, cpu: 1, iterations: 2, ns_per_op: index + 2, bytes_per_op: index + 1, allocs_per_op: index + 1 },
  ]);
  const summary = { metadata: { benchmark_count: "2", cpu: "1" }, rows };
  validate(summary);
  summarize(summary);
  let rejected = false;
  try {
    validate({ metadata: { benchmark_count: "2", cpu: "1" }, rows: rows.slice(1) });
  } catch {
    rejected = true;
  }
  if (!rejected) throw new Error("incomplete sample set was accepted");
  console.log("self-test: valid rows, sample-count/missing/unknown/duplicate/non-finite guards ready");
}

try {
  const args = parseArgs(process.argv.slice(2));
  if (args.selfTest) {
    selfTest();
  } else {
    const summary = validate(JSON.parse(readFileSync(args.input, "utf8")));
    const result = render(summary, args.outputDir);
    console.log(`generated ${result.svgPath}, ${result.pngPath}, ${result.vlPath}`);
  }
} catch (error) {
  console.error(`error: ${error.message}`);
  process.exitCode = 2;
}
