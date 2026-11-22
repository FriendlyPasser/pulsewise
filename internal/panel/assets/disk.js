(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };

  var MESSAGES = {
    zh: {
      diskUsed: function (used, avail) { return "已用 " + used + "，可用 " + avail; },
      diskUnavailable: "容量暂不可用",
      presets: {
        downloads: "下载",
        desktop: "桌面",
        documents: "文稿",
        home: "个人目录"
      },
      states: {
        idle: "就绪",
        scanning: "扫描中",
        completed: "已完成",
        cancelled: "已取消",
        error: "扫描失败"
      },
      scan: "开始扫描",
      rescan: "重新扫描",
      idleHint: "选择目录后点击“开始扫描”以查找占用空间的大文件。",
      scanningHint: function (files, dirs, bytes) {
        return "已扫描 " + files + " 个文件（" + dirs + " 个目录，共 " + bytes + "）…";
      },
      completedHint: function (count) {
        return count > 0 ? "已列出占用最大的 " + count + " 个文件。" : "扫描完成。";
      },
      cancelledHint: "扫描已取消。",
      errorHint: "无法读取选定目录。",
      scopeMeta: function (root) { return "范围：" + root; },
      timeMeta: function (at, dur) { return at ? ("时间：" + at + " · 用时 " + dur) : ("用时 " + dur); },
      totalsMeta: function (files, dirs, bytes) {
        return files + " 个文件 · " + dirs + " 个目录 · 实占 " + bytes;
      },
      cancelledWarning: "扫描已提前取消，以下仅展示取消前已扫描到的部分结果。",
      skippedWarning: function (n) {
        return "有 " + n + " 个目录因权限受限或无法读取被跳过，结果可能不完整。";
      },
      truncatedNote: function (limit) {
        return "仅按实际磁盘占用展示前 " + limit + " 个文件。";
      },
      moreSkipped: function (n) { return "… 以及另外 " + n + " 个无法访问的目录"; },
      filesCount: function (n) { return n + " 项"; },
      emptyIdle: "尚未扫描目录",
      emptyScanning: "正在扫描文件…",
      emptyDone: "该目录中未找到占用本地空间的文件",
      revealInFinder: "在 Finder 中显示",
      revealFailed: "无法在 Finder 中显示该文件（可能已被移动或删除）。",
      scanRequestFailed: "无法开始扫描，请检查目录路径是否存在且有权限读取。",
      scanErrors: {
        invalid_root: "请输入有效的绝对路径或以 ~/ 开头的路径。",
        not_found: "找不到该目录，请确认路径是否存在。",
        not_dir: "该路径不是目录，请选择文件夹。",
        permission: "没有权限访问该目录。"
      },
      allocatedTitle: function (alloc, logical) {
        return "实际磁盘占用 " + alloc + "（逻辑大小 " + logical + "）";
      }
    },
    en: {
      diskUsed: function (used, avail) { return "Used " + used + ", available " + avail; },
      diskUnavailable: "Capacity unavailable",
      presets: {
        downloads: "Downloads",
        desktop: "Desktop",
        documents: "Documents",
        home: "Home"
      },
      states: {
        idle: "Ready",
        scanning: "Scanning",
        completed: "Completed",
        cancelled: "Cancelled",
        error: "Failed"
      },
      scan: "Scan",
      rescan: "Rescan",
      idleHint: "Choose a folder and click Scan to find large files.",
      scanningHint: function (files, dirs, bytes) {
        return "Scanned " + files + " files across " + dirs + " folders (" + bytes + ")…";
      },
      completedHint: function (count) {
        return count > 0 ? "Showing " + count + " largest files." : "Scan complete.";
      },
      cancelledHint: "Scan cancelled.",
      errorHint: "Unable to read the selected folder.",
      scopeMeta: function (root) { return "Scope: " + root; },
      timeMeta: function (at, dur) { return at ? ("Time: " + at + " · " + dur) : ("Duration: " + dur); },
      totalsMeta: function (files, dirs, bytes) {
        return files + " files · " + dirs + " folders · " + bytes + " on disk";
      },
      cancelledWarning: "Scan was cancelled early. Results below are partial.",
      skippedWarning: function (n) {
        return n + (n === 1 ? " folder" : " folders") + " could not be accessed and were skipped; results may be incomplete.";
      },
      truncatedNote: function (limit) {
        return "Showing the top " + limit + " files by allocated disk space.";
      },
      moreSkipped: function (n) { return "… and " + n + (n === 1 ? " more inaccessible folder" : " more inaccessible folders"); },
      filesCount: function (n) { return n + (n === 1 ? " item" : " items"); },
      emptyIdle: "No folder scanned yet",
      emptyScanning: "Scanning files…",
      emptyDone: "No files using local disk space were found",
      revealInFinder: "Show in Finder",
      revealFailed: "Unable to reveal this file in Finder (it may have been moved or removed).",
      scanRequestFailed: "Unable to start scan. Check that the folder exists and is readable.",
      scanErrors: {
        invalid_root: "Enter a valid absolute path or a path starting with ~/.",
        not_found: "Folder not found. Check that the path exists.",
        not_dir: "This path is not a folder. Choose a directory instead.",
        permission: "Permission denied for this folder."
      },
      allocatedTitle: function (alloc, logical) {
        return "Allocated " + alloc + " on disk (logical size " + logical + ")";
      }
    }
  };

  function messages() {
    var lang = (document.documentElement && document.documentElement.lang) || "";
    return lang.toLowerCase().indexOf("zh") === 0 ? MESSAGES.zh : MESSAGES.en;
  }

  function pct(v) { return Math.round(v) + "%"; }

  function diskSize(bytes) {
    if (bytes >= 1e12) return (bytes / 1e12).toFixed(2) + " TB";
    if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
    if (bytes >= 1e6) return (bytes / 1e6).toFixed(1) + " MB";
    if (bytes >= 1e3) return Math.round(bytes / 1e3) + " KB";
    return bytes + " B";
  }

  function formatDuration(ms) {
    if (!ms || ms < 1000) return Math.max(0, Math.round(ms || 0)) + " ms";
    if (ms < 60000) return (ms / 1000).toFixed(1) + "s";
    var totalSec = Math.round(ms / 1000);
    return Math.floor(totalSec / 60) + "m " + (totalSec % 60) + "s";
  }

  function pad2(n) { return n < 10 ? "0" + n : String(n); }

  function formatTime(ms) {
    if (!ms || ms <= 0) return "";
    var d = new Date(ms);
    return pad2(d.getHours()) + ":" + pad2(d.getMinutes()) + ":" + pad2(d.getSeconds());
  }

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  var timer = null;
  var inputInitialized = false;
  var lastScan = null;
  var lastPresets = [];
  var capabilities = { chooseDir: false, reveal: false };
  var clientError = "";
  var mutationEpoch = 0;
  var refreshSeq = 0;
  var choosingDir = false;

  function syncActionButtons(scanning) {
    $("startScanBtn").disabled = scanning || choosingDir;
    $("chooseDirBtn").disabled = scanning || choosingDir;
    $("cancelScanBtn").hidden = !scanning;
  }

  function isVisible() {
    return document.visibilityState !== "hidden" && !document.hidden;
  }

  function startPolling() {
    if (timer !== null || !isVisible()) return;
    timer = setInterval(refresh, 300);
  }

  function stopPolling() {
    if (timer === null) return;
    clearInterval(timer);
    timer = null;
  }

  function renderCapacity(d, msg) {
    var disk = d || {};
    var available = disk.total > 0;
    var usage = available ? Math.max(0, Math.min(100, disk.used / disk.total * 100)) : 0;
    $("detailDiskUsage").textContent = available ? pct(usage) : "–";
    $("detailDiskUsed").textContent = available ? diskSize(disk.used) : "–";
    $("detailDiskTotal").textContent = available ? diskSize(disk.total) : "–";
    $("detailDiskAvailable").textContent = available ? diskSize(disk.available) : "–";
    $("detailDiskFill").style.width = usage + "%";
    var meter = $("detailDiskMeter");
    meter.classList.toggle("low-space", available && usage >= 90);
    if (available) {
      meter.setAttribute("aria-valuenow", usage.toFixed(1));
      meter.setAttribute("aria-valuetext", msg.diskUsed(pct(usage), diskSize(disk.available)));
    } else {
      meter.removeAttribute("aria-valuenow");
      meter.setAttribute("aria-valuetext", msg.diskUnavailable);
    }
  }

  function syncPresetActive() {
    var current = ($("scanRootInput").value || "").trim();
    var container = $("scopePresets");
    for (var i = 0; i < container.children.length; i++) {
      var chip = container.children[i];
      chip.classList.toggle("active", chip.dataset && chip.dataset.path === current);
    }
  }

  function renderPresets(presets, msg) {
    lastPresets = presets || [];
    var container = $("scopePresets");
    container.textContent = "";
    for (var i = 0; i < lastPresets.length; i++) {
      (function (preset) {
        var label = msg.presets[preset.id] || preset.path;
        var btn = el("button", "preset-chip", label);
        btn.type = "button";
        btn.title = preset.path;
        btn.dataset.path = preset.path;
        btn.addEventListener("click", function () {
          clientError = "";
          $("scanRootInput").value = preset.path;
          inputInitialized = true;
          syncPresetActive();
          if (lastScan) renderScan(lastScan);
        });
        container.appendChild(btn);
      })(lastPresets[i]);
    }
    syncPresetActive();
  }

  function showRevealError(message) {
    var box = $("revealError");
    if (!message) {
      box.hidden = true;
      box.textContent = "";
      return;
    }
    box.textContent = message;
    box.hidden = false;
  }

  function revealFile(path) {
    showRevealError("");
    fetch("/api/disk/reveal", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: path })
    }).then(function (r) {
      if (!r.ok) {
        showRevealError(messages().revealFailed);
      }
    }).catch(function () {
      showRevealError(messages().revealFailed);
    });
  }

  function renderFiles(scan, msg) {
    var files = scan.files || [];
    var list = $("fileList");
    list.textContent = "";
    $("fileListMeta").textContent = scan.state === "idle" ? "–" : msg.filesCount(files.length);

    if (!files.length) {
      var emptyMsg = msg.emptyIdle;
      if (scan.state === "scanning") emptyMsg = msg.emptyScanning;
      else if (scan.state === "completed" || scan.state === "cancelled" || scan.state === "error") emptyMsg = msg.emptyDone;
      list.appendChild(el("div", "empty-state", emptyMsg));
      return;
    }

    var maxSize = files[0].size || 0;
    for (var i = 0; i < files.length; i++) {
      (function (file) {
        var item = el("div", "file-item");
        var top = el("div", "file-row-top");
        var name = el("span", "file-name", file.name);
        name.title = file.path;
        top.appendChild(name);

        var actions = el("span", "file-actions");
        var sizeEl = el("span", "file-size", diskSize(file.size));
        if (typeof file.logicalSize === "number" && file.logicalSize !== file.size) {
          sizeEl.title = msg.allocatedTitle(diskSize(file.size), diskSize(file.logicalSize));
        }
        actions.appendChild(sizeEl);

        if (capabilities.reveal) {
          var revealBtn = el("button", "reveal-btn", msg.revealInFinder);
          revealBtn.type = "button";
          revealBtn.title = file.path;
          revealBtn.addEventListener("click", function () {
            revealFile(file.path);
          });
          actions.appendChild(revealBtn);
        }
        top.appendChild(actions);
        item.appendChild(top);

        var dirEl = el("div", "file-dir", file.dir);
        dirEl.title = file.dir;
        item.appendChild(dirEl);

        var bar = el("span", "file-bar");
        var fill = el("span", "file-bar-fill");
        var width = maxSize > 0 ? Math.max(2, Math.min(100, file.size / maxSize * 100)) : 0;
        fill.style.width = width.toFixed(1) + "%";
        bar.appendChild(fill);
        item.appendChild(bar);

        list.appendChild(item);
      })(files[i]);
    }
  }

  function renderScan(scan) {
    lastScan = scan;
    var msg = messages();
    var state = scan.state || "idle";
    var scanning = state === "scanning";
    var files = scan.files || [];

    syncActionButtons(scanning);
    $("startScanBtn").textContent = (state === "completed" || state === "cancelled" || state === "error") ? msg.rescan : msg.scan;

    var badge = $("scanStatusBadge");
    badge.textContent = msg.states[state] || state;
    badge.className = "status-badge status-" + state;

    var statusCard = $("scanStatusCard");
    statusCard.classList.toggle("status-scanning", scanning);

    var hint = msg.idleHint;
    if (state === "scanning") {
      hint = msg.scanningHint(scan.scannedFiles || 0, scan.scannedDirs || 0, diskSize(scan.scannedBytes || 0));
    } else if (state === "completed") {
      hint = msg.completedHint(files.length);
    } else if (state === "cancelled") {
      hint = msg.cancelledHint;
    } else if (state === "error") {
      hint = msg.errorHint;
    }
    $("scanStatusText").textContent = hint;

    var metaRow = $("scanMetaRow");
    if (state === "idle") {
      metaRow.hidden = true;
    } else {
      metaRow.hidden = false;
      $("scanScopeMeta").textContent = msg.scopeMeta(scan.root || "");
      $("scanTimeMeta").textContent = msg.timeMeta(formatTime(scan.finishedAt || scan.startedAt), formatDuration(scan.elapsedMs));
      $("scanTotalsMeta").textContent = msg.totalsMeta(scan.scannedFiles || 0, scan.scannedDirs || 0, diskSize(scan.scannedBytes || 0));
    }

    var warningParts = [];
    if (clientError) {
      warningParts.push(clientError);
    } else if (state === "error") {
      if (scan.error) warningParts.push(scan.error);
    } else {
      if (state === "cancelled") {
        warningParts.push(msg.cancelledWarning);
      }
      if ((scan.skippedDirs || 0) > 0) {
        warningParts.push(msg.skippedWarning(scan.skippedDirs));
      } else if (scan.incomplete && state !== "cancelled") {
        warningParts.push(msg.skippedWarning(1));
      }
      if (scan.truncated) {
        warningParts.push(msg.truncatedNote(scan.limit || 100));
      }
    }

    var warningBox = $("scanWarning");
    var skippedList = $("skippedList");
    skippedList.textContent = "";
    if (!warningParts.length) {
      warningBox.hidden = true;
      skippedList.hidden = true;
    } else {
      warningBox.hidden = false;
      $("scanWarningText").textContent = warningParts.join(" ");
      var samples = clientError ? [] : (scan.skippedSamples || []);
      if (!samples.length) {
        skippedList.hidden = true;
      } else {
        skippedList.hidden = false;
        for (var i = 0; i < samples.length; i++) {
          skippedList.appendChild(el("li", "", samples[i]));
        }
        var extra = (scan.skippedDirs || 0) - samples.length;
        if (extra > 0) {
          skippedList.appendChild(el("li", "", msg.moreSkipped(extra)));
        }
      }
    }

    renderFiles(scan, msg);

    if (scanning && isVisible()) {
      startPolling();
    } else {
      stopPolling();
    }
  }

  function render(data) {
    var msg = messages();
    capabilities = (data && data.capabilities) || { chooseDir: false, reveal: false };
    $("chooseDirBtn").hidden = !capabilities.chooseDir;

    renderCapacity(data && data.disk, msg);

    var scan = (data && data.scan) || { state: "idle", files: [], skippedSamples: [] };
    if (!inputInitialized) {
      var initial = scan.root || (data && data.defaultRoot) || "";
      if (initial) {
        $("scanRootInput").value = initial;
        inputInitialized = true;
      }
    }
    renderPresets((data && data.presets) || [], msg);
    renderScan(scan);
  }

  function refresh() {
    var seq = ++refreshSeq;
    var epoch = mutationEpoch;
    return fetch("/api/disk", { cache: "no-store" })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (seq !== refreshSeq || epoch !== mutationEpoch) return;
        var incoming = (data && data.scan) || null;
        if (incoming && lastScan && incoming.id && lastScan.id && incoming.id < lastScan.id) {
          return;
        }
        render(data);
      })
      .catch(function () {});
  }

  function localizeScanError(r) {
    return r.text().then(function (text) {
      var msg = messages();
      try {
        var parsed = JSON.parse(text);
        if (parsed && parsed.code && msg.scanErrors[parsed.code]) {
          return msg.scanErrors[parsed.code];
        }
      } catch (_) {}
      return msg.scanRequestFailed;
    });
  }

  function startScan(rootPath) {
    stopPolling();
    clientError = "";
    showRevealError("");
    var epoch = ++mutationEpoch;
    var target = (rootPath !== undefined ? rootPath : $("scanRootInput").value || "").trim();
    return fetch("/api/disk/scan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ root: target })
    }).then(function (r) {
      if (epoch !== mutationEpoch) return;
      if (!r.ok) {
        return localizeScanError(r).then(function (errMsg) {
          if (epoch !== mutationEpoch) return;
          clientError = errMsg;
          if (lastScan) renderScan(lastScan);
        });
      }
      return r.json().then(function (scan) {
        if (epoch !== mutationEpoch) return;
        if (scan && scan.root) {
          $("scanRootInput").value = scan.root;
          inputInitialized = true;
          syncPresetActive();
        }
        renderScan(scan);
      });
    }).catch(function () {
      if (epoch !== mutationEpoch) return;
      clientError = messages().scanRequestFailed;
      if (lastScan) renderScan(lastScan);
    });
  }

  $("startScanBtn").addEventListener("click", function () {
    startScan();
  });

  $("scanRootInput").addEventListener("keydown", function (e) {
    if (e.key === "Enter") {
      e.preventDefault();
      if (!lastScan || lastScan.state !== "scanning") {
        startScan();
      }
    }
  });

  $("scanRootInput").addEventListener("input", function () {
    inputInitialized = true;
    clientError = "";
    syncPresetActive();
  });

  $("cancelScanBtn").addEventListener("click", function () {
    stopPolling();
    var epoch = ++mutationEpoch;
    fetch("/api/disk/cancel", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}"
    }).then(function (r) { return r.json(); })
      .then(function (scan) {
        if (epoch !== mutationEpoch) return;
        renderScan(scan);
      })
      .catch(function () {});
  });

  $("chooseDirBtn").addEventListener("click", function () {
    if (choosingDir) return;
    choosingDir = true;
    syncActionButtons(lastScan && lastScan.state === "scanning");
    var initial = ($("scanRootInput").value || "").trim();
    fetch("/api/disk/choose", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ initial: initial })
    }).then(function (r) {
      if (!r.ok) return null;
      return r.json();
    }).then(function (res) {
      choosingDir = false;
      syncActionButtons(lastScan && lastScan.state === "scanning");
      if (res && !res.cancelled && res.path) {
        $("scanRootInput").value = res.path;
        inputInitialized = true;
        syncPresetActive();
        startScan(res.path);
      }
    }).catch(function () {
      choosingDir = false;
      syncActionButtons(lastScan && lastScan.state === "scanning");
    });
  });

  document.addEventListener("visibilitychange", function () {
    if (isVisible()) {
      refresh();
    } else {
      stopPolling();
    }
  });
  window.addEventListener("pagehide", stopPolling);
  window.addEventListener("beforeunload", stopPolling);

  refresh();
})();
