import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
from urllib.request import build_opener, ProxyHandler
from playwright.sync_api import sync_playwright, expect

runtime = Path("ia-audit-runtime").resolve()
exe = Path("audit-data/IA-Locale-Studio.exe").resolve()
ollama = next(Path("audit-data/ollama").rglob("ollama.exe")).resolve()
opener = build_opener(ProxyHandler({}))
env = dict(os.environ, OLLAMA_HOST="127.0.0.1:11434",
           OLLAMA_MODELS=str(runtime / "models"), TMPDIR=str(runtime / "tmp"),
           OLLAMA_LLM_LIBRARY="cpu_avx2", OLLAMA_NO_CLOUD="1", CUDA_VISIBLE_DEVICES="-1")

def wait_json(url, validate, process):
    for attempt in range(120):
        if process.poll() is not None:
            raise RuntimeError(f"Processus arrêté avec le code {process.returncode}")
        try:
            with opener.open(url, timeout=1) as response:
                value = json.load(response)
            if validate(value):
                return value
        except OSError:
            pass
        time.sleep(0.25)
    raise AssertionError(f"État attendu absent sur {url}")

def studio(root, log):
    process = subprocess.Popen([str(exe), "-no-browser", "-port", "0", "-root", root],
                               stdout=subprocess.PIPE, stderr=log, text=True)
    address = re.search(r"http://localhost:\d+/", process.stdout.readline())
    if not address:
        process.wait(timeout=10)
        raise RuntimeError("L'interface locale ne s'est pas lancée")
    return process, address.group().replace("localhost", "127.0.0.1")

with (runtime / "ollama.log").open("w") as engine_log, (runtime / "studio.log").open("w") as app_log:
    server = subprocess.Popen([str(ollama), "serve"], env=env,
                              stdout=engine_log, stderr=subprocess.STDOUT)
    app = None
    try:
        base = "http://127.0.0.1:11434"
        models = wait_json(base + "/api/tags", lambda x: bool(x["models"]), server)
        assert len(models["models"]) == 1, models
        assert models["models"][0]["details"]["format"] == "gguf", models
        print("Modèle GGUF réel détecté par Ollama", flush=True)
        with tempfile.TemporaryDirectory(dir=runtime) as data, sync_playwright() as p:
            app, url = studio(data, app_log)
            browser = p.chromium.launch()
            page = browser.new_page(accept_downloads=True)
            errors = []
            page.on("pageerror", lambda error: errors.append(str(error)))
            external = []
            def block_external(route):
                if route.request.url.startswith("http://127.0.0.1:"):
                    route.continue_()
                else:
                    external.append(route.request.url)
                    route.abort()
            page.route("**/*", block_external)
            page.goto(url)
            expect(page.locator("#models option", has_text="smollm:135m")).to_have_count(1, timeout=20000)
            for kind, prompt in [("Texte", "Reply in English with one short sentence about a blue bicycle."),
                                 ("Code", "Write a small Python function that adds two numbers.")]:
                page.get_by_role("button", name=kind, exact=True).click()
                if not page.locator("#tokens").is_visible():
                    page.locator("#text-settings summary").click()
                page.locator("#tokens").select_option("256")
                page.locator("#context").select_option("2048")
                page.locator("#prompt").fill(prompt)
                with page.expect_response(lambda r: r.url.endswith("/api/generate") and r.request.method == "POST") as reply:
                    page.locator("#generate").click()
                started = reply.value
                assert started.status == 202, started.text()
                job = started.json()["id"]
                artifact = Path(data, "data", "outputs", job, "reponse.txt")
                until = time.monotonic() + 150
                while not artifact.is_file() and time.monotonic() < until:
                    page.wait_for_timeout(250)
                assert artifact.is_file(), page.locator("#result").text_content()
                page.locator("#files button").first.wait_for(timeout=10000)
                expect(page.locator("#state")).to_have_text("Terminé")
                output = page.locator("#result").text_content()
                assert artifact.read_text() == output, (repr(artifact.read_text()), repr(output))
                assert len(output.strip()) > 4, output
                with page.expect_download() as captured:
                    page.locator("#files button").first.click()
                download = captured.value
                assert download.suggested_filename == "reponse.txt"
                assert Path(download.path()).read_text() == output
                wait_json(base + "/api/ps", lambda x: not x.get("models"), server)
                print(f"PASS {kind} : {len(output)} caractères, téléchargement exact, déchargement du modèle", flush=True)
            assert not errors, errors
            assert not external, external
            outputs = list(Path(data, "data", "outputs").glob("*/conversation.json"))
            assert sorted(json.loads(path.read_text())["kind"] for path in outputs) == ["code", "texte"]
            app.terminate()
            app.wait(timeout=10)
            app, url = studio(data, app_log)
            page.goto(url)
            expect(page.locator("#outputs button")).to_have_count(4, timeout=15000)
            print("PASS : les deux conversations et réponses restent accessibles après redémarrage, aucune requête web externe", flush=True)
            browser.close()
    finally:
        if app and app.poll() is None:
            app.terminate()
            app.wait(timeout=10)
        server.terminate()
        server.wait(timeout=15)
