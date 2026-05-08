import { createReadStream, existsSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize, resolve, sep } from "node:path";
import { createGzip } from "node:zlib";

const root = resolve("dist");
const port = Number(process.env.PORT || 4173);
const prefix = "/admin/ui";

const contentTypes = new Map([
  [".css", "text/css; charset=utf-8"],
  [".html", "text/html; charset=utf-8"],
  [".js", "text/javascript; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".map", "application/json; charset=utf-8"],
  [".svg", "image/svg+xml"],
  [".ico", "image/x-icon"]
]);

function filePath(urlPath) {
  const stripped = urlPath === prefix ? "/" : urlPath.slice(prefix.length);
  const clean = normalize(stripped).replace(/^(\.\.[/\\])+/, "");
  return join(root, clean === sep ? "index.html" : clean);
}

function sendFile(res, path) {
  const headers = {
    "Content-Type": contentTypes.get(extname(path)) || "application/octet-stream",
    "Cache-Control": path.includes(`${sep}assets${sep}`) ? "public, max-age=31536000, immutable" : "no-cache"
  };
  res.writeHead(200, headers);
  createReadStream(path).pipe(res);
}

function sendCompressedFile(req, res, path) {
  if (!/\bgzip\b/.test(req.headers["accept-encoding"] || "") || ![".css", ".html", ".js", ".json", ".svg"].includes(extname(path))) {
    sendFile(res, path);
    return;
  }
  const headers = {
    "Content-Type": contentTypes.get(extname(path)) || "application/octet-stream",
    "Content-Encoding": "gzip",
    "Cache-Control": path.includes(`${sep}assets${sep}`) ? "public, max-age=31536000, immutable" : "no-cache",
    "Vary": "Accept-Encoding"
  };
  res.writeHead(200, headers);
  createReadStream(path).pipe(createGzip()).pipe(res);
}

function sendJSON(res, value) {
  res.writeHead(200, {
    "Content-Type": "application/json; charset=utf-8",
    "Cache-Control": "no-cache"
  });
  res.end(JSON.stringify(value));
}

function sendAdminAPI(req, res, pathname) {
  if (pathname === "/admin/v1/events/stream") {
    res.writeHead(200, {
      "Content-Type": "text/event-stream; charset=utf-8",
      "Cache-Control": "no-cache",
      Connection: "keep-alive"
    });
    res.write(": ready\n\n");
    return;
  }
  const responses = new Map([
    ["/admin/v1/keys", []],
    ["/admin/v1/pools", []],
    ["/admin/v1/credentials", { api_keys: [], sessions: [], oauth: [], cli: { agents: [] } }],
    ["/admin/v1/health", { upstreams: [] }],
    ["/admin/v1/endpoints", {}],
    ["/admin/v1/audit", { items: [] }],
    ["/admin/v1/budgets", { daily_used_cents: 0, monthly_used_cents: 0, daily_cents: 0, monthly_cents: 0, keys: 0 }],
    ["/admin/v1/usage", { top: [] }]
  ]);
  if (responses.has(pathname)) {
    sendJSON(res, responses.get(pathname));
    return;
  }
  res.writeHead(404);
  res.end("not found");
}

createServer((req, res) => {
  const url = new URL(req.url || "/", `http://127.0.0.1:${port}`);
  if (url.pathname.startsWith("/admin/v1/")) {
    sendAdminAPI(req, res, url.pathname);
    return;
  }
  if (url.pathname === "/favicon.ico") {
    res.writeHead(204, { "Cache-Control": "public, max-age=31536000, immutable" });
    res.end();
    return;
  }
  if (!url.pathname.startsWith(prefix)) {
    res.writeHead(404);
    res.end("not found");
    return;
  }
  const path = filePath(url.pathname);
  if (path.startsWith(root) && existsSync(path)) {
    sendCompressedFile(req, res, path);
    return;
  }
  if (url.pathname.startsWith(`${prefix}/assets/`)) {
    res.writeHead(404);
    res.end("not found");
    return;
  }
  sendCompressedFile(req, res, join(root, "index.html"));
}).listen(port, "127.0.0.1", () => {
  console.log(`Admin UI test server listening on http://127.0.0.1:${port}${prefix}/`);
});
