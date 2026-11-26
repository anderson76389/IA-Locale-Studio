"""Smoke test of the compiled Windows/Linux server; no model or network required."""
import json, pathlib, re, socket, subprocess, sys, tempfile, time, urllib.request, urllib.error
from html.parser import HTMLParser

class Page(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = []
        self.kinds = []
    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if "id" in a: self.ids.append(a["id"])
        if tag == "button" and "data-kind" in a: self.kinds.append(a["data-kind"])

binary = pathlib.Path(sys.argv[1]).resolve(strict=True)
with tempfile.TemporaryDirectory() as tmp:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        dead = sock.getsockname()[1]
    endpoint = f"http://127.0.0.1:{dead}"
    p = subprocess.Popen([str(binary), "-no-browser", "-root", tmp, "-port", str(port), "-ollama", endpoint, "-comfy", endpoint], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    base = f"http://127.0.0.1:{port}/"
    # Bypass any proxy configured by the runner.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    def request(path="", body=None, token=None):
        headers = {}
        if token: headers["X-Studio-Token"] = token
        if body is not None: headers["Content-Type"] = "application/json"
        req = urllib.request.Request(base + path, data=json.dumps(body).encode() if body is not None else None, headers=headers)
        try:
            with opener.open(req, timeout=20) as resp: return resp.status, resp.read(), resp.headers
        except urllib.error.HTTPError as e: return e.code, e.read(), e.headers
    try:
        for _ in range(100):
            if p.poll() is not None: raise RuntimeError("Server terminated")
            try:
                status, body, headers = request()
                break
            except OSError: time.sleep(.1)
        else: raise RuntimeError("Server did not start")
        assert status == 200 and "Content-Security-Policy" in headers
        page = Page()
        page.feed(body.decode())
        assert len(page.ids) == len(set(page.ids))
        assert page.kinds == ["texte", "code", "image", "video", "musique"]
        status, js, _ = request("app.js")
        assert status == 200 and set(re.findall(r"\$\('([^']+)'\)", js.decode())) <= set(page.ids)
        assert request("style.css")[0] == 200
        status, body, h = request("api/session")
        assert status == 200 and "HttpOnly" in h["Set-Cookie"]
        token = json.loads(body)["token"]
        assert request("api/status")[0] == 403
        status, body, _ = request("api/status", token=token)
        v = json.loads(body)
        assert status == 200 and v["ollama_error"] and v["comfy_error"] and not v["models"]
        assert request("api/generate", {"kind":"texte","prompt":"test","model":"local:1","context":2048,"tokens":128}, token)[0] == 202
        for _ in range(100):
            job = json.loads(request("api/job", token=token)[1])
            if job["state"] not in ("running", "stopping"): break
            time.sleep(.1)
        assert job["state"] == "error" and not job["files"]
        assert request("api/backup", {}, token)[0] == 202
        for _ in range(100):
            job = json.loads(request("api/job", token=token)[1])
            if job["state"] not in ("running", "stopping"): break
            time.sleep(.1)
        assert job["state"] == "done" and (pathlib.Path(tmp)/"backups"/(job["id"]+".zip")).is_file()
        print("PASS: actual executable, five sections, API protection, absent engines, no fake success, backup")
    finally:
        p.terminate()
        try: p.wait(timeout=5)
        except subprocess.TimeoutExpired: p.kill(); p.wait(timeout=5)
