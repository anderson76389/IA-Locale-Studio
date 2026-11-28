IA LOCALE STUDIO — ÉDITION HORS LIGNE EXPÉRIMENTALE
25 novembre 2026

IMPORTANT : cette édition est un nouveau programme indépendant, et non une mise à
jour binaire ou une migration de la v0.7.12. Les archives sources de la v0.7.12
sont restées inaccessibles. Ne supprimez ni ne remplacez votre ancienne version.
Aucun modèle, moteur Ollama/ComfyUI ni pilote GPU n'est inclus dans cette archive.

DÉMARRAGE
1. Extraire le dossier complet dans Téléchargements, sans écraser l'ancienne version.
2. Windows x64 : double-cliquer Lancer-Windows.bat ou IA-Locale-Studio.exe.
   Linux x64 : ./Lancer-Linux.sh (chmod +x IA-Locale-Studio-Linux Lancer-Linux.sh si besoin).
3. Ouvrir http://localhost:1420/ dans votre navigateur. Le serveur écoute uniquement
   sur 127.0.0.1. Si Firefox ne s'ouvre pas, saisir l'adresse manuellement.
4. Si le port est déjà occupé par l'ancienne version, fermer cette dernière ou
   lancer l'édition de consolidation avec -port 1421. Aucun processus n'est tué.
5. Fermer la console pour arrêter ce serveur. Les moteurs externes peuvent rester actifs.

MOTEURS RÉELS (à installer séparément)
Texte / Code : Ollama local à http://127.0.0.1:11434 avec un modèle GGUF déjà installé.
Seuls les modèles locaux GGUF détectés sont affichés. Le moteur doit être démarré.
La génération en flux utilise /api/chat ; le code généré n'est jamais exécuté.
La demande keep_alive=0 est envoyée pour libérer le modèle à la fin. Ce n'est pas
une preuve de libération physique de la VRAM. Contexte par défaut 4096 tokens.
Pour configurer une nouvelle instance Ollama locale, utiliser OLLAMA_NO_CLOUD=1,
OLLAMA_HOST=127.0.0.1:11434 et OLLAMA_MODELS pointant vers data/models/ollama,
puis démarrer Ollama. Ne pas modifier les variables d'une instance déjà en cours.
Le moteur et ses fichiers de configuration peuvent encore vivre hors du dossier.
Documentation officielle : https://docs.ollama.com/faq et https://docs.ollama.com/api/chat

Image / Vidéo / Musique : ComfyUI local à http://127.0.0.1:8188 avec les modèles et
nœuds nécessaires. Exporter un workflow en format API JSON depuis ComfyUI, remplacer
l'entrée de texte positive (valeur entière) par {{PROMPT}} et inclure un nœud de
sauvegarde adapté. Importer le JSON dans l'atelier correspondant et confirmer sa
provenance fiable. Les workflows et nœuds peuvent exécuter du code ou contacter
Internet : ils ne doivent pas être importés depuis une source inconnue. Aucun
workflow ou modèle n'est fourni ; pas de garantie pour tous les nœuds personnalisés.
Les sorties sont copiées si elles sont exposées par /history et /view dans un format
compatible (image PNG/JPEG/WebP, vidéo MP4/WebM/GIF, audio WAV/MP3/FLAC/OGG/M4A).
Limites : 32 fichiers et 4 Gio par génération, 2 Gio par fichier.
Le moteur n'est pas isolé par cette application. Documentation des routes :
https://docs.comfy.org/development/comfyui-server/comms_routes

