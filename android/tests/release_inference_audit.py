import re
import shlex
import subprocess
import time
import xml.etree.ElementTree as ET

PACKAGE = "fr.ialocale.mobile"
REMOTE_XML = "/sdcard/ia-audit.xml"

def adb(*args):
    return subprocess.check_output(["adb", *args], text=True, timeout=40)

def wait_node(text, timeout=35, *, attribute="text"):
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        adb("shell", "uiautomator", "dump", REMOTE_XML)
        root = ET.fromstring(adb("shell", "cat", REMOTE_XML))
        for node in root.iter("node"):
            if node.get("package") == PACKAGE and node.get(attribute) == text:
                return node
        time.sleep(0.4)
    raise AssertionError(f"État absent : {text}")

def tap(node):
    x1, y1, x2, y2 = map(int, re.findall(r"\d+", node.get("bounds")))
    adb("shell", "input", "tap", str((x1+x2)//2), str((y1+y2)//2))

def write_private(name, content):
    command = f"run-as {PACKAGE} sh -c {shlex.quote('cat > ' + name)}"
    subprocess.run(["adb", "shell", "-T", command], stdin=content, check=True, timeout=100)

adb("install", "audit-data/IA-Locale-Studio.apk")
adb("shell", "wm", "size", "1080x2160")
adb("shell", "wm", "density", "280")
adb("shell", "svc", "wifi", "disable")
adb("shell", "svc", "data", "disable")
adb("shell", "run-as", PACKAGE, "mkdir", "-p", "files/models", "shared_prefs")
with open("audit-data/model.gguf", "rb") as model:
    write_private("files/models/audit.gguf", model)
prefs = b'<map><string name="model">audit.gguf</string></map>'
subprocess.run(["adb", "shell", "-T", f"run-as {PACKAGE} sh -c 'cat > shared_prefs/MainActivity.xml'"],
               input=prefs, check=True, timeout=30)
for kind, prompt in [("Texte", "Say hello in one sentence"), ("Code", "Write a short Python function")]:
    adb("shell", "am", "force-stop", PACKAGE)
    start = adb("shell", "am", "start", "-W", "-n", f"{PACKAGE}/.MainActivity")
    assert "Status: ok" in start, start
    wait_node("Hors ligne · modèles installés utilisables")
    tap(wait_node(kind))
    assert wait_node(kind).get("selected") == "true"
    tap(wait_node("android.widget.EditText", attribute="class"))
    adb("shell", "input", "text", prompt.replace(" ", "%s"))
    action = wait_node("Générer localement")
    assert action.get("enabled") == "true"
    tap(action)
    wait_node("Génération terminée · modèle déchargé.", 150)
    print(f"PASS {kind} : vraie génération GGUF hors ligne avec l’APK publié", flush=True)
