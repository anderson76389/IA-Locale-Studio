# IA Locale Studio — Android natif expérimental
Préversion indépendante du PC, sans compte ni serveur d'inférence distant.
Texte et Code : llama.cpp b6000 en CPU, import local de GGUF (commencer par 1–3B Q4). Aucun modèle inclus. ARM64 et x86-64, Android 8+. Internet requis pour récupérer les modèles depuis Hugging Face (navigateur), puis import via le sélecteur Android. Image, Vidéo, Musique et GPU/NPU non intégrés. Pas de pause/reprise de génération ni historique dans cette préversion.
Compilation : Android SDK 35, NDK 27.2.12479018, CMake 3.22.1, JDK 17, Gradle 8.9. Le workflow produit un APK debug pour essais, pas une release signée avec une clé pérenne. Aucune clé privée n'est stockée dans le dépôt. Les tests JVM ne prouvent pas la génération réelle sur téléphone. 
