#!/usr/bin/env node

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, "../../..");
const outputDir = join(repoRoot, "docs/research/outputs/issue-560");
const chartDir = join(repoRoot, "docs/images/readme-charts");
const expectedSHA = readFileSync(join(outputDir, "environment.md"), "utf8").match(/^- Git SHA: `([0-9a-f]{40})`$/m)?.[1];
if (!expectedSHA) throw new Error("environment manifest has no Git SHA");

const providers = {
  Redis: { color: "#2563EB", shape: "circle" },
  MongoDB: { color: "#D97706", shape: "square" },
  PostgreSQL: { color: "#7C3AED", shape: "diamond" },
  etcd: { color: "#059669", shape: "triangle" },
  Memory: { color: "#475569", shape: "circle" },
  RedisL2: { color: "#2563EB", shape: "square" },
  Tiered: { color: "#7C3AED", shape: "diamond" },
  NearCachePubSub: { color: "#059669", shape: "triangle" },
  CSV: { color: "#2563EB", shape: "circle" },
  NDJSON: { color: "#D97706", shape: "square" },
  GraphML: { color: "#7C3AED", shape: "diamond" },
  Neo4j: { color: "#2563EB", shape: "circle" },
  Memgraph: { color: "#059669", shape: "square" },
  PostgreSQLRecursiveCTE: { color: "#7C3AED", shape: "diamond" },
};

function section(label, rows, samples) {
  return { label, rows: new Set(rows), samples };
}

function product(prefix, groups) {
  return groups.flatMap(([left, rights]) => rights.map((right) => `${prefix}/${left}/${right}`));
}

