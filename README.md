<div align="center">

# IA Locale Studio

**Cinq ateliers de création IA dans une interface française, avec des modèles qui tournent sur ton appareil.**

<a href="https://github.com/anderson76389/IA-Locale-Studio/releases"><img alt="Ouvrir les versions publiées sur GitHub" src="https://img.shields.io/badge/TÉLÉCHARGER-GitHub_Releases-7253D8?style=for-the-badge&amp;logo=github&amp;logoColor=white"></a>

[**Toutes les versions publiées →**](https://github.com/anderson76389/IA-Locale-Studio/releases)

</div>

| Plateforme | Téléchargement direct de la préversion `aa7b3c06662d` | Démarrage |
| --- | --- | --- |
| **Windows x64** | [**Télécharger le .exe**](https://github.com/anderson76389/IA-Locale-Studio/releases/download/preview-aa7b3c06662d/IA-Locale-Studio.exe) | Place le fichier dans un dossier dédié et lance-le. |
| **Linux x64** | [**Télécharger le binaire Linux**](https://github.com/anderson76389/IA-Locale-Studio/releases/download/preview-aa7b3c06662d/IA-Locale-Studio-Linux) | Exécute `chmod +x IA-Locale-Studio-Linux`, puis `./IA-Locale-Studio-Linux`. |
| **Android 8+** | [**Télécharger l’APK**](https://github.com/anderson76389/IA-Locale-Studio/releases/download/preview-aa7b3c06662d/IA-Locale-Studio.apk) | Installe l’APK ARM64/x86-64 et importe un modèle GGUF. |

## L’interface

<div align="center">
<img src="docs/interface-pc.jpg" alt="Atelier Texte sur PC avec les cinq ateliers et la bibliothèque de modèles" width="72%">
<img src="docs/atelier-video-mobile.jpg" alt="Atelier Vidéo dans l’interface web en format mobile" width="22%">
</div>

**Windows et Linux** ouvrent l’interface sur [localhost:1420](http://localhost:1420/). **Texte** et **Code** utilisent Ollama. **Image**, **Vidéo** et **Musique** utilisent ComfyUI avec un modèle et un workflow compatibles. Installe les moteurs et les modèles localement pour générer hors ligne. [Guide PC et compilation](desktop/README.md)

**Android** embarque un moteur CPU pour **Texte** et **Code**. Le même GGUF pilote trois ateliers de création locale : illustrations vectorielles PNG, animations MP4 de trois secondes et mélodies instrumentales WAV. Chaque atelier possède son historique et ses commandes de lecture, d’export et de suppression adaptées au format. La signature de test peut empêcher une mise à jour par-dessus un APK précédent. [Guide Android et compilation](android/README.md)

Les versions publiées sont des préversions. Les essais avec génération réelle sur GPU et sur téléphone restent à effectuer. [Contrôles GitHub Actions](https://github.com/anderson76389/IA-Locale-Studio/actions) · [Empreintes SHA-256](https://github.com/anderson76389/IA-Locale-Studio/releases/download/preview-aa7b3c06662d/SHA256SUMS.txt)

### Historique et pièces jointes

Sur Windows et Linux, chaque atelier a son historique local. Ouvrez une discussion pour la reprendre. « Supprimer définitivement » efface tous ses tours et leurs sorties du dossier actif. Les archives de sauvegarde et les copies exportées sont indépendantes.

Dans Texte et Code, ajoutez des documents UTF-8, DOCX, PPTX, des PDF avec couche texte (outil `pdftotext`) ou des images JPEG, PNG et WebP avec un modèle Ollama vision. Pour transcrire un enregistrement, installez FFmpeg et `whisper-cli`, renseignez la variable `IA_WHISPER_MODEL` avec le chemin absolu de votre modèle whisper.cpp et relancez le Studio. La transcription traite les dix premières minutes.

Sur Android, Texte et Code conservent leurs discussions dans l’application. « Gérer les modèles » permet de changer de GGUF ou de le supprimer. L’import reconnaît les copies identiques. Le bouton de pièce jointe extrait le texte des images et transcrit la parole des pistes audio compatibles avec le décodeur du téléphone. Pour activer la transcription française, importez le ZIP du modèle via « Modèle de transcription Vosk ». [Modèle français Vosk](https://alphacephei.com/vosk/models/vosk-model-small-fr-0.22.zip)

« Tester hors ligne » lance un calcul avec le modèle installé. Sur PC, dans Image, Vidéo et Musique, le test exécute le workflow ComfyUI choisi et vérifie la sortie attendue. Pour vérifier le fonctionnement sans Internet d’un workflow comportant des nœuds tiers, coupez le réseau avant le test.
