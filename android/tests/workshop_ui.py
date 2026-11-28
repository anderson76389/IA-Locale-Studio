"""Exercise the published APK's native workshop navigation on a clean emulator."""
import re
import subprocess
import sys
import time
import xml.etree.ElementTree as ET

PACKAGE = "fr.ialocale.mobile"
DUMP = "/sdcard/ia-workshops.xml"


def adb(*args):
    return subprocess.check_output(["adb", *args], encoding="utf-8", timeout=35)


def view():
    adb("shell", "uiautomator", "dump", DUMP)
    root = ET.fromstring(adb("shell", "cat", DUMP))
    return {node.get("text"): node.attrib for node in root.iter("node")
            if node.get("package") == PACKAGE and node.get("text")}


def expect_workshop(active):
    inactive = "Code" if active == "Texte" else "Texte"
    for _ in range(5):
        nodes = view()
        if nodes.get(active, {}).get("selected") == "true":
            assert nodes[inactive]["selected"] == "false"
            assert all(nodes[name]["enabled"] == "true" for name in (active, inactive))
            return nodes
        time.sleep(0.5)
    raise AssertionError(f"Atelier {active} non sélectionné. Nœuds visibles : {list(nodes)}")


def tap(node):
    left, top, right, bottom = map(int, re.findall(r"\d+", node["bounds"]))
    adb("shell", "input", "tap", str((left + right) // 2), str((top + bottom) // 2))


adb("install", sys.argv[1])
launch = adb("shell", "am", "start", "-W", "-a", "android.intent.action.MAIN",
    "-c", "android.intent.category.LAUNCHER", "-f", "0x10200000", "-n", f"{PACKAGE}/.MainActivity")
assert "Status: ok" in launch, launch
first = view()
tap(first["Choisir un modèle ▾"])
assert "Importer un modèle GGUF" in view()
adb("shell","input","keyevent","4")
tap(view()["☰"])
tap(expect_workshop("Texte")["Code"])
tap(view()["☰"])
tap(expect_workshop("Code")["Texte"])
tap(view()["☰"])
expect_workshop("Texte")
adb("shell", "input", "keyevent", "4")
print("PASS : installation, démarrage et sélection native Texte → Code → Texte")

# Exercise the same offline generation button as an installed user. The model is
# provisioned before launch so this test can run unattended on a clean emulator.
import pathlib
import struct
import urllib.request

model = pathlib.Path("smollm2-media-test.gguf")
url = ("https://huggingface.co/bartowski/SmolLM2-135M-Instruct-GGUF/resolve/"
       "09816acd5d99df7be770d85ea30822623dab342c/SmolLM2-135M-Instruct-Q4_K_M.gguf")
urllib.request.urlretrieve(url, model)
assert model.stat().st_size > 50000000
adb("shell", "am", "force-stop", PACKAGE)
adb("push", str(model), "/data/local/tmp/test.gguf")
def private(*args):
    return adb("shell", "run-as", PACKAGE, *args)
private("mkdir", "-p", "files/models", "shared_prefs")
private("cp", "/data/local/tmp/test.gguf", "files/models/test.gguf")
prefs = pathlib.Path("studio-test-preferences.xml")
prefs.write_text('<map><string name="model">test.gguf</string></map>')
adb("push", str(prefs), "/data/local/tmp/studio-test.xml")
private("cp", "/data/local/tmp/studio-test.xml", "shared_prefs/MainActivity.xml")
adb("shell", "am", "start", "-W", "-a", "android.intent.action.MAIN",
    "-c", "android.intent.category.LAUNCHER", "-f", "0x10200000", "-n", f"{PACKAGE}/.MainActivity")
adb("shell","wm","size","1080x2400")
adb("shell","wm","density","320")
for category, ext in [("Image","png"),("Vidéo","mp4"),("Musique","wav")]:
    tap(view()["☰"])
    tap(view()[category])
    nodes=view()
    model_button=next(node for node in nodes.values() if node.get("content-desc")=="Modèles locaux")
    tap(model_button)
    tap(view()["Tester hors ligne"])
    until=time.monotonic()+300
    while time.monotonic()<until:
        paths=private("find","files/creations","-name",f"creation.{ext}") if "creations" in private("ls","files") else ""
        if "Création enregistrée" in str(view()) and paths.strip():
            break
        if paths.strip() and private("ls","-l",paths.strip()).strip():
            # The status is below the fold on some emulator sizes. Wait for
            # workshop navigation to unlock after the file and history commit.
            if "Arrêter" not in view(): break
        time.sleep(1)
    else:raise AssertionError(f"{category} : {view()}")
    content=subprocess.check_output(["adb","exec-out","run-as",PACKAGE,"cat",paths.strip()])
    if ext=="png":
        assert content.startswith(b"\x89PNG") and struct.unpack(">II",content[16:24])==(768,768)
    elif ext=="mp4":
        assert b"ftyp" in content[:40] and b"moov" in content and b"avc1" in content
    else:
        assert content[:4]==b"RIFF" and content[8:12]==b"WAVE" and len(content)>10000
    print("PASS génération GGUF réelle",category,len(content),"octets",flush=True)

# Reopen the generated music from a filtered history after starting a fresh chat.
tap(view()["☰"])
tap(view()["Nouvelle discussion"])
tap(view()["☰"])
search = view()["Rechercher une discussion"]
tap(search)
adb("shell", "input", "text", "introuvable")
assert "Aucun résultat" in view()
adb("shell", "input", "keyevent", *(["67"] * len("introuvable")))
tap(view()["Une mélodie au piano"])
assert "Exporter la création" in view()
print("PASS : recherche, nouvelle discussion et reprise de la création")


def open_workshop(name):
    tap(view()["☰"])
    tap(view()[name])


def enter_prompt(text):
    editor = next(node for node in view().values()
                  if node["class"] == "android.widget.EditText")
    tap(editor)
    adb("shell", "input", "text", text.replace(" ", "%s"))
    adb("shell", "input", "keyevent", "4")


def generate_reply(question):
    enter_prompt(question)
    tap(view()["↑"])
    until = time.monotonic() + 300
    while time.monotonic() < until:
        nodes = view()
        if question in nodes and "Arrêter" not in nodes:
            if any(node.get("class") == "android.widget.EditText" and text != question
                   for text, node in nodes.items()):
                return
        time.sleep(1)
    raise AssertionError(f"Demande sans réponse : {question} : {view()}")


for category, question in [("Texte", "Reply with OK"), ("Code", "Print 42 in Python")]:
    open_workshop(category)
    generate_reply(question)
    tap(view()["☰"])
    nodes = expect_workshop(category)
    tap(nodes[question])
    assert question in view()
    print("PASS : envoi et reprise", category, flush=True)

enter_prompt("Brouillon mobile")
# A background process kill retains the task, like Android reclaiming memory.
adb("shell", "input", "keyevent", "3")
time.sleep(2)
adb("shell", "am", "kill", PACKAGE)
pid = subprocess.run(["adb", "shell", "pidof", PACKAGE], capture_output=True, text=True)
assert pid.returncode == 1 and not pid.stdout.strip(), pid
adb("shell", "am", "start", "-W", "-a", "android.intent.action.MAIN",
    "-c", "android.intent.category.LAUNCHER", "-f", "0x10200000", "-n", f"{PACKAGE}/.MainActivity")
restored = view()
assert "Brouillon mobile" in restored and "Print 42 in Python" in restored, restored
print("PASS : discussion et brouillon restaurés après arrêt du processus")
