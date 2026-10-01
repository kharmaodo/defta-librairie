# Adaptateur Go OCR expérimental — US-1714

Le runner local reste actif par défaut. L’adaptateur US-1714 ne change ni le matching, ni
l’authentification, ni les décisions humaines. La qualité et l’activation de
recette restent à valider dans US-1715 et US-1716.

## Configuration

| Variable Go | Défaut | Contrôle |
|---|---|---|
| `OCR_EXPERIMENTAL_ENABLED` | `false` | Booléen strict ; off ignore l’endpoint et le délai expérimentaux |
| `OCR_EXPERIMENTAL_ENDPOINT` | vide | Requis on ; origine HTTP(S) privée sans identifiants, chemin ou query |
| `OCR_EXPERIMENTAL_TIMEOUT_SECONDS` | `60` | Entier de 1 à 180 ; couvre lecture de la source et appel HTTP |

On exige `OCR_LANGUAGE=ara`. Le budget `COVER_WORKER_MAX_DELIVER` est limité
à 100 quand on. Une configuration invalide bloque le démarrage ; aucun fallback
silencieux n’est effectué après une panne du service Python. Le transport
ignore les proxies d’environnement et refuse les adresses publiques et link-local,
y compris après résolution DNS ; il ne suit aucune redirection.

Les variables `OCR_SERVICE_*` configurent le conteneur Python, pas le worker Go.
Avec les limites Python par défaut (15 secondes d’upload et 30 d’OCR), le délai
Go de 60 secondes conserve une marge. Ajuster ensemble ces limites si nécessaire.

## Branchement privé

Le worker OCR est démarré dans l’application `cmd` (`go run -tags fts5 ./cmd`).
Le binaire `cmd/cover-worker` traite les dérivés et ne porte pas ce worker OCR.
Le service Python doit être démarré selon son [guide](../services/ocr-experimental/README.md).
Pour une application Go sur l’hôte, utiliser le profil diagnostic lié uniquement
à loopback et `http://127.0.0.1:8091`. Pour une application Go conteneurisée,
la joindre au réseau privé OCR et utiliser `http://ocr-experimental:8091`.
Ne pas publier ce service sur une interface publique.

Après vérification des probes et sauvegarde de la base, configurer explicitement
le flag sur `true` et redémarrer l’application. Les variables exemples ne
modifient pas le fichier `.env` existant. L’activation générale attend US-1716.

## Contrat et persistance

Le Go lit la source privée MinIO, contrôle taille et signature PNG/JPEG, puis
transmet une unique image à `POST /v1/ocr`, avec le job comme `X-Request-Id`.
Python valide aussi MIME, dimensions et pixels avant inférence. La réponse JSON
est strictement contrôlée : version, champs obligatoires, absence de doublons,
langue arabe, moteur Tesseract 5, politique, PSM 6/11, prétraitement et confiance
nullable bornée à [0,1]. Corps et textes sont bornés ; les erreurs exposent des
codes fixes sans texte OCR ni image ni URL privée.

La migration additive **035** conserve les migrations précédentes et les anciens
résultats. Elle ajoute `policy_version`, `psm`, `preprocessing` et les colonnes de
bail/jeton/tentatives du job. La confiance utilise la colonne existante ; une
confiance absente reste NULL. Résultat, état MATCHING, événement d’outbox et audit
sont écrits dans une seule transaction, limitée à la bibliothèque du job. L’audit
contient les métadonnées, jamais le contenu OCR. Aucun rattachement automatique.

## Reprises et retour au runner local

Un bail de délai Go + 30 secondes protège chaque tentative. Le jeton clôture
uniquement sa propre tentative ; un worker périmé ne peut ni modifier le job ni
ACK avant le succès durable d’une reprise. Un traitement local ancien sans jeton
n’est jamais volé. La configuration JetStream est actualisée pour aligner AckWait
et le budget ; une livraison supplémentaire permet de terminer durablement un
job ayant épuisé son budget après un crash. Un bail déjà détenu entraîne un NAK
retardé de quatre minutes, sans nouvelle tentative métier.

Busy, indisponibilité, timeout et réponse invalide sont rejoués jusqu’au budget
persisté. Une source invalide est terminale immédiatement. Échec transactionnel
et arrêt/cancellation rétablissent le pending si le jeton est toujours détenu ;
sinon l’expiration permet la reprise. Un succès ou échec terminal durable est
ACK ; aucune duplication de résultat, événement ou audit.

Pour rollback, remettre le flag à `false` et redémarrer. Les baux actifs sont
respectés ; les baux expérimentaux expirés sont rendus au runner local. Les
résultats déjà écrits restent lisibles. Ne pas supprimer la migration 035 ni
réécrire les anciens résultats.

## Vérification

Les tests Go couvrent off/on, transport privé, multipart et contrat, erreurs,
cancellation, budget, expiration, clôture d’un jeton périmé, isolation bibliothèque,
NULL et rollback transactionnel. La CI OCR compile un binaire de test Go puis
l’exécute dans le conteneur Python réel isolé pour vérifier le contrat et les
métadonnées Tesseract arabe. Les suites existantes Go/FTS5, vet, frontend et
navigateur restent requises. Aucun gain de qualité n’est revendiqué par ce lot.

US-1715 branche également le [matching v2](OCR_MATCHING_QUALITY_V1_7_1.md) sur ce flag. Off conserve la politique de matching existante ; on applique le dictionnaire et le classement isolés par bibliothèque.