DONNÉES ET MAINTENANCE
Les créations, conversations, rapports et modèles copiés manuellement dans data/
restent sous ce dossier. Les modèles installés ailleurs par un moteur ne sont pas
déplacés automatiquement. Le navigateur peut stocker son propre cache ailleurs.
L'historique de conversation en cours est conservé pendant la session navigateur ;
les échanges terminés sont enregistrés dans conversation.json (non chiffré).
Les commandes sont sérialisées : une génération/maintenance à la fois. Arrêter
l'attente annule la requête locale ; pour ComfyUI une suppression de l'élément de
file est demandée. Un moteur externe déjà actif peut continuer. Pause/reprise de
calcul non implémentée. Aucun bouton fictif de pause/reprise.
La recherche Hugging Face est une requête réseau explicite. Elle affiche jusqu'à
30 résultats, pas une garantie de compatibilité ni une installation automatique.
Le benchmark réutilise les tests Ollama de l'outil précédent ; les erreurs restent
visibles. Les mesures matérielles utilisent nvidia-smi, sysfs AMD sous Linux et
inventaire CIM sous Windows ; les informations non disponibles ne sont pas inventées.
Le bouton sauvegarde crée une archive SHA-256 vérifiée de data/ dans backups/.
Restauration dans un dossier neuf restauration-ID/IA-Locale-Studio/ sans écrasement.
Les archives ne sont pas chiffrées ; sauvegarder seulement si les moteurs ne
modifient pas les modèles simultanément. Les modèles externes sont exclus.
La restauration n'active pas automatiquement les données restaurées.

CLI : IA-Locale-Studio.exe diagnostic [-bench auto] [-out rapport.json]
      IA-Locale-Studio.exe backup DOSSIER ARCHIVE.zip
      IA-Locale-Studio.exe verify ARCHIVE.zip
      IA-Locale-Studio.exe restore ARCHIVE.zip NOUVEAU_DOSSIER
Sous Linux remplacer IA-Locale-Studio.exe par ./IA-Locale-Studio-Linux.
Options du serveur : -no-browser -root DOSSIER -port 1420 -ollama URL -comfy URL.
Les URL moteur sont limitées à localhost/boucle locale. Pas de service LAN ou HTTPS
mobile dans cette édition. Ne pas exposer le serveur à Internet.

VALIDATION
28 tests principaux Go (dont plusieurs sous-tests), test -race, go vet, syntaxe JS,
serveur Linux compilé effectivement exécuté, 44 identifiants HTML et 44 références
JS vérifiés, format des deux binaires. Voir tests-*.txt et source/.
Les cinq parcours génération sont testés avec des moteurs simulés : aucune
inférence réelle, aucun rendu réel, aucune validation sur RX 9060 XT/RTX 5060 Ti.
Le test Chromium n'a pas pu s'exécuter : politique administrateur bloque les URL
locales. Firefox, Windows en exécution et Android natif non testés.
L'APK, le gestionnaire de téléchargement avec reprise garantie, l'optimisation automatique
par GPU, la mesure thermique AMD Windows et le test de longue durée restent à faire.
Ce n'est pas une v1.0 certifiée, ni le remplacement de la v0.7.12.

MODE HORS LIGNE
- L'interface et les générations locales ne nécessitent pas de connexion si les
  moteurs, modèles, dépendances et workflows sont déjà installés.
- Le statut réseau utilise l'état du navigateur; une connexion réseau détectée
  ne prouve pas l'accès Internet. Aucun sondage Internet automatique au repos.
- La recherche Hugging Face et l'ouverture de sa page sont des actions réseau
  explicites. Les téléchargements de fichiers Hugging Face se font sur ce site.
- Pour Texte/Code, entrer le nom d'un modèle Ollama dans la bibliothèque puis
  confirmer Télécharger. Le registre est vérifié et la progression des fichiers
  est affichée. Le succès exige que le modèle GGUF local soit détecté ensuite.
- La coupure réseau affiche un message; aucun succès n'est annoncé sur flux tronqué.
  Arrêter l'attente ne garantit pas l'annulation côté moteur. Le stockage et la
  reprise éventuelle sont gérés par Ollama, pas par cette application.
- Les moteurs externes et les nœuds ComfyUI ne sont pas isolés du réseau.
  Pour un mode strictement hors ligne, configurer Ollama sans cloud, utiliser
  uniquement des workflows et nœuds locaux, puis couper la connexion Internet.
- Les nouveaux tests réseau utilisent des transports et moteurs simulés. Aucun
  modèle réel n'a été téléchargé ni exécuté sur GPU physique. Le registre réel
  n'a pas été vérifié. Le test navigateur précédent n'a pas été réexécuté.

Sous Linux, lance `./IA-Locale-Studio-Linux --install-launcher` depuis son dossier définitif pour ajouter IA Locale Studio et son icône au menu des applications.
