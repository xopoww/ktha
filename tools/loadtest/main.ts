// Load test for ktha.
//
// Usage:
//   npx tsx tools/loadtest/main.ts [options]
//
// Options:
//   --apps N          Number of apps to provision (default: 10)
//   --rps N           Per-app RPS during steady phase (default: 2)
//   --burst-size N    Requests per burst (default: 8)
//   --mixed N         Duration of mixed (burst-pause) phase in seconds (default: 60)
//   --sustained N     Duration of sustained (all-warm) phase in seconds (default: 0)
//   --base-url URL    Proxy URL (default: http://localhost:40000)
//   --admin-url URL   Admin API URL (default: http://localhost:40001)
//   --admin-key KEY   Admin auth key (default: dev)

import { parseArgs } from "node:util";
import goldenCoinChange from "./golden_coin_change.js";
import goldenWordSearch from "./golden_word_search.js";

const { values: args } = parseArgs({
  options: {
    apps:         { type: "string", default: "10" },
    rps:          { type: "string", default: "2" },
    "burst-size": { type: "string", default: "8" },
    mixed:        { type: "string", default: "60" },
    sustained:    { type: "string", default: "0" },
    "base-url":   { type: "string", default: "http://localhost:40000" },
    "admin-url":  { type: "string", default: "http://localhost:40001" },
    "admin-key":  { type: "string", default: "dev" },
  },
});

const NUM_APPS      = parseInt(args.apps!);
const RPS           = parseFloat(args.rps!);
const BURST_SIZE    = parseInt(args["burst-size"]!);
const MIXED_S       = parseInt(args.mixed!);
const SUSTAINED_S   = parseInt(args.sustained!);
const BASE_URL      = args["base-url"]!;
const ADMIN_URL     = args["admin-url"]!;
const ADMIN_KEY     = args["admin-key"]!;

// Must match ktha-node config. Apps that are idle longer than this will be
// stopped and will cold-start on next request.
const IDLE_TIMEOUT_S = 10;

const IMAGE = "leetcode";
const APP_PREFIX = "lt";

// --- Metrics ---

const latencies: number[] = [];
let totalRequests = 0;
let totalErrors = 0;
let totalMismatches = 0;

function recordLatency(ms: number): void {
  latencies.push(ms);
  totalRequests++;
}

function recordError(): void {
  totalErrors++;
  totalRequests++;
}

function recordMismatch(endpoint: string, input: unknown, expected: unknown, got: unknown): void {
  totalMismatches++;
  console.error(`MISMATCH ${endpoint}: input=${JSON.stringify(input)} expected=${JSON.stringify(expected)} got=${JSON.stringify(got)}`);
}

function percentile(sorted: number[], p: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

function printSummary(label: string): void {
  const sorted = latencies.slice().sort((a, b) => a - b);
  console.log(`\n--- ${label} ---`);
  console.log(`Total requests: ${totalRequests}`);
  console.log(`Errors:         ${totalErrors}`);
  console.log(`Success:        ${totalRequests - totalErrors}`);
  console.log(`Mismatches:     ${totalMismatches}`);
  if (sorted.length > 0) {
    console.log(`Latency p50:    ${percentile(sorted, 50).toFixed(0)} ms`);
    console.log(`Latency p95:    ${percentile(sorted, 95).toFixed(0)} ms`);
    console.log(`Latency p99:    ${percentile(sorted, 99).toFixed(0)} ms`);
    console.log(`Latency max:    ${sorted[sorted.length - 1].toFixed(0)} ms`);
  }
}

function resetMetrics(): void {
  latencies.length = 0;
  totalRequests = 0;
  totalErrors = 0;
  totalMismatches = 0;
}

// --- Admin API ---

function appId(i: number): string {
  return `${APP_PREFIX}-${String(i).padStart(3, "0")}`;
}

async function adminCall(path: string, body: unknown): Promise<void> {
  const res = await fetch(`${ADMIN_URL}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${ADMIN_KEY}`,
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`admin ${path}: ${res.status} ${text}`);
  }
}

async function provisionApps(n: number): Promise<void> {
  console.log(`Provisioning ${n} apps...`);
  for (let i = 0; i < n; i++) {
    await adminCall("/apps/add", { appId: appId(i), image: IMAGE });
    if ((i + 1) % 10 === 0 || i === n - 1) {
      process.stdout.write(`  ${i + 1}/${n}\n`);
    }
  }
}

async function cleanupApps(n: number): Promise<void> {
  console.log(`Cleaning up ${n} apps...`);
  for (let i = 0; i < n; i++) {
    try {
      await adminCall("/apps/delete", { appId: appId(i) });
    } catch {
      // best effort
    }
  }
}

// --- Request sender ---

// Golden test cases are sorted roughly by computational cost (cheapest first).
// The load test picks cases randomly, giving a natural mix of light and heavy
// requests.

