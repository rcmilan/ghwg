// Minimal browser globals so Ebitengine's js init doesn't panic in Node (logic tests only; nothing draws).
const noop = () => {};
const el = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => "" : k in t ? t[k] : el()), apply: () => el() });
class Document {}
Object.defineProperty(Document.prototype, "hidden", { get() { return false; } });
const docBase = Object.assign(new Document(), {
  hasFocus: () => true, addEventListener: noop, removeEventListener: noop,
  createElement: () => el(), body: el(), documentElement: el(), querySelector: () => null, fonts: el(),
});
const document = new Proxy(docBase, { get: (t, k) => (k in t ? t[k] : el()) });
globalThis.Document = Document;
globalThis.document = document;
globalThis.window = globalThis;
globalThis.addEventListener = noop;
globalThis.removeEventListener = noop;
globalThis.screen = { width: 960, height: 540 };
globalThis.devicePixelRatio = 1;
globalThis.navigator = globalThis.navigator || {};
globalThis.requestAnimationFrame = (f) => setTimeout(() => f(performance.now()), 16);
require(process.env.WASM_EXEC_NODE);
