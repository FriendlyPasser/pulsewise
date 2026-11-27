"""Exercise the real browser host, API, embedded assets, sampling, and shutdown."""

from html.parser import HTMLParser

import json
import math
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


class DetailNavigation(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = {}
        self.in_nav = False

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "nav":
            self.in_nav = True
        elif self.in_nav and tag == "a":
            self.links[attrs["href"]] = attrs.get("aria-current")

    def handle_endtag(self, tag):
        if tag == "nav":
            self.in_nav = False


def check(binary):
    with tempfile.TemporaryFile(mode="w+") as log:
        process = subprocess.Popen(
            [binary, "-web", "127.0.0.1:0"], stdout=log, stderr=log
        )
        try:
            deadline = time.monotonic() + 10
            base = None
            while time.monotonic() < deadline:
                log.seek(0)
                match = re.search(r"serving the panel at (http://\S+)", log.read())
                if match:
                    base = match[1]
                    break
                if process.poll() is not None:
                    raise AssertionError("browser host exited before listening")
                time.sleep(0.02)
            assert base, "browser host did not start"

            def request(path, body=None):
                data = None if body is None else json.dumps(body).encode()
                req = urllib.request.Request(
                    base + path, data=data,
                    headers={"Content-Type": "application/json"},
                )
                return urllib.request.urlopen(req, timeout=3)

            def snapshot():
                with request("/api/state") as response:
                    assert response.headers["Cache-Control"] == "no-store"
                    return json.load(response)

            first = snapshot()
            assert first["at"] > 0
            assert first["memory"]["total"] > 0
            assert 0 <= first["cpu"]["usage"] <= 100
            for key in ("cpu", "gpu", "memory", "network"):
                assert 1 <= len(first[key]["history"]) <= 17

            def check_network(state):
                net = state["network"]
                assert math.isfinite(net["down"]) and net["down"] >= 0
                assert math.isfinite(net["up"]) and net["up"] >= 0
                hist = net["history"]
                assert 1 <= len(hist) <= 17
                for pt in hist:
                    assert math.isfinite(pt["down"]) and pt["down"] >= 0
                    assert math.isfinite(pt["up"]) and pt["up"] >= 0
                assert hist[-1]["down"] == net["down"]
                assert hist[-1]["up"] == net["up"]

            def check_disk(state):
                disk = state["disk"]
                assert disk["total"] > 0, "system disk capacity is unavailable"
                assert 0 <= disk["available"] <= disk["total"]
                assert disk["used"] == disk["total"] - disk["available"]
                # Compare the API with an independent OS read. Allow space to
                # change while the two calls run, for example during a download.
                try:
                    capacity = os.statvfs("/System/Volumes/Data")
                except FileNotFoundError:
                    capacity = os.statvfs("/")
                assert disk["total"] == capacity.f_blocks * capacity.f_frsize
                available = capacity.f_bavail * capacity.f_frsize
                assert abs(disk["available"] - available) < 256 * 1024 * 1024

            check_disk(first)
            check_network(first)

            deadline = time.monotonic() + 4
            while time.monotonic() < deadline:
                latest = snapshot()
                if latest["at"] > first["at"]:
                    break
                time.sleep(0.05)
            else:
                raise AssertionError("the sampler did not publish a new snapshot")
            check_disk(latest)
            check_network(latest)
            assert len(latest["network"]["history"]) >= 2

            with request("/api/update") as response:
                assert json.load(response)["state"] == "off"
            for path, expected in (
                ("/", b"cpuValue"), ("/memory", b"appList"),
                ("/memory.html", b"unattributedList"), ("/disk", b"fileList"),
                ("/disk.html", b"scanRootInput"), ("/app.js", b"/api/state"),
                ("/memory.js", b"/api/memory"), ("/disk.js", b"/api/disk"),
                ("/app.css", b"--panel"),
            ):
                with request(path) as response:
                    asset = response.read()
                    assert expected in asset, path
                    if path == "/":
                        assert b'diskValue' in asset
                        assert b'diskMeter' in asset
                        assert b'diskMetric' in asset
                        assert b'memMetric' in asset
                        assert b'netDown' in asset
                        assert b'netUp' in asset
                        assert b'netDownSpark' in asset
                        assert b'netUpSpark' in asset
                        assert b'netPeak' in asset
                    if path in ("/memory.html", "/disk.html"):
                        assert asset.count(b'class="logo"') == 1
                        assert re.search(rb'<header class="brand detail-titlebar">.*?class="logo".*?>Pulsewise</span>\s*</header>', asset, re.S)
                        assert re.search(rb'<header class="detail-head">\s*<h1>[^<]+</h1>', asset)
                    if path in ("/memory", "/memory.html"):
                        assert b"detailMemArea" in asset, path

            path = "/memory"
            for current, other in (("memory", "disk"), ("disk", "memory"), ("memory", "disk")):
                with request(path) as response:
                    nav = DetailNavigation()
                    nav.feed(response.read().decode())
                    assert nav.links == {current + ".html": "page", other + ".html": None}
                    path = "/" + next(href for href, active in nav.links.items() if active is None)

            with request("/api/memory") as response:
                assert response.headers["Cache-Control"] == "no-store"
                detail = json.load(response)
            assert detail["at"] > 0
            assert detail["memory"]["total"] > 0
            assert 1 <= len(detail["memory"]["history"]) <= 17
            apps = detail["apps"]
            unattributed = detail["unattributed"]
            assert len(apps) + len(unattributed) > 0
            for i in range(len(apps) - 1):
                assert apps[i]["memory"] >= apps[i + 1]["memory"]
            for app in apps:
                assert app["name"]
                procs = app["processes"]
                assert len(procs) > 0
                assert sum(p["memory"] for p in procs) == app["memory"]
                for i in range(len(procs) - 1):
                    assert procs[i]["memory"] >= procs[i + 1]["memory"]
            assert sum(p["memory"] for p in unattributed) == detail["unattributedMemory"]
            for i in range(len(unattributed) - 1):
                assert unattributed[i]["memory"] >= unattributed[i + 1]["memory"]

            with request("/api/memory/open", {}) as response:
                assert response.status == 204
            with request("/api/disk/open", {}) as response:
                assert response.status == 204

            with request("/api/disk") as response:
                assert response.headers["Cache-Control"] == "no-store"
                disk_detail = json.load(response)
            assert disk_detail["capabilities"]["chooseDir"] is False
            assert disk_detail["capabilities"]["reveal"] is False
            assert disk_detail["scan"]["state"] == "idle"

            with tempfile.TemporaryDirectory() as scan_dir:
                big_path = os.path.join(scan_dir, "big.bin")
                small_path = os.path.join(scan_dir, "small.bin")
                with open(big_path, "wb") as fh:
                    fh.write(b"x" * (64 * 1024))
                with open(small_path, "wb") as fh:
                    fh.write(b"y" * (16 * 1024))

                with request("/api/disk/scan", {"root": scan_dir}) as response:
                    assert response.status == 200
                    started_scan = json.load(response)
                assert started_scan["root"] == scan_dir

                scan_deadline = time.monotonic() + 5
                completed_scan = None
                while time.monotonic() < scan_deadline:
                    with request("/api/disk") as response:
                        completed_scan = json.load(response)["scan"]
                    if completed_scan["state"] != "scanning":
                        break
                    time.sleep(0.05)
                assert completed_scan and completed_scan["state"] == "completed", completed_scan
                assert completed_scan["root"] == scan_dir
                assert completed_scan["scannedFiles"] == 2
                files = completed_scan["files"]
                assert len(files) == 2
                assert files[0]["name"] == "big.bin" and files[1]["name"] == "small.bin"
                assert files[0]["size"] > files[1]["size"] > 0

                try:
                    request("/api/disk/reveal", {"path": "/etc/hosts"})
                    raise AssertionError("out-of-root reveal was accepted")
                except urllib.error.HTTPError as error:
                    assert error.code == 400
                    error.close()

                try:
                    request("/api/disk/reveal", {"path": big_path})
                    raise AssertionError("browser host revealed file")
                except urllib.error.HTTPError as error:
                    assert error.code == 501
                    error.close()

            try:
                request("/api/disk/choose", {})
                raise AssertionError("browser host opened native folder picker")
            except urllib.error.HTTPError as error:
                assert error.code == 501
                error.close()

            with request("/api/disk/cancel", {}) as response:
                assert response.status == 200

            with request("/api/fit", {"height": 440}) as response:
                assert response.status == 204
            try:
                request("/api/fit", {"height": 0})
                raise AssertionError("invalid panel height was accepted")
            except urllib.error.HTTPError as error:
                assert error.code == 400
                error.close()
            with request("/api/quit", {}) as response:
                assert response.status == 204
            assert process.wait(timeout=5) == 0
            print("Browser smoke check passed: live samples, disk capacity matches OS, assets, API, and quit.")
        except BaseException:
            log.seek(0)
            print(log.read(), file=sys.stderr)
            raise
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    check(sys.argv[1])