const schemas = {
  "leader-local": [section("leader-local", ["BenchmarkProviderLeaderLocal/LocalHarness/CampaignContention/N=8"], 5)],
  "leader-containers": [
    section("ordinary", [
      ...product("BenchmarkProviderLeaderContainers", [
        ["Redis", ["CampaignUncontended", "ResignOwned", "CampaignContention/N=8", "LeaderLookup"]],
        ["MongoDB", ["CampaignUncontended", "ResignOwned", "CampaignContention/N=8", "LeaderLookup"]],
        ["PostgreSQL", ["CampaignUncontended", "ResignOwned", "CampaignContention/N=8", "LeaderLookup"]],
        ["etcd", ["CampaignUncontended", "ResignOwned", "LeaderLookup"]],
      ]),
    ], 3),
    section("expiry", product("BenchmarkProviderLeaderContainers", [
      ["Redis", ["ExpiryTakeover"]], ["MongoDB", ["ExpiryTakeover"]],
      ["PostgreSQL", ["ExpiryTakeover"]], ["etcd", ["ExpiryTakeover"]],
    ]), 3),
  ],
  "leader-probes": [section("leader-probes", [], 0)],
  "ratelimit-local": [section("ratelimit-local", [
    "BenchmarkProviderRateLimitLocal/Local/TokenBucket/AllowAvailable",
    "BenchmarkProviderRateLimitLocal/Local/TokenBucket/AllowRejected",
  ], 5)],
  "ratelimit-containers": [section("ratelimit-containers", product("BenchmarkProviderRateLimitContainers", [
    ["Redis", ["AllowAvailable", "AllowRejected", "AllowParallel/N=8", "AllowDistinctKeys/N=8"]],
    ["PostgreSQL", ["AllowAvailable", "AllowRejected", "AllowParallel/N=8", "AllowDistinctKeys/N=8"]],
  ]), 3)],
  "cache-local": [section("cache-local", product("BenchmarkProviderCacheLocal", [
    ["Memory", ["GetHit/128B", "GetHit/4KiB", "GetMiss/128B", "GetMiss/4KiB", "GetOrLoadHot/128B", "GetOrLoadHot/4KiB", "Set/128B", "Set/4KiB"]],
    ["SerializationBaseline", ["Marshal/128B", "Marshal/4KiB", "Unmarshal/128B", "Unmarshal/4KiB"]],
  ]), 5)],
  "cache-redis": [section("cache-redis", product("BenchmarkProviderCacheRedis", [
    ["RedisL2", ["GetHit/128B", "GetHit/4KiB", "GetMiss/128B", "GetMiss/4KiB", "Set/128B", "Set/4KiB", "Delete/128B", "Delete/4KiB"]],
    ["Tiered", ["L1Hit/128B", "L1Hit/4KiB", "L2Hit/128B", "L2Hit/4KiB", "LoadMiss/128B", "LoadMiss/4KiB", "WriteThrough/128B", "WriteThrough/4KiB"]],
    ["NearCachePubSub", ["LocalHit/128B", "LocalHit/4KiB", "LocalMiss/128B", "LocalMiss/4KiB", "PeerInvalidation/128B", "PeerInvalidation/4KiB", "PublishDelete/128B", "PublishDelete/4KiB", "PublishSet/128B", "PublishSet/4KiB"]],
  ]), 3)],
  graphio: [section("graphio", product("BenchmarkGraphIOFormats", [
    ["CSV", ["Small/100V-200E-3P/Write", "Small/100V-200E-3P/Read", "Small/100V-200E-3P/RoundTrip", "Small/100V-200E-3P/RecordConstructionBaseline", "Medium/10000V-20000E-5P/Write", "Medium/10000V-20000E-5P/Read", "Medium/10000V-20000E-5P/RoundTrip", "Medium/10000V-20000E-5P/RecordConstructionBaseline", "WideProperties/1000V-2000E-20P/Write", "WideProperties/1000V-2000E-20P/Read", "WideProperties/1000V-2000E-20P/RoundTrip", "WideProperties/1000V-2000E-20P/RecordConstructionBaseline"]],
    ["NDJSON", ["Small/100V-200E-3P/Write", "Small/100V-200E-3P/Read", "Small/100V-200E-3P/RoundTrip", "Small/100V-200E-3P/RecordConstructionBaseline", "Medium/10000V-20000E-5P/Write", "Medium/10000V-20000E-5P/Read", "Medium/10000V-20000E-5P/RoundTrip", "Medium/10000V-20000E-5P/RecordConstructionBaseline", "WideProperties/1000V-2000E-20P/Write", "WideProperties/1000V-2000E-20P/Read", "WideProperties/1000V-2000E-20P/RoundTrip", "WideProperties/1000V-2000E-20P/RecordConstructionBaseline"]],
    ["GraphML", ["Small/100V-200E-3P/Write", "Small/100V-200E-3P/Read", "Small/100V-200E-3P/RoundTrip", "Small/100V-200E-3P/RecordConstructionBaseline", "Medium/10000V-20000E-5P/Write", "Medium/10000V-20000E-5P/Read", "Medium/10000V-20000E-5P/RoundTrip", "Medium/10000V-20000E-5P/RecordConstructionBaseline", "WideProperties/1000V-2000E-20P/Write", "WideProperties/1000V-2000E-20P/Read", "WideProperties/1000V-2000E-20P/RoundTrip", "WideProperties/1000V-2000E-20P/RecordConstructionBaseline"]],
  ]), 5)],
  graphdb: [section("graphdb", product("BenchmarkGraphProviderTraversalContainers", [
    ["Neo4j", ["LongChain/Depth16", "LongChain/Depth64", "DeepWide/Depth4Fanout4"]],
    ["Memgraph", ["LongChain/Depth16", "LongChain/Depth64", "DeepWide/Depth4Fanout4"]],
    ["PostgreSQLRecursiveCTE", ["LongChain/Depth16", "LongChain/Depth64", "DeepWide/Depth4Fanout4"]],
  ]), 3)],
};

