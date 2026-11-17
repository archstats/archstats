// Screenshots of Archstats Desktop for the documentation site.
//
// Drives the app the way archstats-ui's scripts/capture.mjs does: headless
// Chrome against a running `wails dev` (:34115), with every read pinned to one
// snapshot through QueryIn, so the native window and app.db never move.
// Unlike capture.mjs it serves a static build of the frontend (--dist) in place
// of the dev server's modules, so the pictures show a committed UI rather than
// whatever is mid-edit in the checkout, and it dismisses the "older analysis"
// and import-coverage notices before each shot.
//
// Usage (see README.md beside this file):
//   node shoot.mjs --dist <archstats-ui>/frontend/dist --workspace NAME --out DIR
//     [--scan ID] [--routes name=/hash/path,...] [--scheme light|dark|both]
//     [--dpr 2] [--size 1440x900] [--wait MS] [--actions file.json] [--debug true]
import { spawn } from "node:child_process";
import { writeFileSync, mkdirSync, readFileSync, existsSync, statSync } from "node:fs";
import { join } from "node:path";

const args = Object.fromEntries(process.argv.slice(2).map((a, i, all) => a.startsWith("--") ? [a.slice(2), all[i + 1] && !all[i + 1].startsWith("--") ? all[i + 1] : "true"] : []).filter(Boolean));
const out = args.out ?? ".impeccable/review/captures";
const [W, H] = (args.size ?? "1440x900").split("x").map(Number);
const workspace = args.workspace;
if (!workspace) throw new Error("--workspace NAME is required");
const waitMs = Number(args.wait ?? 2500);
const pin = args.pin !== "false";
const schemes = args.scheme === "both" || !args.scheme ? ["light", "dark"] : [args.scheme];
const DEFAULT_ROUTES = [
  ["dashboard", "/"],
  ["metrics", "/views/metrics"],
  ["hotspots", "/views/components/hotspots"],
  ["cycles", "/views/components/cycles"],
  ["matrix", "/views/components/matrix"],
  ["chord", "/views/components/chord"],
  ["clustering", "/views/components/clustering"],
  ["git-coupling", "/views/git/coupling"],
  ["authors", "/views/git/authors"],
  ["activity", "/views/git/activity"],
  ["files-dependencies", "/views/files/dependencies"],
  ["java-classes", "/views/java/classes"],
];
const routes = args.routes ? args.routes.split(",").map((p) => { const i = p.indexOf("="); return [p.slice(0, i), p.slice(i + 1)]; }) : DEFAULT_ROUTES;
mkdirSync(out, { recursive: true });

const CHROME = process.env.CHROME ?? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Wraps the Wails bindings whenever the page reads window.go. Against
// `wails dev` the binding objects ignore assignment, so patching them in
// place silently left every read on the backend's open snapshot; each level
// is a proxy over a fresh object instead, the bindings' own invariants untouched.
const PIN = `(() => {
  let pinned = null, real = null, wrapped = null, from = null;
  const sqlUrl = ${JSON.stringify(args.sql ?? null)};
  // Writes go through: Wails adds the services one by one after the first read.
  const over = (target, own) => new Proxy({}, {
    get: (_, k) => (k in own ? own[k] : target[k]),
    set: (_, k, v) => { target[k] = v; return true; },
    has: (_, k) => k in own || k in target,
  });
  const wrap = (go) => {
    const q = go && go.app && go.app.QueryService;
    if (!q) return go;
    if (from === go) return wrapped;
    // Looked up at call time: Wails adds a service's methods after its first read.
    const live = () => real.app.QueryService;
    const qs = over(q, {
      Open: async (id) => { pinned = id; return null; },
      // Reads cross as Rows(queryId, scanId, sql); an empty scan id means the backend's open snapshot.
      Rows: sqlUrl
        ? async (_id, _scan, sql) => { const b = await (await fetch(sqlUrl, { method: "POST", body: JSON.stringify({ sql }) })).json(); if (b.error) throw new Error(b.error); return b; }
        : (id, scan, sql) => live().Rows(id, scan || pinned || "", sql),
      Query: (sql) => (pinned ? live().QueryIn(pinned, sql) : Promise.reject(new Error("no scan pinned yet"))),
      CurrentScan: async () => pinned ?? "",
    });
    from = go;
    wrapped = over(go, { app: over(go.app, { QueryService: qs }) });
    return wrapped;
  };
  Object.defineProperty(window, "go", { configurable: true, get() { return wrap(real); }, set(v) { real = v; } });
})()`;

