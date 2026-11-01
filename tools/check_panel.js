// Exercise the embedded panel script with real markup IDs and controlled samples.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const assets = path.join(__dirname, "../internal/panel/assets");
const html = fs.readFileSync(path.join(assets, "index.html"), "utf8");
const elements = new Map();
for (const match of html.matchAll(/\bid="([^"]+)"/g)) {
  const classes = new Set();
  const attributes = new Map();
  elements.set(match[1], {
    textContent: "", style: {}, dataset: {},
    classList: {
      toggle(name, on) { if (on) classes.add(name); else classes.delete(name); },
      contains(name) { return classes.has(name); },
    },
    setAttribute(name, value) { attributes.set(name, value); },
    getAttribute(name) { return attributes.get(name); },
    removeAttribute(name) { attributes.delete(name); },
    addEventListener() {},
    getBoundingClientRect() { return { height: 40 }; },
  });
}

let disk = { total: 1e12, used: 750e9, available: 250e9 };
let tick;
let fits = 0;
const context = {
  document: {
    getElementById(id) { assert(elements.has(id), "missing markup ID: " + id); return elements.get(id); },
    body: { getBoundingClientRect() { return { height: 600 }; } },
  },
  fetch(url) {
    if (url === "/api/fit") fits++;
    const state = url === "/api/update" ? { state: "off" } : {
      cpu: { usage: 20, user: 15, system: 5, idle: 80 },
      gpu: { usage: -1 },
      memory: { used: 4e9, total: 8e9, app: 2e9, wired: 1e9, compressed: 1e9, swap: 0 },
      disk,
    };
    return Promise.resolve({ json() { return Promise.resolve(state); } });
  },
  setInterval(callback) { tick = callback; },
};
vm.runInNewContext(fs.readFileSync(path.join(assets, "app.js"), "utf8"), context);
const flush = () => new Promise(setImmediate);
const el = id => elements.get(id);

async function check() {
  await flush();
  assert.equal(el("diskValue").textContent, "75%");
  assert.equal(el("diskUsed").textContent, "750.0 GB");
  assert.equal(el("diskTotal").textContent, "1.00 TB");
  assert.equal(el("diskAvailable").textContent, "250.0 GB");
  assert.equal(el("diskFill").style.width, "75%");
  assert.equal(el("diskMeter").getAttribute("aria-valuenow"), "75.0");
  assert.equal(el("diskMeter").classList.contains("low-space"), false);
  assert.equal(fits, 1, "the panel must resize after rendering disk capacity");

  for (const [used, expected, warning] of [[899e9, "90%", false], [900e9, "90%", true], [1e12, "100%", true], [0, "0%", false]]) {
    disk = { total: 1e12, used, available: 1e12 - used };
    tick();
    await flush();
    assert.equal(el("diskValue").textContent, expected);
    assert.equal(el("diskMeter").classList.contains("low-space"), warning);
  }

  // A failed sample must clear a previous reading, including the warning.
  disk = { total: 1e12, used: 1e12, available: 0 };
  tick();
  await flush();
  disk = { total: 0, used: 0, available: 0 };
  tick();
  await flush();
  for (const id of ["diskValue", "diskUsed", "diskTotal", "diskAvailable"]) {
    assert.equal(el(id).textContent, "–");
  }
  assert.equal(el("diskFill").style.width, "0%");
  assert.equal(el("diskMeter").classList.contains("low-space"), false);
  assert.equal(el("diskMeter").getAttribute("aria-valuenow"), undefined);
  assert.equal(el("diskMeter").getAttribute("aria-valuetext"), "容量暂不可用");

  disk = { total: 2e12, used: 1.5e12, available: 500e9 };
  tick();
  await flush();
  assert.equal(el("diskValue").textContent, "75%");
  assert.equal(el("diskUsed").textContent, "1.50 TB");
  assert.equal(el("diskAvailable").textContent, "500.0 GB");
  console.log("Panel check passed: disk capacity, warnings, unavailable readings, recovery, and resize.");
}

check().catch(error => { console.error(error); process.exitCode = 1; });
