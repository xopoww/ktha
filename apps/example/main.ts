import { createServer, IncomingMessage, ServerResponse } from "node:http";

const sock = process.env.KTHA_SOCK;
if (!sock) {
  console.error("KTHA_SOCK is not set");
  process.exit(1);
}

const server = createServer((req: IncomingMessage, res: ServerResponse) => {
  if (req.method === "GET" && req.url === "/healthcheck") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ ok: true }));
    return;
  }

  // Add app-specific handlers here:
  //
  // if (req.method === "POST" && req.url === "/echo") {
  //   ...
  //   return;
  // }

  res.writeHead(404);
  res.end();
});

server.listen(sock, () => {
  console.log(`listening on ${sock}`);
});
