// The panel: reads /api/state once a second and draws it, with no
// framework. The window is sized to the page through /api/fit.
(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };
  var GB = 1024 * 1024 * 1024;
  var MB = 1024 * 1024;

  function pct(v) { return Math.round(v) + "%"; }

  function size(bytes) {
    if (bytes >= GB) return (bytes / GB).toFixed(1) + " GB";
    return Math.round(bytes / MB) + " MB";
  }

  // the design's sparkline: 288×28, its points spaced evenly, a level of
  // 100% near the top and 0% near the foot
  function spark(el, history) {
    var n = 17, step = 288 / (n - 1);
    var pad = Array(Math.max(0, n - history.length)).fill(history[0] || 0);
    var points = pad.concat(history.slice(-n)).map(function (v, i) {
      var y = 26 - Math.max(0, Math.min(100, v)) * 0.24;
      return Math.round(i * step) + "," + y.toFixed(1);
    });
    el.setAttribute("points", points.join(" "));
  }

  function width(el, part, total) {
    el.style.width = total > 0 ? (part / total * 100).toFixed(1) + "%" : "0";
  }

  function render(s) {
    var c = s.cpu, g = s.gpu, m = s.memory;

    $("cpuValue").textContent = pct(c.usage);
    $("cpuUser").textContent = pct(c.user);
    $("cpuSys").textContent = pct(c.system);
    $("cpuIdle").textContent = pct(c.idle);
    spark($("cpuSpark"), c.history || []);

    $("gpuValue").textContent = g.usage < 0 ? "–" : pct(g.usage);
    $("gpuModel").textContent = g.model || "–";
    $("gpuCores").textContent = g.cores > 0 ? String(g.cores) : "–";
    spark($("gpuSpark"), g.history || []);

    $("memUsed").textContent = (m.used / GB).toFixed(1);
    $("memTotal").textContent = String(Math.round(m.total / GB));
    width($("memAppBar"), m.app, m.total);
    width($("memWiredBar"), m.wired, m.total);
    width($("memCompBar"), m.compressed, m.total);
    $("memApp").textContent = size(m.app);
    $("memWired").textContent = size(m.wired);
    $("memComp").textContent = size(m.compressed);
    $("memSwap").textContent = size(m.swap);

    fit();
  }

  var fitted = 0;
  function fit() {
    var h = Math.ceil(document.body.getBoundingClientRect().height);
    if (h === fitted) return;
    fitted = h;
    post("/api/fit", { height: h });
  }

  function post(path, body) {
    return fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {})
    }).catch(function () {});
  }

  function tick() {
    fetch("/api/state", { cache: "no-store" })
      .then(function (r) { return r.json(); })
      .then(render)
      .catch(function () {});
  }

  $("quit").addEventListener("click", function () { post("/api/quit"); });

  tick();
  setInterval(tick, 1000);
})();
