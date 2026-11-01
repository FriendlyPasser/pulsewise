"""Exercise the real browser host, API, embedded assets, sampling, and shutdown."""

import json
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


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
            for key in ("cpu", "gpu", "memory"):
                assert 1 <= len(first[key]["history"]) <= 17

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

            deadline = time.monotonic() + 4
            while time.monotonic() < deadline:
                latest = snapshot()
                if latest["at"] > first["at"]:
                    break
                time.sleep(0.05)
            else:
                raise AssertionError("the sampler did not publish a new snapshot")
            check_disk(latest)

            with request("/api/update") as response:
                assert json.load(response)["state"] == "off"
            for path, expected in (
                ("/", b"cpuValue"), ("/app.js", b"/api/state"),
                ("/app.css", b"--panel"),
            ):
                with request(path) as response:
                    asset = response.read()
                    assert expected in asset, path
                    if path == "/":
                        assert b'diskValue' in asset
                        assert b'diskMeter' in asset
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