function parseArtifact(raw, schema, source = "fixture") {
  const shaValues = [...raw.matchAll(/^git_sha: ([0-9a-f]{40})$/gm)].map((match) => match[1]);
  const labels = [...raw.matchAll(/^command_label: (.+)$/gm)].map((match) => match[1]);
  if (labels.length !== schema.length) throw new Error(`${source}: expected ${schema.length} command sections, got ${labels.length}`);
  if (shaValues.length !== schema.length || shaValues.some((sha) => sha !== expectedSHA)) throw new Error(`${source}: command sections do not use environment Git SHA`);

  const chunks = raw.split(/^command_label: /m).slice(1).map((chunk) => `command_label: ${chunk}`);
  const allRows = new Map();
  for (const [index, expected] of schema.entries()) {
    const chunk = chunks[index];
    const label = chunk.match(/^command_label: (.+)$/m)?.[1];
    if (label !== expected.label) throw new Error(`${source}: section ${index + 1} label ${label} != ${expected.label}`);
    const exits = [...chunk.matchAll(/^exit_status: (\d+)$/gm)].map((match) => Number(match[1]));
    if (exits.length !== 1 || exits[0] !== 0) throw new Error(`${source}/${label}: non-zero or missing exit status`);

    const rows = new Map();
    for (const line of chunk.split(/\r?\n/)) {
      if (!line.startsWith("Benchmark")) continue;
      const match = /^(Benchmark.+)-\d+\s+\d+\s+(\S+)\s+ns\/op(?:\s+(\S+)\s+MB\/s)?\s+(\S+)\s+B\/op\s+(\S+)\s+allocs\/op$/.exec(line.trim());
      if (!match) throw new Error(`${source}/${label}: malformed benchmark row ${line}`);
      const name = match[1];
      if (!expected.rows.has(name)) throw new Error(`${source}/${label}: unknown benchmark row ${name}`);
      const values = [match[2], match[4], match[5]].map(Number);
      if (match[3] !== undefined) values.push(Number(match[3]));
      if (values.some((value) => !Number.isFinite(value))) throw new Error(`${source}/${label}: non-finite benchmark row ${name}`);
      const sample = { nsPerOp: values[0], bytesPerOp: values[1], allocsPerOp: values[2], mbPerSec: match[3] === undefined ? null : Number(match[3]) };
      if (!rows.has(name)) rows.set(name, []);
      rows.get(name).push(sample);
    }
    if (rows.size !== expected.rows.size) throw new Error(`${source}/${label}: expected ${expected.rows.size} rows, got ${rows.size}`);
    for (const name of expected.rows) {
      const samples = rows.get(name);
      if (!samples) throw new Error(`${source}/${label}: missing benchmark row ${name}`);
      if (samples.length !== expected.samples) throw new Error(`${source}/${label}: duplicate or missing samples for ${name}; expected ${expected.samples}, got ${samples.length}`);
      if (allRows.has(name)) throw new Error(`${source}: benchmark row appears in multiple sections: ${name}`);
      allRows.set(name, samples);
    }
  }
  return allRows;
}

function load(name) {
  const raw = readFileSync(join(outputDir, `${name}.txt`), "utf8");
  return parseArtifact(raw, schemas[name], name);
}

function summary(samples) {
  const values = samples.map((sample) => sample.nsPerOp).sort((a, b) => a - b);
  return { min: values[0], median: values[Math.floor(values.length / 2)], max: values.at(-1), samples: values.length };
}

function selectedRow(rows, name, provider, scenario, panel) {
  const stats = summary(rows.get(name));
  return { provider, scenario, panel, ...stats };
}

function leaderRows() {
  const rows = load("leader-containers");
  const selected = [];
  for (const provider of ["Redis", "MongoDB", "PostgreSQL", "etcd"]) {
    selected.push(selectedRow(rows, `BenchmarkProviderLeaderContainers/${provider}/CampaignUncontended`, provider, "Campaign uncontended", "Ordinary operations"));
    selected.push(selectedRow(rows, `BenchmarkProviderLeaderContainers/${provider}/LeaderLookup`, provider, "Leader lookup", "Ordinary operations"));
    selected.push(selectedRow(rows, `BenchmarkProviderLeaderContainers/${provider}/ExpiryTakeover`, provider, "Expiry takeover", "Deadline-driven takeover"));
  }
  return selected;
}

function rateRows() {
  const rows = load("ratelimit-containers");
  return ["AllowAvailable", "AllowRejected", "AllowParallel/N=8", "AllowDistinctKeys/N=8"].flatMap((scenario) =>
    ["Redis", "PostgreSQL"].map((provider) => selectedRow(rows, `BenchmarkProviderRateLimitContainers/${provider}/${scenario}`, provider, scenario.replaceAll("/", " "), "Equivalent allow decisions")),
  );
}

