// The panel: reads /api/state once a second and draws it, with no
// framework. The window is sized to the page through /api/fit.
(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };
  var GB = 1024 * 1024 * 1024;
  var MB = 1024 * 1024;

  var MESSAGES = {
    zh: {
      diskUsed: function (used, avail) { return "已用 " + used + "，可用 " + avail; },
      diskUnavailable: "容量暂不可用",
      downloading: "下载中",
      update: "更新",
      restartTo: "重启以更新到 ",
      newVersion: "新版本",
      translocated: "先把 Pulsewise 移到“应用程序”文件夹，它才能自己更新",
      readOnly: "Pulsewise 在只读的磁盘上，没法自己更新",
      downloadLine: "\n下载 ",
      restarting: "正在重启…"
    },
    en: {
      diskUsed: function (used, avail) { return "Used " + used + ", available " + avail; },
      diskUnavailable: "Capacity unavailable",
      downloading: "Downloading",
      update: "Update",
      restartTo: "Restart to update to ",
      newVersion: "New Version",
      translocated: "Move Pulsewise to the Applications folder so it can update itself",
      readOnly: "Pulsewise is on a read-only disk and cannot update itself",
      downloadLine: "\nDownload ",
      restarting: "Restarting…"
    }
  };

  function messages() {
    var lang = (document.documentElement && document.documentElement.lang) || "";
    return lang.toLowerCase().indexOf("zh") === 0 ? MESSAGES.zh : MESSAGES.en;
  }

  function pct(v) { return Math.round(v) + "%"; }

  function size(bytes) {
    if (bytes >= GB) return (bytes / GB).toFixed(1) + " GB";
    return Math.round(bytes / MB) + " MB";
  }

  // Disk capacities use decimal units, as macOS storage settings do.
  function diskSize(bytes) {
    if (bytes >= 1e12) return (bytes / 1e12).toFixed(2) + " TB";
    if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
    return Math.round(bytes / 1e6) + " MB";
  }

  // Network rates use decimal SI units, matching Activity Monitor's Network tab.
  function rate(bytesPerSec) {
    var v = Math.max(0, bytesPerSec || 0);
    if (v >= 1e9) return (v / 1e9).toFixed(2) + " GB/s";
    if (v >= 1e6) return (v / 1e6).toFixed(1) + " MB/s";
    if (v >= 1e3) return (v / 1e3).toFixed(1) + " KB/s";
    return Math.round(v) + " B/s";
  }

  // Fritsch–Butland monotone cubic Hermite spline: keeps horizontal crests at
  // peaks/troughs and prevents overshoot above peaks or below the baseline.
  function smoothLine(pts) {
    var n = pts.length;
    if (!n) return "";
    if (n === 1) return "M" + pts[0][0].toFixed(1) + "," + pts[0][1].toFixed(1);
    var d = [], i;
    for (i = 0; i < n - 1; i++) {
      var dx = pts[i + 1][0] - pts[i][0];
      d.push(dx === 0 ? 0 : (pts[i + 1][1] - pts[i][1]) / dx);
    }
    var m = new Array(n);
    m[0] = d[0];
    m[n - 1] = d[n - 2];
    for (i = 1; i < n - 1; i++) {
      m[i] = d[i - 1] * d[i] <= 0 ? 0 : (2 * d[i - 1] * d[i]) / (d[i - 1] + d[i]);
    }
    var path = "M" + pts[0][0].toFixed(1) + "," + pts[0][1].toFixed(1);
    for (i = 0; i < n - 1; i++) {
      var x0 = pts[i][0], y0 = pts[i][1], x1 = pts[i + 1][0], y1 = pts[i + 1][1];
      var dx3 = (x1 - x0) / 3;
      path += "C" + (x0 + dx3).toFixed(1) + "," + (y0 + m[i] * dx3).toFixed(1) +
        " " + (x1 - dx3).toFixed(1) + "," + (y1 - m[i + 1] * dx3).toFixed(1) +
        " " + x1.toFixed(1) + "," + y1.toFixed(1);
    }
    return path;
  }

  function smoothArea(linePath, pts, baseY) {
    if (!linePath || pts.length < 2) return "";
    return linePath +
      "L" + pts[pts.length - 1][0].toFixed(1) + "," + baseY.toFixed(1) +
      "L" + pts[0][0].toFixed(1) + "," + baseY.toFixed(1) + "Z";
  }

  function renderNetwork(net) {
    net = net || {};
    var hist = Array.isArray(net.history) ? net.history : [];
    $("netDown").textContent = rate(net.down);
    $("netUp").textContent = rate(net.up);

    var n = 17, step = 288 / (n - 1);
    var slice = hist.slice(-n);
    var win = Array(Math.max(0, n - slice.length)).fill({ down: 0, up: 0 }).concat(slice);

    var peak = win.reduce(function (m, p) {
      return Math.max(m, p.down || 0, p.up || 0);
    }, 0);
    $("netPeak").textContent = rate(peak);

    var scale = Math.max(peak, 1000);
    var plot = function (lineEl, areaEl, key) {
      var pts = win.map(function (pt, idx) {
        return [idx * step, 30 - Math.max(0, Math.min(1, (pt[key] || 0) / scale)) * 28];
      });
      var d = smoothLine(pts);
      lineEl.setAttribute("d", d);
      areaEl.setAttribute("d", smoothArea(d, pts, 30));
    };
    plot($("netDownSpark"), $("netDownArea"), "down");
    plot($("netUpSpark"), $("netUpArea"), "up");
  }

  function renderDisk(d) {
    var available = d.total > 0;
    var usage = available ? Math.max(0, Math.min(100, d.used / d.total * 100)) : 0;
    $("diskValue").textContent = available ? pct(usage) : "–";
    $("diskUsed").textContent = available ? diskSize(d.used) : "–";
    $("diskTotal").textContent = available ? diskSize(d.total) : "–";
    $("diskAvailable").textContent = available ? diskSize(d.available) : "–";
    $("diskFill").style.width = usage + "%";
    var meter = $("diskMeter");
    meter.classList.toggle("low-space", available && usage >= 90);
    var m = messages();
    if (available) {
      meter.setAttribute("aria-valuenow", usage.toFixed(1));
      meter.setAttribute("aria-valuetext", m.diskUsed(pct(usage), diskSize(d.available)));
    } else {
      meter.removeAttribute("aria-valuenow");
      meter.setAttribute("aria-valuetext", m.diskUnavailable);
    }
  }

  // the design's sparkline: 288×28, its points spaced evenly, a level of
  // 100% near the top and 0% near the foot
  function spark(lineEl, areaEl, history) {
    var n = 17, step = 288 / (n - 1);
    var pad = Array(Math.max(0, n - history.length)).fill(history[0] || 0);
    var pts = pad.concat(history.slice(-n)).map(function (v, i) {
      return [i * step, 26 - Math.max(0, Math.min(100, v)) * 0.24];
    });
    var d = smoothLine(pts);
    lineEl.setAttribute("d", d);
    areaEl.setAttribute("d", smoothArea(d, pts, 26));
  }

  // memory's stacked area, 288×40 with 0 at the foot and the total at the
  // top. One more point than fits is drawn, the newest past the right
  // edge; the group then slides a step left over the second, so each new
  // sample glides in where the last one ended.
  function area(history, total) {
    var n = 17, step = 288 / (n - 2), H = 40;
    var h = history.slice(-n);
    while (h.length && h.length < n) h.unshift(h[0]);
    if (!h.length || total <= 0) return;
    var y = function (v) { return H - Math.min(v / total, 1) * H; };
    var base = h.map(function () { return H; });
    var app = h.map(function (p) { return y(p.app); });
    var wired = h.map(function (p) { return y(p.app + p.wired); });
    var comp = h.map(function (p) { return y(p.app + p.wired + p.compressed); });
    $("memAppArea").setAttribute("d", band(app, base, step));
    $("memWiredArea").setAttribute("d", band(wired, app, step));
    $("memCompArea").setAttribute("d", band(comp, wired, step));

    var g = $("memArea");
    g.style.transition = "none";
    g.style.transform = "translateX(0)";
    g.getBoundingClientRect();
    g.style.transition = "transform 1s linear";
    g.style.transform = "translateX(" + (-step).toFixed(2) + "px)";
  }

  // the band between two curves, each drawn smooth through its points
  function band(top, bottom, step) {
    var pts = function (ys) { return ys.map(function (v, i) { return [i * step, v]; }); };
    var back = pts(bottom).reverse();
    return curve(pts(top), "M") + curve(back, "L") + "Z";
  }

  function curve(p, start) {
    var d = start + p[0][0].toFixed(1) + "," + p[0][1].toFixed(1);
    for (var i = 1; i < p.length; i++) {
      var m = ((p[i - 1][0] + p[i][0]) / 2).toFixed(1);
      d += "C" + m + "," + p[i - 1][1].toFixed(1) + " " + m + "," + p[i][1].toFixed(1) +
        " " + p[i][0].toFixed(1) + "," + p[i][1].toFixed(1);
    }
    return d;
  }

  function render(s) {
    var c = s.cpu, g = s.gpu, m = s.memory;

    $("cpuValue").textContent = pct(c.usage);
    $("cpuUser").textContent = pct(c.user);
    $("cpuSys").textContent = pct(c.system);
    spark($("cpuSpark"), $("cpuArea"), c.history || []);

    $("gpuValue").textContent = g.usage < 0 ? "–" : pct(g.usage);
    $("gpuModel").textContent = g.model || "–";
    $("gpuCores").textContent = g.cores > 0 ? String(g.cores) : "–";
    spark($("gpuSpark"), $("gpuArea"), g.history || []);

    $("memUsed").textContent = (m.used / GB).toFixed(1);
    $("memTotal").textContent = String(Math.round(m.total / GB));
    area(m.history || [], m.total);
    $("memApp").textContent = size(m.app);
    $("memWired").textContent = size(m.wired);
    $("memComp").textContent = size(m.compressed);
    $("memSwap").textContent = size(m.swap);

    renderDisk(s.disk);
    renderNetwork(s.network);
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
    fetch("/api/update", { cache: "no-store" })
      .then(function (r) { return r.json(); })
      .then(renderUpdate)
      .catch(function () {});
  }

  // the update's state (internal/update's Status), a pill at the header's
  // right: what it does is the button's do; drawn again only when it
  // changes, so a hover isn't lost each second
  var shown = "";
  function renderUpdate(u) {
    var m = messages();
    var label = "", title = "", act = "", busy = false;
    switch (u.state) {
      case "downloading":
        label = m.downloading + (u.total > 0 ? " " + Math.floor(u.done / u.total * 100) + "%" : "…");
        busy = true;
        break;
      case "ready":
        label = m.update;
        act = "restart";
        title = m.restartTo + u.latest + (u.notes ? "\n\n" + u.notes : "");
        break;
      case "available":
        label = m.newVersion;
        act = "page";
        title = (u.stuck === "translocated" ? m.translocated : m.readOnly) + m.downloadLine + u.latest;
        break;
    }
    var key = [label, title, act, busy].join("\n");
    if (key === shown) return;
    shown = key;
    var el = $("update");
    $("updateLabel").textContent = label;
    el.title = title;
    el.dataset.do = act;
    el.classList.toggle("busy", busy);
    el.hidden = label === "";
  }

  $("update").addEventListener("click", function () {
    var el = $("update");
    if (el.dataset.do === "restart") {
      el.classList.add("busy");
      $("updateLabel").textContent = messages().restarting;
      // the app quits; if it couldn't, the next tick draws the pill again
      post("/api/update/restart").then(function () { shown = ""; });
    } else if (el.dataset.do === "page") {
      post("/api/update/page");
    }
  });

  function bindOpen(id, endpoint) {
    var el = $(id);
    el.addEventListener("click", function () { post(endpoint); });
    el.addEventListener("keydown", function (e) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        post(endpoint);
      }
    });
  }

  bindOpen("memMetric", "/api/memory/open");
  bindOpen("diskMetric", "/api/disk/open");

  $("quit").addEventListener("click", function () { post("/api/quit"); });

  tick();
  setInterval(tick, 1000);
})();
