import { createServer, IncomingMessage, ServerResponse } from "node:http";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

const sock = process.env.KTHA_SOCK;
if (!sock) {
  console.error("KTHA_SOCK is not set");
  process.exit(1);
}

const indexHtml = fs.readFileSync("/public/index.html", "utf-8");

function readFileOrNA(filePath: string): string {
  try {
    return fs.readFileSync(filePath, "utf-8").trim();
  } catch {
    return "N/A";
  }
}

function getProcessList(): Array<{ pid: number; cmdline: string }> {
  const results: Array<{ pid: number; cmdline: string }> = [];
  try {
    const entries = fs.readdirSync("/proc");
    for (const entry of entries) {
      const pid = parseInt(entry, 10);
      if (isNaN(pid)) continue;
      const cmdline = readFileOrNA(path.join("/proc", entry, "cmdline"))
        .replace(/\0/g, " ")
        .trim();
      results.push({ pid, cmdline: cmdline || "N/A" });
    }
    results.sort((a, b) => a.pid - b.pid);
  } catch {
    // /proc may not be readable
  }
  return results;
}

function getMountTable(): string {
  return readFileOrNA("/proc/self/mounts");
}

function getFileTree(dir: string, prefix: string = "", depth: number = 0): string {
  if (depth > 4) return prefix + "...\n";
  let result = "";
  try {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i];
      const isLast = i === entries.length - 1;
      const connector = isLast ? "\u2514\u2500\u2500 " : "\u251c\u2500\u2500 ";
      const childPrefix = isLast ? "    " : "\u2502   ";
      if (entry.isDirectory()) {
        // skip large host-mounted directories
        if (dir === "/" && (entry.name === "proc" || entry.name === "lib" || entry.name === "lib64")) {
          result += prefix + connector + entry.name + "/  (host mount)\n";
          continue;
        }
        result += prefix + connector + entry.name + "/\n";
        result += getFileTree(path.join(dir, entry.name), prefix + childPrefix, depth + 1);
      } else {
        result += prefix + connector + entry.name + "\n";
      }
    }
  } catch {
    result += prefix + "(permission denied)\n";
  }
  return result;
}

function gatherInfo() {
  return {
    processList: getProcessList(),
    mountTable: getMountTable(),
    fileTree: getFileTree("/"),
    networkInterfaces: os.networkInterfaces(),
    processInfo: {
      pid: process.pid,
      uid: process.getuid?.() ?? -1,
      gid: process.getgid?.() ?? -1,
      hostname: os.hostname(),
    },
    envVars: process.env,
  };
}

function handleRequest(req: IncomingMessage, res: ServerResponse): void {
  const url = new URL(req.url ?? "/", "http://localhost");

  if (req.method === "GET" && url.pathname === "/healthcheck") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ ok: true }));
    return;
  }

  if (req.method === "GET" && url.pathname === "/api/info") {
    const info = gatherInfo();
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(info));
    return;
  }

  if (req.method === "GET" && url.pathname === "/") {
    res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
    res.end(indexHtml);
    return;
  }

  res.writeHead(404, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ error: "not found" }));
}

const server = createServer(handleRequest);

server.listen(sock, () => {
  console.log(`sandbox-inspector listening on ${sock}`);
});
