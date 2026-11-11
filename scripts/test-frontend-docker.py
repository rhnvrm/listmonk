#!/usr/bin/env python3
"""Smoke-test frontend builds and watch mode with real Compose volumes."""

import os
from pathlib import Path
import subprocess
import uuid


ROOT = Path(__file__).resolve().parents[1]
COMPOSE = [
    "docker", "compose", "-f", str(ROOT / "dev/docker-compose.yml"),
    "-p", "listmonk-smoke-" + uuid.uuid4().hex[:12],
]


def compose(*args, **kwargs):
    return subprocess.run(COMPOSE + list(args), cwd=ROOT, check=True, **kwargs)


def python_in_container(source):
    compose("exec", "-T", "dev", "python3", "-", input=source, text=True)


def start_app():
    compose("exec", "-T", "dev", "./listmonk", "--config=dev/config.toml",
            "--install", "--idempotent", "--yes")
    compose("exec", "-T", "-d", "dev", "sh", "-c",
            "exec ./listmonk --config=dev/config.toml > /tmp/listmonk-smoke.log 2>&1")
    python_in_container('''
import gzip, time, urllib.request
for attempt in range(60):
    try:
        with urllib.request.urlopen("http://127.0.0.1:9000/admin/login", timeout=2) as response:
            assert response.status == 200
            break
    except (OSError, AssertionError):
        time.sleep(1)
else:
    raise SystemExit("Listmonk did not serve its login page")
request = urllib.request.Request("http://127.0.0.1:9000/admin/static/js/main.js",
                                 headers={"Accept-Encoding": "gzip"})
with urllib.request.urlopen(request, timeout=10) as response:
    assert response.status == 200
    assert response.headers.get("Content-Encoding") == "gzip"
    assert gzip.decompress(response.read())
''')


def main():
    try:
        compose("up", "-d", "--build", "--wait", "--wait-timeout", "180", "dev", "mailhog")
        # The project name is unique: both dependency and output volumes start empty.
        compose("exec", "-T", "dev", "make", "dist")
        python_in_container('''
import gzip, json, os
from pathlib import Path
root = Path("static/admin/dist")
assert Path("static/admin/vendor/node_modules/.installed").is_file()
assert Path("static/public/static/altcha.umd.js").is_file()
assert (root / "js/main.js.gz").is_file()
for path in root.rglob("*.gz"):
    assert gzip.decompress(path.read_bytes())
    assert not path.with_suffix("").exists(), path
Path("/tmp/frontend-root.json").write_text(json.dumps([root.stat().st_dev, root.stat().st_ino]))
''')
        start_app()
        compose("exec", "-T", "-d", "dev", "sh", "-c",
                "exec make watch-frontend > /tmp/frontend-watch.log 2>&1")
        python_in_container('''
import gzip, json, time
from pathlib import Path
root = Path("static/admin/dist")
source = Path("static/admin/assets/js/views/esgun-docker-smoke.js")
output = root / "js/views/esgun-docker-smoke.js.gz"
def await_output(predicate):
    for attempt in range(300):
        try:
            if predicate():
                return
        except (OSError, EOFError):
            pass
        time.sleep(0.1)
    raise AssertionError("watch did not update output")
try:
    source.write_text('console.log("docker-smoke-first");')
    await_output(lambda: b"docker-smoke-first" in gzip.decompress(output.read_bytes()))
    source.write_text('console.log("docker-smoke-second");')
    await_output(lambda: b"docker-smoke-second" in gzip.decompress(output.read_bytes()))
finally:
    source.unlink(missing_ok=True)
await_output(lambda: not output.exists())
assert [root.stat().st_dev, root.stat().st_ino] == json.loads(Path("/tmp/frontend-root.json").read_text())
''')
        compose("restart", "dev")
        compose("exec", "-T", "dev", "make", "build-frontend")
        python_in_container('''
import json
from pathlib import Path
root = Path("static/admin/dist")
assert [root.stat().st_dev, root.stat().st_ino] == json.loads(Path("/tmp/frontend-root.json").read_text())
assert Path("static/admin/vendor/node_modules/.installed").is_file()
assert (root / "js/main.js.gz").is_file()
''')
        start_app()
        print("PASS: clean build, app startup, watch create/edit/delete, volume identity, restart", flush=True)
    finally:
        # Diagnostics precede teardown, including when a build or assertion fails.
        subprocess.run(COMPOSE + ["logs", "--no-color"], cwd=ROOT, check=False)
        subprocess.run(COMPOSE + ["exec", "-T", "dev", "sh", "-c",
                       "cat /tmp/frontend-watch.log /tmp/listmonk-smoke.log 2>/dev/null; "
                       "rm -f static/admin/assets/js/views/esgun-docker-smoke.js"], cwd=ROOT, check=False)
        compose("down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    os.chdir(ROOT)
    main()
