// webprof: plays the web build in headless Chrome on the real GPU and reports its frame times; the GPU side of a
// frame shows only in a browser (08 W3: shifts fell to 8 FPS and a growth in ghost crashed the browser, neither
// visible in the Go benchmarks). Serve first (sh tools/serve.sh), then:
//
//	node tools/webprof.mjs "<main's flags>" [seconds] [--series] [--cpu]
//	node tools/webprof.mjs "-level 1 -busy -shifts" 6 --cpu
//
// The flags reach the game through the page's ?args=. --series prints every frame's ms; --cpu the functions using
// the most CPU (names need a build that keeps them: GOOS=js GOARCH=wasm go build -o web/game.wasm . && gzip -9 -nfk
// web/game.wasm; tools/web.sh strips them). CHROME overrides the browser's path; SETTLE the ms waited before measuring; LOGS=1 prints the page's
// console (a println in the game lands there).
import { spawn } from "node:child_process";
import os from "node:os";

const opt = process.argv.slice(2).filter(a => a.startsWith("--")), pos = process.argv.slice(2).filter(a => !a.startsWith("--"));
const [args = "", secs = "6"] = pos;
const chrome = process.env.CHROME ?? "C:/Program Files/Google/Chrome/Application/chrome.exe";
const port = 9333, settle = +(process.env.SETTLE ?? 4000);
const browser = spawn(chrome, ["--headless=new", `--remote-debugging-port=${port}`, `--user-data-dir=${os.tmpdir()}/webprof`, "--use-angle=d3d11",
  "--enable-gpu", "--ignore-gpu-blocklist", "--window-size=960,540", "--no-first-run", "about:blank"], { stdio: "ignore" });
const sleep = ms => new Promise(r => setTimeout(r, ms));
let ws, id = 0;
const pend = new Map(), events = [];
const send = (method, params = {}, sessionId) => new Promise(res => { const i = ++id; pend.set(i, res); ws.send(JSON.stringify({ id: i, method, params, sessionId })); });
const watchdog = setTimeout(() => { console.log(`${args} | no frame for ${+secs + 30}s: hung or crashed`); browser.kill(); process.exit(1); }, (settle / 1000 + +secs + 30) * 1000);
try {
  let ver;
  for (let i = 0; i < 50 && !ver; i++) { try { ver = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json(); } catch { await sleep(200); } }
  ws = new WebSocket(ver.webSocketDebuggerUrl);
  await new Promise(r => ws.onopen = r);
  ws.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pend.has(d.id)) { pend.get(d.id)(d.result ?? d); pend.delete(d.id); } else events.push(d); };
  const { targetId } = await send("Target.createTarget", { url: "about:blank" });
  const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
  const s = (m, p) => send(m, p, sessionId);
  await s("Runtime.enable"); await s("Profiler.enable");
  await s("Emulation.setDeviceMetricsOverride", { width: 960, height: 540, deviceScaleFactor: 1, mobile: false });
  await s("Page.navigate", { url: `http://localhost:8080/play?fps&args=${encodeURIComponent(args)}` });
  await sleep(settle);
  const ev = async e => (await s("Runtime.evaluate", { expression: e, awaitPromise: true, returnByValue: true })).result?.value;
  const gpu = await ev(`(()=>{const c=document.createElement('canvas').getContext('webgl2');const x=c&&c.getExtension('WEBGL_debug_renderer_info');return x?c.getParameter(x.UNMASKED_RENDERER_WEBGL):'?'})()`);
  if (opt.includes("--cpu")) await s("Profiler.start");
  const t = await ev(`new Promise(r=>{const a=[];let last=performance.now();const end=last+${+secs * 1000};(function f(n){a.push(n-last);last=n;if(n<end)requestAnimationFrame(f);else r(a)})(last)})`);
  const ms = t.slice(1);
  if (process.env.LOGS) for (const e of events) if (e.method === "Runtime.consoleAPICalled") console.log(e.params.args.map(a => a.value).join(" "));
  if (opt.includes("--series")) console.log(ms.map(x => x.toFixed(0)).join(" "));
  const a = [...ms].sort((x, y) => x - y), q = f => a[Math.min(a.length - 1, Math.floor(a.length * f))].toFixed(1);
  const errs = events.filter(e => e.method === "Runtime.exceptionThrown" || (e.method === "Runtime.consoleAPICalled" && e.params.type === "error")).length;
  console.log(`${args} | ${String(gpu).slice(0, 40)} | ${(a.length / secs).toFixed(0)} fps | frame ms p50 ${q(.5)} p90 ${q(.9)} p99 ${q(.99)} max ${a.at(-1).toFixed(0)} | errors ${errs}`);
  if (opt.includes("--cpu")) {
    const { profile: p } = await s("Profiler.stop");
    const dt = new Map(); p.samples.forEach((n, i) => dt.set(n, (dt.get(n) ?? 0) + (p.timeDeltas[i] ?? 0)));
    const byId = new Map(p.nodes.map(x => [x.id, x])), parent = new Map(), incl = new Map(), self = new Map();
    for (const x of p.nodes) for (const c of x.children ?? []) parent.set(c, x.id);
    let tot = 0;
    for (const x of p.nodes) {
      const t = (dt.get(x.id) ?? 0) / 1000; tot += t;
      const name = x.callFrame.functionName || `(${x.callFrame.url.split("/").pop() || "native"})`;
      self.set(name, (self.get(name) ?? 0) + t);
      const seen = new Set();
      for (let i = x.id; i; i = parent.get(i)) { const k = byId.get(i).callFrame.functionName; if (k.startsWith("ghosthotel") && !seen.has(k)) { seen.add(k); incl.set(k, (incl.get(k) ?? 0) + t); } }
    }
    const show = (title, m) => { console.log(title); [...m].sort((x, y) => y[1] - x[1]).slice(0, 15).forEach(([k, v]) => console.log(`${(100 * v / tot).toFixed(1).padStart(6)}%  ${k}`)); };
    show("self time:", self); show("the game's functions, with what they call:", incl);
  }
} finally { clearTimeout(watchdog); browser.kill(); }
