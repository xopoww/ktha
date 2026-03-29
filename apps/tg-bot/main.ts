import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { Bot, webhookCallback } from "grammy";

const sock = process.env.KTHA_SOCK;
if (!sock) {
  console.error("KTHA_SOCK is not set");
  process.exit(1);
}

const token = process.env.TG_BOT_TOKEN;

let handleUpdate: ((req: IncomingMessage, res: ServerResponse) => Promise<void>) | null = null;

if (!token) {
  console.warn("TG_BOT_TOKEN is not set");
} else {
  const bot = new Bot(token, {
    // Provide botInfo to skip the getMe call at init time — that would
    // require an outgoing HTTP request which we explicitly block below.
    botInfo: {
      "id": 8606354409,
      "is_bot": true,
      "first_name": "KTHA Demo Bot",
      "username": "ktha_demo_bot",
      "can_join_groups": false,
      "can_read_all_group_messages": false,
      "supports_inline_queries": false,
      "can_connect_to_business": false,
      "has_main_web_app": false,
      "has_topics_enabled": false,
      "allows_users_to_create_topics": false
    },
    client: {
      // Enable reply-via-webhook-response: the first API call per update is
      // written to the HTTP response body instead of making a separate request.
      canUseWebhookReply: () => true,
      // Safety net: if anything bypasses webhook reply and tries to call the
      // Telegram API over HTTP, fail loudly instead of leaking requests.
      fetch: () => {
        throw new Error("outgoing Telegram API calls are not allowed; use webhook reply");
      },
    },
  });

  let startCount = 0;

  bot.command("start", (ctx) => {
    startCount++;
    return ctx.reply(
      `This command has been invoked ${startCount} times since container startup.`,
    );
  });

  handleUpdate = webhookCallback(bot, "http", {
    secretToken: process.env.TG_WEBHOOK_SECRET,
  });
}

const server = createServer(async (req: IncomingMessage, res: ServerResponse) => {
  if (req.method === "GET" && req.url === "/healthcheck") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ ok: true }));
    return;
  }

  if (req.method === "POST" && req.url === "/webhook") {
    if (!handleUpdate) {
      res.writeHead(503);
      res.end();
      return;
    }
    try {
      await handleUpdate(req, res);
    } catch (err) {
      console.error("error handling update:", err);
      if (!res.headersSent) {
        res.writeHead(500);
        res.end(String(err));
      }
    }
    return;
  }

  res.writeHead(404);
  res.end();
});

server.listen(sock, () => {
  console.log(`tg-bot listening on ${sock}`);
});