async function session(scheme) {
  const port = 9400 + Math.floor(Math.random() * 100);
  const chrome = spawn(CHROME, [
    "--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
    `--remote-debugging-port=${port}`, `--window-size=${W},${H}`,
    `--user-data-dir=/tmp/archstats-capture-${port}-${Date.now()}`, "about:blank",
  ], { stdio: "ignore" });
  let target;
  for (let i = 0; i < 40 && !target; i++) {
    try { const l = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json(); target = l.find((t) => t.type === "page"); } catch {}
    if (!target) await sleep(250);
  }
  if (!target) { chrome.kill(); throw new Error("headless Chrome did not start"); }
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((r) => (ws.onopen = r));
  let id = 0; const pending = new Map();
  const DIST = args.dist;
  const TYPES = { ".js": "text/javascript", ".css": "text/css", ".html": "text/html", ".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png", ".woff2": "font/woff2", ".wasm": "application/wasm", ".ico": "image/x-icon" };
  ws.onmessage = (m) => {
    const msg = JSON.parse(m.data);
    if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); return; }
    if (args.debug && (msg.method === "Runtime.exceptionThrown" || (msg.method === "Runtime.consoleAPICalled" && !JSON.stringify(msg.params).includes("Queueing")))) console.log("[page]", JSON.stringify(msg.params).slice(0, 400));
    if (msg.method === "Fetch.requestPaused") {
      const { requestId, request } = msg.params;
      const u = new URL(request.url);
      if (!DIST || u.port !== "34115" || u.pathname.startsWith("/wails")) { ws.send(JSON.stringify({ id: ++id, method: "Fetch.continueRequest", params: { requestId } })); return; }
      if (u.pathname === "/_nuxt/@vite/client") { ws.send(JSON.stringify({ id: ++id, method: "Fetch.fulfillRequest", params: { requestId, responseCode: 200, responseHeaders: [{ name: "Content-Type", value: "text/javascript" }], body: Buffer.from("export {}").toString("base64") } })); return; }
      if (u.pathname.endsWith("/nuxt/dist/app/entry.js")) {
        const html = readFileSync(join(DIST, "index.html"), "utf8");
        const entry = html.match(/<script type="module" src="([^"]+)"/)[1];
        const css = [...html.matchAll(/<link rel="stylesheet" href="([^"]+)"/g)].map((m) => m[1]);
        const build = html.match(/buildId:"([^"]+)"/)?.[1];
        const shim = `${css.map((h) => `{ const l = document.createElement("link"); l.rel = "stylesheet"; l.href = ${JSON.stringify(h)}; document.head.append(l); }`).join("\n")}
if (window.__NUXT__ && window.__NUXT__.config && ${JSON.stringify(build)}) window.__NUXT__.config.app.buildId = ${JSON.stringify(build)};
await import(${JSON.stringify(entry)});`;
        ws.send(JSON.stringify({ id: ++id, method: "Fetch.fulfillRequest", params: { requestId, responseCode: 200, responseHeaders: [{ name: "Content-Type", value: "text/javascript" }], body: Buffer.from(shim).toString("base64") } }));
        return;
      }
      let file = join(DIST, decodeURIComponent(u.pathname));
      if (!existsSync(file) || statSync(file).isDirectory()) file = join(DIST, "index.html");
      let body = readFileSync(file);
      const ext = file.slice(file.lastIndexOf("."));
      if (ext === ".html") body = Buffer.from(body.toString().replace("<head>", '<head><script src="/wails/ipc.js"></script><script src="/wails/runtime.js"></script>'));
      ws.send(JSON.stringify({ id: ++id, method: "Fetch.fulfillRequest", params: { requestId, responseCode: 200, responseHeaders: [{ name: "Content-Type", value: TYPES[ext] ?? "application/octet-stream" }], body: body.toString("base64") } }));
    }
  };
  const send = (method, params = {}) => new Promise((r) => { const i = ++id; pending.set(i, r); ws.send(JSON.stringify({ id: i, method, params })); });
  const evaluate = async (expression) => (await send("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true })).result?.result?.value;

  await send("Page.enable"); await send("Network.enable"); if (args.debug) await send("Runtime.enable");
  await send("Network.setCacheDisabled", { cacheDisabled: true });
  await send("Emulation.setDeviceMetricsOverride", { width: W, height: H, deviceScaleFactor: Number(args.dpr ?? 1), mobile: false });
  await send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: scheme }] });
  if (pin) await send("Page.addScriptToEvaluateOnNewDocument", { source: PIN });
  if (DIST) await send("Fetch.enable", { patterns: (process.env.PATS ? process.env.PATS.split(",") : ["/_nuxt/*", "/fonts/*", "/img/*", "/*.wasm", "/favicon.ico"]).map((p) => ({ urlPattern: "http://localhost:34115" + p, requestStage: p === "/" ? "Response" : "Request" })) });
  await send("Page.navigate", { url: (args.url ?? "http://localhost:34115") + "/" });

  // Wait for the app, select the workspace, wait for its data.
  let ready = false;
  for (let i = 0; i < (args.debug ? 6 : 80) && !ready; i++) {
    ready = await evaluate(`(() => {
      const n = document.getElementById('__nuxt'); if (!n || !n.__vue_app__) return false;
      const p = n.__vue_app__.config.globalProperties.$pinia; const ws = p._s.get('workspaces'); const d = p._s.get('data');
      if (ws && ws.workspaces.length && ws.active?.name !== ${JSON.stringify(workspace)}) {
        const t = ws.workspaces.find(x => x.name === ${JSON.stringify(workspace)}); if (t) ws.select(t.id); return false; }
      return !!(ws && ws.loaded && d.hasData);
    })()`).catch(() => false);
    if (!ready) await sleep(500);
  }
  if (!ready) console.warn(`warning: workspace "${workspace}" did not load; capturing anyway`);
  if (args.debug) console.log("[debug]", await evaluate(`(async () => { const g = window.go; let ws = null; ws = await Promise.race([window.go.app.WorkspaceService.List().then(l => l.length, e => String(e)), new Promise(r => setTimeout(() => r("hang"), 2000))]); const wsr = await new Promise((r) => { const w = new WebSocket("ws://localhost:34115/wails/ipc"); w.onopen = () => r("open"); w.onerror = () => r("error"); w.onclose = (e) => r("close " + e.code); setTimeout(() => r("timeout"), 3000); }); return JSON.stringify({ wsr, go: !!g, app: g && g.app ? Object.keys(g.app) : null, ws, wails: typeof window.wails, rt: typeof window.runtime }) })()`));
  // --scan ID: capture that snapshot of the workspace instead of the one it opens on.
  if (args.scan) {
    await evaluate(`(async () => { const p = document.getElementById('__nuxt').__vue_app__.config.globalProperties.$pinia;
      await p._s.get('workspaces').openSnapshot(${JSON.stringify(args.scan)}); return true; })()`).catch((e) => console.warn("warning: --scan", e));
    await sleep(2000);
  }
  await sleep(1000);

  const DISMISS = `(() => { const bs = [...document.querySelectorAll('button[title="Dismiss for this scan"]')]; bs.forEach((b) => b.click()); return bs.length; })()`;
  const ACTIONS = args.actions ? JSON.parse(readFileSync(args.actions, "utf8")) : {};
  for (const [name, route] of routes) {
    await evaluate(`(() => { location.hash = '#${route}'; return true; })()`);
    await sleep(/chord|clustering|dependencies|classes/.test(route) ? waitMs + 1500 : waitMs);
    await evaluate(DISMISS);
    if (ACTIONS[name]) { await evaluate(ACTIONS[name]); await sleep(1500); }
    await sleep(300);
    const r = await send("Page.captureScreenshot", { format: "png" });
    const file = join(out, `${name}${scheme === "dark" ? "-dark" : ""}.png`);
    writeFileSync(file, Buffer.from(r.result.data, "base64"));
    console.log("wrote", file);
  }
  ws.close(); chrome.kill();
}

for (const scheme of schemes) await session(scheme);
