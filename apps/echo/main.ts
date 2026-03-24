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

const server = createServer(async (req: IncomingMessage, res: ServerResponse) => {
  if (req.method === "GET" && req.url === "/healthcheck") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ ok: true }));
    return;
  }

  if (req.method === "POST" && req.url === "/echo") {
    const body = await readBody(req);
    let message: string;
    try {
      message = JSON.parse(body).message;
    } catch {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "invalid json" }));
      return;
    }
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ response: message }));
    return;
  }

  res.writeHead(404);
  res.end();
});

server.listen(sock, () => {
  console.log(`listening on ${sock}`);
});
