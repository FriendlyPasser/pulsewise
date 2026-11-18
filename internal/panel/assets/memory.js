// Application memory detail view: polls /api/memory once per second while
// visible and pauses automatically when hidden or closed.
(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };
  var GB = 1024 * 1024 * 1024;
  var MB = 1024 * 1024;
  var KB = 1024;

  var MESSAGES = {
    zh: {
      appsCount: function (n) { return n + " 个应用"; },
      procsCount: function (n) { return n + " 个进程"; },
      emptyApps: "暂无可统计的应用进程"
    },
    en: {
      appsCount: function (n) { return n + (n === 1 ? " app" : " apps"); },
      procsCount: function (n) { return n + (n === 1 ? " process" : " processes"); },
      emptyApps: "No application processes reported"
    }
  };

  function messages() {
    var lang = (document.documentElement && document.documentElement.lang) || "";
    return lang.toLowerCase().indexOf("zh") === 0 ? MESSAGES.zh : MESSAGES.en;
  }

  function size(bytes) {
    if (bytes >= GB) return (bytes / GB).toFixed(1) + " GB";
    if (bytes === 0 || bytes >= MB) return Math.round(bytes / MB) + " MB";
    return Math.max(1, Math.round(bytes / KB)) + " KB";
  }

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  var expanded = Object.create(null);
  var unattributedOpen = false;
  var timer = null;

  function renderProcesses(container, procs) {
    container.textContent = "";
    for (var i = 0; i < procs.length; i++) {
      var p = procs[i];
      var row = el("div", "proc-row");
      var info = el("span", "proc-info");
      info.appendChild(el("span", "proc-name", p.name));
      info.appendChild(el("span", "proc-pid", "PID " + p.pid));
      row.appendChild(info);
      row.appendChild(el("span", "proc-mem", size(p.memory)));
      container.appendChild(row);
    }
  }

  function renderApps(apps, msg) {
    var list = $("appList");
    list.textContent = "";
    $("appCount").textContent = msg.appsCount(apps.length);

    if (!apps.length) {
      expanded = Object.create(null);
      list.appendChild(el("div", "empty-state", msg.emptyApps));
      return;
    }

    var nextExpanded = Object.create(null);
    var maxMem = apps[0].memory || 0;
    for (var i = 0; i < apps.length; i++) {
      var app = apps[i];
      var isOpen = !!expanded[app.name];
      if (isOpen) nextExpanded[app.name] = true;

      var item = el("div", "app-item" + (isOpen ? " expanded" : ""));
      var row = el("button", "app-row");
      row.type = "button";
      row.setAttribute("aria-expanded", String(isOpen));

      var top = el("div", "app-row-top");
      var main = el("span", "app-main");
      var chevron = el("span", "row-chevron", "›");
      chevron.setAttribute("aria-hidden", "true");
      main.appendChild(chevron);
      main.appendChild(el("span", "app-name", app.name));
      var procs = app.processes || [];
      main.appendChild(el("span", "app-procs", msg.procsCount(procs.length)));
      top.appendChild(main);
      top.appendChild(el("span", "app-mem", size(app.memory)));
      row.appendChild(top);

      var bar = el("span", "app-bar");
      var fill = el("span", "app-bar-fill");
      var pct = maxMem > 0 ? Math.max(2, Math.min(100, app.memory / maxMem * 100)) : 0;
      fill.style.width = pct.toFixed(1) + "%";
      bar.appendChild(fill);
      row.appendChild(bar);

      var procList = el("div", "proc-list");
      procList.hidden = !isOpen;
      renderProcesses(procList, procs);

      (function (name, itemEl, rowEl, procsEl) {
        rowEl.addEventListener("click", function () {
          var open = !expanded[name];
          if (open) {
            expanded[name] = true;
          } else {
            delete expanded[name];
          }
          itemEl.classList.toggle("expanded", open);
          rowEl.setAttribute("aria-expanded", String(open));
          procsEl.hidden = !open;
        });
      })(app.name, item, row, procList);

      item.appendChild(row);
      item.appendChild(procList);
      list.appendChild(item);
    }
    expanded = nextExpanded;
  }

  function syncUnattributedVisibility() {
    var card = $("unattributedCard");
    var toggle = $("unattributedToggle");
    var list = $("unattributedList");
    card.classList.toggle("expanded", unattributedOpen);
    toggle.setAttribute("aria-expanded", String(unattributedOpen));
    list.hidden = !unattributedOpen;
  }

  function render(d) {
    var m = (d && d.memory) || {};
    var msg = messages();

    $("detailMemUsed").textContent = m.total > 0 ? (m.used / GB).toFixed(1) : "–";
    $("detailMemTotal").textContent = m.total > 0 ? String(Math.round(m.total / GB)) : "–";
    $("detailMemApp").textContent = m.total > 0 ? size(m.app) : "–";
    $("detailMemWired").textContent = m.total > 0 ? size(m.wired) : "–";
    $("detailMemComp").textContent = m.total > 0 ? size(m.compressed) : "–";
    $("detailMemSwap").textContent = m.total > 0 ? size(m.swap) : "–";

    renderApps((d && d.apps) || [], msg);

    var unattr = (d && d.unattributed) || [];
    var card = $("unattributedCard");
    if (!unattr.length) {
      card.hidden = true;
      $("unattributedList").textContent = "";
      return;
    }
    card.hidden = false;
    $("unattributedCount").textContent = msg.procsCount(unattr.length);
    $("unattributedMem").textContent = size((d && d.unattributedMemory) || 0);
    renderProcesses($("unattributedList"), unattr);
    syncUnattributedVisibility();
  }

  $("unattributedToggle").addEventListener("click", function () {
    unattributedOpen = !unattributedOpen;
    syncUnattributedVisibility();
  });

  function isVisible() {
    return document.visibilityState !== "hidden" && !document.hidden;
  }

  function tick() {
    fetch("/api/memory", { cache: "no-store" })
      .then(function (r) { return r.json(); })
      .then(render)
      .catch(function () {});
  }

  function startPolling() {
    if (timer !== null || !isVisible()) return;
    tick();
    timer = setInterval(tick, 1000);
  }

  function stopPolling() {
    if (timer === null) return;
    clearInterval(timer);
    timer = null;
  }

  document.addEventListener("visibilitychange", function () {
    if (isVisible()) {
      startPolling();
    } else {
      stopPolling();
    }
  });
  if (typeof window !== "undefined" && window.addEventListener) {
    window.addEventListener("pagehide", stopPolling);
    window.addEventListener("beforeunload", stopPolling);
  }

  startPolling();
})();
