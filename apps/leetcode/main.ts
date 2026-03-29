import { createServer, IncomingMessage, ServerResponse } from "node:http";

const sock = process.env.KTHA_SOCK;
if (!sock) {
  console.error("KTHA_SOCK is not set");
  process.exit(1);
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks).toString()));
    req.on("error", reject);
  });
}

function json(res: ServerResponse, status: number, body: unknown): void {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(body));
}

// https://leetcode.com/problems/coin-change/
function coinChange(coins: number[], amount: number): number {
  const dp = new Array(amount + 1).fill(Infinity);
  dp[0] = 0;
  for (let i = 1; i <= amount; i++) {
    for (const coin of coins) {
      if (coin <= i && dp[i - coin] + 1 < dp[i]) {
        dp[i] = dp[i - coin] + 1;
      }
    }
  }
  return dp[amount] === Infinity ? -1 : dp[amount];
}

// https://leetcode.com/problems/word-search/
function wordSearch(board: string[][], word: string): boolean {
  const rows = board.length;
  const cols = board[0].length;

  function dfs(r: number, c: number, idx: number): boolean {
    if (idx === word.length) return true;
    if (r < 0 || r >= rows || c < 0 || c >= cols) return false;
    if (board[r][c] !== word[idx]) return false;

    const saved = board[r][c];
    board[r][c] = "#";

    const found =
      dfs(r + 1, c, idx + 1) ||
      dfs(r - 1, c, idx + 1) ||
      dfs(r, c + 1, idx + 1) ||
      dfs(r, c - 1, idx + 1);

    board[r][c] = saved;
    return found;
  }

  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      if (dfs(r, c, 0)) return true;
    }
  }
  return false;
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url ?? "/", "http://localhost");

  if (req.method === "GET" && url.pathname === "/healthcheck") {
    return json(res, 200, { ok: true });
  }

  if (req.method === "POST" && url.pathname === "/coin-change") {
    let body: unknown;
    try {
      body = JSON.parse(await readBody(req));
    } catch {
      return json(res, 400, { error: "invalid JSON" });
    }

    if (typeof body !== "object" || body === null) {
      return json(res, 400, { error: "request body must be an object" });
    }

    const { coins, amount } = body as Record<string, unknown>;

    if (
      !Array.isArray(coins) ||
      coins.length === 0 ||
      !coins.every((c) => typeof c === "number" && Number.isInteger(c) && c > 0)
    ) {
      return json(res, 400, {
        error: "coins must be a non-empty array of positive integers",
      });
    }

    if (typeof amount !== "number" || !Number.isInteger(amount) || amount < 0 || amount > 100_000) {
      return json(res, 400, {
        error: "amount must be a non-negative integer (max 100000)",
      });
    }

    const result = coinChange(coins, amount);
    return json(res, 200, { result });
  }

  if (req.method === "POST" && url.pathname === "/word-search") {
    let body: unknown;
    try {
      body = JSON.parse(await readBody(req));
    } catch {
      return json(res, 400, { error: "invalid JSON" });
    }

    if (typeof body !== "object" || body === null) {
      return json(res, 400, { error: "request body must be an object" });
    }

    const { board, word } = body as Record<string, unknown>;

    if (typeof word !== "string" || word.length === 0) {
      return json(res, 400, { error: "word must be a non-empty string" });
    }

    if (
      !Array.isArray(board) ||
      board.length === 0 ||
      board.length > 20 ||
      !board.every(
        (row) =>
          Array.isArray(row) &&
          row.length === board[0].length &&
          row.length > 0 &&
          row.length <= 20 &&
          row.every((cell) => typeof cell === "string" && cell.length === 1),
      )
    ) {
      return json(res, 400, {
        error:
          "board must be a non-empty 2D array of single-character strings, max 20x20",
      });
    }

    const result = wordSearch(board as string[][], word);
    return json(res, 200, { result });
  }

  json(res, 404, { error: "not found" });
});

server.listen(sock, () => {
  console.log(`leetcode app listening on ${sock}`);
});
