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
launch = adb("shell", "am", "start", "-W", "-n", f"{PACKAGE}/.MainActivity")
assert "Status: ok" in launch, launch
first = expect_workshop("Texte")
assert "Importer un modèle GGUF" in first
assert "Aucun modèle installé" in first
tap(first["Code"])
tap(expect_workshop("Code")["Texte"])
expect_workshop("Texte")
print("PASS : installation, démarrage et sélection native Texte → Code → Texte")