function cacheRows() {
  const local = load("cache-local");
  const remote = load("cache-redis");
  return [
    selectedRow(local, "BenchmarkProviderCacheLocal/Memory/GetHit/128B", "Memory", "Get hit 128 B", "Hot read paths"),
    selectedRow(remote, "BenchmarkProviderCacheRedis/Tiered/L1Hit/128B", "Tiered", "L1 hit 128 B", "Hot read paths"),
    selectedRow(remote, "BenchmarkProviderCacheRedis/NearCachePubSub/LocalHit/128B", "NearCachePubSub", "Local hit 128 B", "Hot read paths"),
    selectedRow(remote, "BenchmarkProviderCacheRedis/RedisL2/GetHit/128B", "RedisL2", "Get hit 128 B", "Remote read paths"),
    selectedRow(remote, "BenchmarkProviderCacheRedis/Tiered/L2Hit/128B", "Tiered", "L2 hit 128 B", "Remote read paths"),
    selectedRow(remote, "BenchmarkProviderCacheRedis/NearCachePubSub/PeerInvalidation/128B", "NearCachePubSub", "Peer invalidation 128 B", "Invalidation observation"),
  ];
}

function graphIORows() {
  const rows = load("graphio");
  return ["CSV", "NDJSON", "GraphML"].map((provider) => selectedRow(rows, `BenchmarkGraphIOFormats/${provider}/Medium/10000V-20000E-5P/RoundTrip`, provider, "Medium round trip", "Equivalent format round trips"));
}

function graphDBRows() {
  const rows = load("graphdb");
  return ["LongChain/Depth16", "LongChain/Depth64", "DeepWide/Depth4Fanout4"].flatMap((scenario) =>
    ["Neo4j", "Memgraph", "PostgreSQLRecursiveCTE"].map((provider) => selectedRow(rows, `BenchmarkGraphProviderTraversalContainers/${provider}/${scenario}`, provider, scenario.replaceAll("/", " "), "Equivalent seeded traversals")),
  );
}

const charts = {
  leader: { title: "Leader provider latency", subtitle: "min - median - max ns/op; lower is better; expiry isolated; logarithmic axis", rows: leaderRows, scale: "log" },
  ratelimit: { title: "Rate-limit provider latency", subtitle: "min - median - max ns/op; lower is better; logarithmic axis", rows: rateRows, scale: "log" },
  cache: { title: "Cache path latency", subtitle: "min - median - max ns/op; lower is better; logarithmic axis", rows: cacheRows, scale: "log" },
  graphio: { title: "Graph I/O medium round-trip latency", subtitle: "min - median - max ns/op; lower is better", rows: graphIORows, scale: "linear" },
  graphdb: { title: "Graph traversal provider latency", subtitle: "min - median - max ns/op; lower is better", rows: graphDBRows, scale: "linear" },
};

function esc(value) {
  return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}

