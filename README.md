<div align="center">

# IA Locale Studio

### Votre intelligence artificielle. Vos appareils. Vos données.

**Cinq ateliers de création. Une IA locale, sans cloud obligatoire.**

`Windows` · `Linux` · `Android` · `Navigateur local`

[**Téléchargements**](#-téléchargements) · [Fonctionnalités](#-cinq-ateliers) · [Installation](#-démarrage) · [Code source](#-organisation)

</div>

---

## 📦 Téléchargements

Toutes les éditions sont réunies dans ce dépôt. Les binaires sont destinés à être utilisés indépendamment, sans compte GitHub ni PC obligatoire pour Android.

| Plateforme | Fichier | État |
| :--- | :--- | :--- |
| Windows x64 | `IA-Locale-Studio.exe` | Préversion PC, compilation CI |
| Linux x64 | `IA-Locale-Studio-Linux` | Préversion PC, exécutable natif |
| Navigateur | [http://localhost:1420/](http://localhost:1420/) | Interface intégrée au binaire PC |
| Android ARM64 / x86-64 | `IA-Locale-Studio.apk` | [Première compilation validée](https://github.com/anderson76389/IA-Locale-Studio/actions/runs/45727507034) |

**[Versions publiées](https://github.com/anderson76389/IA-Locale-Studio/releases)** · **[Compilations et artefacts](https://github.com/anderson76389/IA-Locale-Studio/actions)**

Les préversions multiplateformes sont publiées par CI uniquement si les trois compilations et contrôles requis passent. Les artefacts Actions sont temporaires. Les APK de test utilisent une signature debug : une clé de publication stable reste nécessaire. Les modèles et moteurs externes ne sont pas inclus dans les binaires PC.

> **Important :** cette édition PC est la consolidation hors ligne expérimentale, indépendante de l'ancienne v0.7.12. Elle ne constitue pas une migration ni une version 1.0 certifiée.

## 🎨 Cinq ateliers

| Atelier | Windows / Linux / navigateur local | Android |
| :--- | :--- | :--- |
| 💬 Texte | Ollama local | llama.cpp CPU |
| ⌨️ Code | Ollama local | llama.cpp CPU |
| 🖼️ Image | ComfyUI + workflow et modèle compatibles | À intégrer |
| 🎬 Vidéo | ComfyUI + workflow et modèle compatibles | À intégrer |
| 🎵 Musique | ComfyUI + workflow et modèle compatibles | À intégrer |

Sur PC : bibliothèque Hugging Face, téléchargement Ollama, historique local, observateur matériel, benchmark, sauvegarde et restauration. Les cinq parcours PC ont des tests avec moteurs simulés ; ils ne sont pas encore validés avec des générations réelles sur GPU. Aucun bouton de pause/reprise fictif : l'arrêt d'une attente ne garantit pas l'arrêt immédiat du moteur externe.

## 🔒 Local par conception

- Aucun compte ni serveur d'inférence distant obligatoire.
- Internet pour rechercher et télécharger modèles et dépendances ; avertissement en cas de connexion indisponible.
- Modèles installés utilisables hors ligne lorsque les moteurs sont configurés localement.
- Serveur PC limité à la boucle locale ; pas d'exposition Internet.
- Pas de télémétrie volontaire ajoutée dans le code de l'application ; moteurs, workflows et navigateur externes à vérifier séparément.
- Aucun modèle volumineux, historique personnel ou clé privée publié dans GitHub.

## 🚀 Démarrage

### Windows

Télécharger le binaire, le placer dans un dossier dédié, puis l'exécuter. L'interface s'ouvre dans le navigateur local. Firefox est pris en charge par le lanceur lorsqu'il est détecté.

### Linux

```bash
chmod +x IA-Locale-Studio-Linux
./IA-Locale-Studio-Linux
```

### Navigateur

Une fois le programme PC démarré, ouvrir **http://localhost:1420/**. Ce mode n'est pas un site hébergé : le serveur d'inférence et les fichiers restent sur l'appareil. Utiliser `-no-browser` pour démarrer sans ouverture automatique.

Installer Ollama et/ou ComfyUI localement, puis les modèles nécessaires. Consulter le [guide PC complet](desktop/README.md) pour les workflows API JSON, les dossiers, le stockage et les paramètres de confidentialité.

### Android

Installer uniquement `IA-Locale-Studio.apk`. Télécharger un modèle GGUF compatible depuis Hugging Face, puis l'importer dans l'application. Le moteur CPU natif est embarqué. Commencer par un petit modèle instruct quantifié (1–3B Q4). La génération réelle, la consommation et l'accélération GPU/NPU restent à tester.

## 🗂️ Organisation

```text
IA-Locale-Studio/
├── android/           Application native Java / C++ et llama.cpp
├── desktop/           Serveur Go, moteurs locaux et outils
│   ├── web/            Interface navigateur embarquée
│   ├── *_test.go       Tests Go
│   └── README.md       Guide PC détaillé
└── .github/workflows/  Compilation et publication CI
```

### Compiler

**PC :** Go 1.22 minimum (CI avec Go 1.25), aucun module Go externe.

```bash
cd desktop
go test ./...
go build -o IA-Locale-Studio .
```

Sous Windows, utiliser `go build -o IA-Locale-Studio.exe .`.

**Android :** JDK 17, Gradle 8.9, SDK Android 35, NDK 27.2.12479018, CMake 3.22.1.

```bash
cd android
gradle --no-daemon :app:assembleDebug
```

Sortie : `app/build/outputs/apk/debug/app-debug.apk`.

## 🛠️ Validation restante

- [x] Première compilation Android ARM64 et x86-64, signature debug vérifiée
- [x] Code source PC et interface navigateur réunis
- [x] Tests Go de la consolidation PC exécutés localement
- [ ] Validation des nouvelles compilations PC en CI
- [ ] Génération réelle et mesure des performances AMD / NVIDIA
- [ ] Validation Android sur téléphone physique
- [ ] Moteurs Image, Vidéo et Musique natifs Android
- [ ] Signature pérenne Android, optimisation et tests prolongés

## 🧩 Technologies et licences

Go, HTML/CSS/JavaScript, Android Java/C++/JNI, Gradle, CMake, Ollama, ComfyUI et [llama.cpp](https://github.com/ggml-org/llama.cpp) (MIT). Les modèles et moteurs gardent leurs propres licences. Aucune licence globale n'est déclarée à ce stade.

---

<div align="center">

**IA Locale Studio** · Créer localement, garder le contrôle.

Préversions expérimentales — aucune fonctionnalité non validée n'est présentée comme certifiée.

</div>