async function sendRequest(id: string): Promise<void> {
  // Pick a random endpoint and golden case.
  const useCoinChange = Math.random() < 0.5;

  if (useCoinChange) {
    const tc = goldenCoinChange[Math.floor(Math.random() * goldenCoinChange.length)];
    const url = `${BASE_URL}/${id}/coin-change`;
    const start = performance.now();
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ coins: tc.coins, amount: tc.amount }),
      });
      const elapsed = performance.now() - start;
      if (!res.ok) {
        recordError();
        await res.text();
        return;
      }
      const data = await res.json() as { result: number };
      recordLatency(elapsed);
      if (data.result !== tc.result) {
        recordMismatch("coin-change", { coins: tc.coins, amount: tc.amount }, tc.result, data.result);
      }
    } catch {
      recordError();
    }
  } else {
    const tc = goldenWordSearch[Math.floor(Math.random() * goldenWordSearch.length)];
    const url = `${BASE_URL}/${id}/word-search`;
    const start = performance.now();
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ board: tc.board, word: tc.word }),
      });
      const elapsed = performance.now() - start;
      if (!res.ok) {
        recordError();
        await res.text();
        return;
      }
      const data = await res.json() as { result: boolean };
      recordLatency(elapsed);
      if (data.result !== tc.result) {
        recordMismatch("word-search", { board: tc.board, word: tc.word }, tc.result, data.result);
      }
    } catch {
      recordError();
    }
  }
}

// --- Per-app client ---

function randBetween(min: number, max: number): number {
  return min + Math.random() * (max - min);
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

// Each app independently alternates between steady and burst-pause phases.
// With enough apps, the overall pattern averages out to a natural mix of
// cold starts, warm requests, and idle containers.
async function runMixedClient(id: string, signal: AbortSignal): Promise<void> {
  // Stagger app startup to avoid thundering herd.
  await sleep(Math.random() * 2000);

  while (!signal.aborted) {
    const mode = Math.random() < 0.5 ? "steady" : "burst";

    if (mode === "steady") {
      // Send at configured RPS for a random duration.
      const phaseDuration = randBetween(10, 30) * 1000;
      const interval = 1000 / RPS;
      const phaseEnd = Date.now() + phaseDuration;

      while (Date.now() < phaseEnd && !signal.aborted) {
        sendRequest(id); // fire and forget
        await sleep(interval);
      }
    } else {
      // Burst: send a batch quickly, then pause long enough to trigger cold start.
      const promises: Promise<void>[] = [];
      for (let i = 0; i < BURST_SIZE && !signal.aborted; i++) {
        promises.push(sendRequest(id));
      }
      await Promise.all(promises);

      // Pause longer than idle timeout to guarantee cold start on next request.
      const pause = (IDLE_TIMEOUT_S + randBetween(3, 8)) * 1000;
      await sleep(pause);
    }
  }
}

// All apps send at steady RPS without pauses — every container stays warm.
// Tests sustained concurrent load with N simultaneously running apps.
async function runSustainedClient(id: string, signal: AbortSignal): Promise<void> {
  const interval = 1000 / RPS;
  while (!signal.aborted) {
    sendRequest(id); // fire and forget
    await sleep(interval);
  }
}

// --- Phase runner ---

async function runPhase(
  label: string,
  durationS: number,
  clientFn: (id: string, signal: AbortSignal) => Promise<void>,
  outerSignal: AbortSignal,
): Promise<void> {
  if (durationS <= 0) return;

  resetMetrics();
  console.log(`\n=== ${label} (${durationS}s) ===\n`);

  const ac = new AbortController();

  // Abort on outer signal (SIGINT/SIGTERM) or phase timeout.
  const onAbort = () => ac.abort();
  outerSignal.addEventListener("abort", onAbort, { once: true });
  const timer = setTimeout(() => ac.abort(), durationS * 1000);

  const clients: Promise<void>[] = [];
  for (let i = 0; i < NUM_APPS; i++) {
    clients.push(clientFn(appId(i), ac.signal));
  }

  const progressInterval = setInterval(() => {
    process.stdout.write(`  requests: ${totalRequests}, errors: ${totalErrors}, mismatches: ${totalMismatches}\r`);
  }, 1000);

  await Promise.all(clients);

  clearTimeout(timer);
  clearInterval(progressInterval);
  outerSignal.removeEventListener("abort", onAbort);

  printSummary(label);
}

// --- Main ---

async function main(): Promise<void> {
  console.log("ktha load test");
  console.log(`  apps: ${NUM_APPS}, rps/app: ${RPS}, burst: ${BURST_SIZE}`);
  console.log(`  mixed: ${MIXED_S}s, sustained: ${SUSTAINED_S}s`);
  console.log(`  proxy: ${BASE_URL}, admin: ${ADMIN_URL}`);

  await provisionApps(NUM_APPS);

  const ac = new AbortController();
  const onSignal = () => {
    console.log("\nStopping...");
    ac.abort();
  };
  process.on("SIGINT", onSignal);
  process.on("SIGTERM", onSignal);

  if (!ac.signal.aborted) {
    await runPhase("Mixed (burst-pause)", MIXED_S, runMixedClient, ac.signal);
  }

  if (!ac.signal.aborted) {
    await runPhase("Sustained (all warm)", SUSTAINED_S, runSustainedClient, ac.signal);
  }

  await cleanupApps(NUM_APPS);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
