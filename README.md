<div align="center">

# IA Locale Studio

### Votre intelligence artificielle. Vos appareils. Vos données.

**Un atelier de création IA local, pensé pour fonctionner sans cloud.**

`Confidentialité` · `Hors ligne` · `Modèles libres` · `Interface française`

[Découvrir le projet](#-le-projet) · [Téléchargements](#-téléchargements) · [État du développement](#-état-du-développement) · [Compilation](#-compiler-android)

</div>

---

## ✨ Le projet

IA Locale Studio vise à réunir cinq ateliers de création dans une application indépendante, légère et accessible, sans abonnement ni compte obligatoire. L'objectif : exécuter les modèles directement sur l'appareil, conserver ses créations localement et ne solliciter Internet que pour récupérer les modèles ou composants nécessaires.

> **Projet en développement.** La première édition Android native a été compilée et vérifiée par CI ; les essais sur appareil physique restent à effectuer. Ce dépôt ne contient pas encore une version finale multiplateforme.

## 🎨 Cinq espaces de création

| Atelier | Utilisation | Android natif — état actuel |
| :--- | :--- | :--- |
| 💬 **Texte** | Conversation, rédaction, résumé | Moteur CPU intégré, à valider |
| ⌨️ **Code** | Assistance et génération de code | Moteur CPU intégré, à valider |
| 🖼️ **Image** | Création visuelle | À intégrer |
| 🎬 **Vidéo** | Génération vidéo | À intégrer |
| 🎵 **Musique** | Composition et génération musicale | À intégrer |

## 🔒 Local par conception

- Génération sans serveur distant : moteur natif embarqué pour Texte et Code dans la préversion Android.
- Modèles GGUF importés dans le stockage privé de l'application.
- Aucun compte requis pour l'inférence locale.
- Avertissement explicite en cas de tentative de téléchargement hors ligne.
- Aucun PC ou service GitHub nécessaire pour utiliser l'APK après installation et import d'un modèle compatible.
- Pas de clé privée, de modèle volumineux ni de données personnelles dans le dépôt.

Internet reste nécessaire pour obtenir de nouveaux modèles ; la préversion ouvre Hugging Face dans le navigateur pour le téléchargement, puis permet l'import local. Les modèles et moteurs médias restent à intégrer.

## 📦 Téléchargements

| Plateforme | Format cible | Disponibilité dans ce dépôt |
| :--- | :--- | :--- |
| Android | `IA-Locale-Studio.apk` | [Préversion compilée](https://github.com/anderson76389/IA-Locale-Studio/actions/runs/45727507034) |
| Windows 64 bits | `.exe` | Pas encore publiée ici |
| Linux 64 bits | `.AppImage` | Pas encore publiée ici |

**Android :** consulter les [compilations GitHub Actions](https://github.com/anderson76389/IA-Locale-Studio/actions) et la [code source Android](https://github.com/anderson76389/IA-Locale-Studio/tree/main/android).

Pour la [compilation validée](https://github.com/anderson76389/IA-Locale-Studio/actions/runs/45727507034), télécharger l'artefact **IA-Locale-Studio-Android-preview** sur sa page Actions (un compte GitHub peut être requis pour récupérer l'artefact). Extraire uniquement `IA-Locale-Studio.apk`, puis l'installer sur Android. Aucun dossier Windows à transférer. Les artefacts de test sont temporaires et signés en mode debug ; les mises à jour pérennes nécessiteront une signature de publication stable.

## 📱 Compatibilité visée

| Appareil | Architecture / accélération |
| :--- | :--- |
| Smartphones et tablettes Android 8+ | ARM64 et x86-64 ; CPU dans la préversion |
| Windows / Linux — NVIDIA | CUDA, selon moteur et matériel (objectif) |
| Windows / Linux — AMD | Vulkan ou ROCm, selon compatibilité (objectif) |

Pour commencer sur Android, privilégier un petit modèle GGUF instruct de 1 à 3 milliards de paramètres, quantifié en Q4. La mémoire disponible, la température et les capacités du modèle déterminent les performances. La compatibilité réelle sur téléphone reste à tester.

## 🛠️ État du développement

- [x] Architecture Android native indépendante du PC
- [x] Code de génération Texte / Code avec llama.cpp
- [x] Import GGUF, arrêt de génération et avertissement réseau
- [x] Tests automatisés de l'import et workflow de compilation
- [x] APK compilé, signature debug et intégrité vérifiées en CI
- [ ] Exécution et génération sur appareil physique
- [ ] Téléchargement intégré des modèles avec pause et reprise
- [ ] Accélération GPU / NPU et mesures de mémoire, batterie et température
- [ ] Moteurs Image, Vidéo et Musique
- [ ] Historique, bibliothèque et sauvegarde intégrée
- [ ] Versions stables Windows, Linux et Android avec validation hors ligne

## ⚙️ Compiler Android

Le code Android est disponible dans la [branche principale](https://github.com/anderson76389/IA-Locale-Studio/tree/main/android).

**Prérequis :** JDK 17, Gradle 8.9, SDK Android 35, Build Tools 35.0.0, NDK 27.2.12479018 et CMake 3.22.1.

```bash
git clone https://github.com/anderson76389/IA-Locale-Studio.git
cd IA-Locale-Studio/android
gradle --no-daemon :app:assembleDebug
```

Sortie : `app/build/outputs/apk/debug/app-debug.apk`.

La compilation récupère les dépendances et le moteur llama.cpp. L'application compilée n'a pas besoin de GitHub pour générer localement.

## 🧩 Technologies et licences

Android natif (Java / C++ / JNI), Gradle, CMake et [llama.cpp](https://github.com/ggml-org/llama.cpp) (MIT). Les modèles externes conservent leurs licences respectives. Aucune licence globale du projet n'est déclarée à ce stade.

---

<div align="center">

**IA Locale Studio** · Une IA personnelle, indépendante et locale.

Développement en cours — les fonctionnalités non validées ne sont pas présentées comme disponibles.

</div>