function formatNS(value) {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)} s`;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)} ms`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)} us`;
  return `${value.toFixed(value < 100 ? 1 : 0)} ns`;
}

function marker(shape, x, y, color) {
  if (shape === "square") return `<rect x="${x - 5}" y="${y - 5}" width="10" height="10" rx="1" fill="${color}"/>`;
  if (shape === "diamond") return `<path d="M ${x} ${y - 7} L ${x + 7} ${y} L ${x} ${y + 7} L ${x - 7} ${y} Z" fill="${color}"/>`;
  if (shape === "triangle") return `<path d="M ${x} ${y - 7} L ${x + 7} ${y + 6} L ${x - 7} ${y + 6} Z" fill="${color}"/>`;
  return `<circle cx="${x}" cy="${y}" r="6" fill="${color}"/>`;
}

function renderSVG(chart, rows) {
  const width = 1200;
  const rowH = 48;
  const panelGap = 58;
  const panels = [...new Set(rows.map((row) => row.panel))];
  const height = 190 + rows.length * rowH + panels.length * panelGap + 80;
  const labelX = 44;
  const plotX = 420;
  const plotW = 570;
  const valueX = 1014;
  const positiveMin = Math.min(...rows.map((row) => row.min));
  const max = Math.max(...rows.map((row) => row.max));
  const domainMin = chart.scale === "log" ? Math.pow(10, Math.floor(Math.log10(positiveMin))) : 0;
  const domainMax = chart.scale === "log" ? Math.pow(10, Math.ceil(Math.log10(max))) : max * 1.08;
  const x = (value) => chart.scale === "log"
    ? plotX + ((Math.log10(value) - Math.log10(domainMin)) / (Math.log10(domainMax) - Math.log10(domainMin))) * plotW
    : plotX + (value / domainMax) * plotW;
  const parts = [`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">`, `<defs><style>
    .title{font-family:'Architects Daughter','Comic Sans MS',cursive;font-size:34px;font-weight:700;fill:#0F172A}
    .mono{font-family:'Comic Mono','SFMono-Regular',Consolas,monospace;fill:#334155}
    .subtitle{font-size:16px}.panel{font-size:20px;font-weight:700}.label{font-size:14px}.value{font-size:13px;font-weight:700}.axis{font-size:12px;fill:#64748B}
  </style></defs>`, `<rect width="${width}" height="${height}" fill="#F8FAFC"/>`, `<rect x="20" y="20" width="${width - 40}" height="${height - 40}" rx="24" fill="#FFFFFF" stroke="#CBD5E1" stroke-width="2"/>`, `<text x="44" y="66" class="title">${esc(chart.title)}</text>`, `<text x="44" y="96" class="mono subtitle">${esc(chart.subtitle)}</text>`, `<text x="44" y="124" class="mono axis">Whisker = observed min/max; marker = median; labels and marker shapes identify providers.</text>`];
  let y = 170;
  for (const panel of panels) {
    parts.push(`<text x="44" y="${y}" class="title panel">${esc(panel)}</text>`);
    y += 28;
    const panelRows = rows.filter((row) => row.panel === panel);
    for (const row of panelRows) {
      const style = providers[row.provider] ?? { color: "#475569", shape: "circle" };
      const minX = x(row.min);
      const medianX = x(row.median);
      const maxX = x(row.max);
      parts.push(`<text x="${labelX}" y="${y + 6}" class="mono label">${esc(`${row.provider} - ${row.scenario}`)}</text>`);
      parts.push(`<line x1="${plotX}" y1="${y}" x2="${plotX + plotW}" y2="${y}" stroke="#E2E8F0"/>`);
      parts.push(`<line x1="${minX}" y1="${y}" x2="${maxX}" y2="${y}" stroke="${style.color}" stroke-width="5" stroke-linecap="round"/>`);
      parts.push(`<line x1="${minX}" y1="${y - 7}" x2="${minX}" y2="${y + 7}" stroke="${style.color}" stroke-width="2"/>`);
      parts.push(`<line x1="${maxX}" y1="${y - 7}" x2="${maxX}" y2="${y + 7}" stroke="${style.color}" stroke-width="2"/>`);
      parts.push(marker(style.shape, medianX, y, style.color));
      parts.push(`<text x="${valueX}" y="${y + 5}" class="mono value">${esc(formatNS(row.median))}</text>`);
      y += rowH;
    }
    y += panelGap - 28;
  }
  const axisY = height - 74;
  const ticks = chart.scale === "log"
    ? Array.from({ length: Math.round(Math.log10(domainMax) - Math.log10(domainMin)) + 1 }, (_, index) => domainMin * 10 ** index)
    : [0, 0.25, 0.5, 0.75, 1].map((fraction) => domainMax * fraction);
  for (const tick of ticks) {
    const tickX = chart.scale === "log" ? x(tick) : plotX + (tick / domainMax) * plotW;
    parts.push(`<line x1="${tickX}" y1="${axisY - 8}" x2="${tickX}" y2="${axisY + 2}" stroke="#64748B"/>`);
    parts.push(`<text x="${tickX}" y="${axisY + 22}" text-anchor="middle" class="mono axis">${esc(formatNS(tick))}</text>`);
  }
  parts.push(`<line x1="${plotX}" y1="${axisY}" x2="${plotX + plotW}" y2="${axisY}" stroke="#64748B"/>`, `<text x="${plotX + plotW / 2}" y="${height - 30}" text-anchor="middle" class="mono subtitle">latency (ns/op) - lower is better${chart.scale === "log" ? " - log scale" : ""}</text>`, `</svg>`);
  return { svg: parts.join("\n"), width, height };
}

function vegaSpec(chart, rows, width, height) {
  return {
    $schema: "https://vega.github.io/schema/vega-lite/v5.json",
    title: { text: chart.title, subtitle: chart.subtitle },
    width: width - 460,
    height: height - 180,
    data: { values: rows },
    encoding: {
      y: { field: "scenario", type: "nominal", title: null },
      color: { field: "provider", type: "nominal", title: "Provider" },
      shape: { field: "provider", type: "nominal", title: "Provider" },
      row: { field: "panel", type: "nominal", title: null },
    },
    layer: [
      { mark: { type: "rule", strokeWidth: 4 }, encoding: { x: { field: "min", type: "quantitative", title: "latency (ns/op) - lower is better", scale: { type: chart.scale, zero: chart.scale !== "log" } }, x2: { field: "max" } } },
      { mark: { type: "point", filled: true, size: 100 }, encoding: { x: { field: "median", type: "quantitative", scale: { type: chart.scale, zero: chart.scale !== "log" } } } },
    ],
    config: { font: "Comic Mono, SFMono-Regular, Consolas, monospace", title: { font: "Architects Daughter, Comic Sans MS, cursive" }, background: "#FFFFFF" },
  };
}

function run(command, args) {
  const result = spawnSync(command, args, { encoding: "utf8" });
  if (result.status !== 0) throw new Error(`${command} failed: ${result.stderr || result.stdout}`);
}

function generate(name) {
  const chart = charts[name];
  if (!chart) throw new Error(`unknown chart ${name}`);
  const rows = chart.rows();
  const rendered = renderSVG(chart, rows);
  const base = join(chartDir, `provider-benchmark-${name}-summary`);
  const svgPath = `${base}.svg`;
  const pngPath = `${base}.png`;
  writeFileSync(`${base}.vl.json`, `${JSON.stringify(vegaSpec(chart, rows, rendered.width, rendered.height), null, 2)}\n`);
  writeFileSync(svgPath, `${rendered.svg}\n`);
  run("rsvg-convert", ["--width", String(rendered.width * 2), "--height", String(rendered.height * 2), "--output", pngPath, svgPath]);
  run("cairosvg", [svgPath, "-o", pngPath, "-s", "2"]);
  process.stdout.write(`generated provider-benchmark-${name}-summary.{vl.json,svg,png}\n`);
}

function benchmarkLine(name, value = "100") {
  return `${name}-10  1  ${value} ns/op  10 B/op  1 allocs/op`;
}

function fixtureSection(label, sha, lines, exit = 0) {
  return `command_label: ${label}\ngit_sha: ${sha}\noutput_begin\n${lines.join("\n")}\noutput_end\nexit_status: ${exit}\n`;
}

function expectFailure(name, action) {
  try { action(); } catch { return; }
  throw new Error(`self-test ${name}: expected failure`);
}

function selfTest() {
  const row = "BenchmarkFixture/One";
  const validSchema = [section("one", [row], 1)];
  parseArtifact(fixtureSection("one", expectedSHA, [benchmarkLine(row)]), validSchema, "valid");
  const twoSchema = [section("ordinary", [row], 1), section("expiry", ["BenchmarkFixture/Expiry"], 1)];
  parseArtifact(`${fixtureSection("ordinary", expectedSHA, [benchmarkLine(row)])}${fixtureSection("expiry", expectedSHA, [benchmarkLine("BenchmarkFixture/Expiry")])}`, twoSchema, "valid-two-command");
  expectFailure("unknown", () => parseArtifact(fixtureSection("one", expectedSHA, [benchmarkLine("BenchmarkFixture/Unknown")]), validSchema));
  expectFailure("missing", () => parseArtifact(fixtureSection("one", expectedSHA, []), validSchema));
  expectFailure("duplicate", () => parseArtifact(fixtureSection("one", expectedSHA, [benchmarkLine(row), benchmarkLine(row)]), validSchema));
  expectFailure("non-finite", () => parseArtifact(fixtureSection("one", expectedSHA, [benchmarkLine(row, "NaN")]), validSchema));
  expectFailure("error-exit", () => parseArtifact(fixtureSection("one", expectedSHA, [benchmarkLine(row)], 1), validSchema));
  for (const name of Object.keys(schemas)) load(name);
  process.stdout.write("provider benchmark parser self-tests: PASS\n");
}

const args = process.argv.slice(2);
if (args[0] === "--self-test") {
  selfTest();
} else if (args[0] === "--only" && args[1]) {
  generate(args[1]);
} else if (args.length === 0) {
  for (const name of Object.keys(charts)) generate(name);
} else {
  throw new Error("usage: generate-provider-benchmark-summaries.mjs [--self-test | --only leader|ratelimit|cache|graphio|graphdb]");
}
