# OCR expérimental v1.7.1 — cœur CPU, CLI et service interne

Diagnostic CPU local, partagé avec le service FastAPI interne. Ce lot ne modifie pas
le worker Go et n'active aucun feature flag. Le catalogue reste celui de
DEFTA-LIBRAIRIE ; aucun second schéma ou CRUD livres n'est créé.

## Installation WSL

Depuis la racine du dépôt :

```bash
sudo apt-get update
sudo apt-get install -y python3-venv tesseract-ocr tesseract-ocr-ara fonts-dejavu-core
python3 -m venv services/ocr-experimental/.venv
services/ocr-experimental/.venv/bin/python -m pip install -r services/ocr-experimental/requirements-dev.txt
tesseract --version
tesseract --list-langs
```

Tesseract 5 et la langue ara sont obligatoires. Aucun téléchargement n'a lieu
pendant l'extraction. Les modèles doivent être présents avant le traitement.

## Comparer une image ou un dossier

Créer une copie SQLite cohérente (ne pas copier simplement un fichier SQLite
ouvert avec son WAL ignoré) :

```bash
mkdir -p data/ocr-benchmark
sqlite3 data/defta.db ".backup 'data/ocr-benchmark/catalogue.db'"

services/ocr-experimental/.venv/bin/python services/ocr-experimental/cli.py /chemin/vers/couvertures --db data/ocr-benchmark/catalogue.db --library-id ID_DE_LA_LIBRAIRIE --output data/ocr-benchmark/comparaison.json --timeout-seconds 30
```

Le chemin cible accepte une image ou un dossier, jusqu'à 100 JPEG/PNG. Le CLI
ouvre SQLite avec mode=ro et PRAGMA query_only, vérifie une bibliothèque active,
filtre library_id et deleted_at, et renvoie au plus cinq candidats par passe.
Il ne crée aucune table, aucun livre, aucun état de job ou audit.

Sortie CSV : ajouter --format csv et choisir un nom de sortie neuf en .csv.
Le fichier de sortie doit être inexistant ; aucune base, image ou sortie
existante ne peut être remplacée. Les exports ont les permissions 0600 sur
Linux et contiennent des données OCR privées. Ne pas les commiter.

Chaque enregistrement comprend empreinte SHA-256, version réelle Tesseract,
durée, baseline original/PSM 6, quatre passes (PSM 6/11 sur original et contraste
niveaux de gris), passe sélectionnée et candidats. Le texte arabe reste dans
l'ordre Unicode logique ; aucun reshaping visuel n'est appliqué au stockage.

La sélection utilise une heuristique de confiance OCR et conserve toutes les
passes pour comparaison. La confiance TSV est une moyenne pondérée par longueur
des mots, dans [0,1] ou null. Elle ne représente pas la probabilité de retrouver
le bon livre. Un résultat BM25 n'augmente jamais cette confiance.

Les deux recherches baseline/selected utilisent la même requête de diagnostic
sur les mots utiles de l'ensemble du texte (64 tokens au maximum). Elles ne
reproduisent pas exactement le matching Go v1.7.0 limité aux douze premiers mots.
Le nouveau matching de production et la correction par dictionnaire relèvent
de l'US-1715. FTS5 ne corrige pas à lui seul les fautes OCR.

Limites par image : 10 Mio, 24 millions de pixels, 1 Mio de sortie TSV par passe,
timeout global de 30 secondes, configurable jusqu'à 120. Un échec n'interrompt
pas les autres images sauf erreur de configuration/catalogue. Les temporaires
sont supprimés sur succès ou échec. Les sorties sont diagnostiques, sans
promotion automatique.

Codes de sortie : 0 tous les fichiers traités ; 1 au moins une erreur image/OCR
exportée ; 2 configuration, catalogue ou destination invalide. Le terminal
n'affiche pas le texte OCR ni les détails internes.

## Vérifier le lot

```bash
cd services/ocr-experimental
.venv/bin/python -m unittest discover -s tests -v
.venv/bin/python -m ruff check .
.venv/bin/python -m mypy --strict .
```

Le test réel utilise une image arabe synthétique générée en mémoire. Il prouve
la disponibilité du runtime, de la langue et du parsing TSV, pas la qualité sur
les douze couvertures de recette. Pour évaluer la qualité, préparer un corpus
privé avec book_id attendu ou absence réelle, puis comparer recall@1/5, erreurs
de transcription, latence et mémoire. Le service reste expérimental jusqu'à
cette mesure.

FastAPI/Docker est livré par US-1713 ci-dessous. Adaptateur Go et flag off/on : [guide US-1714](../../docs/OCR_GO_ADAPTER_V1_7_1.md). Aucune API
publique ou autorisation utilisateur supplémentaire n'est introduite ici.

## Comparaison couleur explicite

Ajouter `--color-diagnostics` pour comparer huit passes au lieu de quatre.
Les quatre passes supplémentaires utilisent une fenêtre centrale fixe, le canal
rouge et la différence rouge/bleu, avec PSM 6/11. Ce recadrage n'est pas une
détection automatique du titre et peut exclure certains titres. Le budget global
de temps et la limite de pixels restent applicables.

`pass_candidates` contient les candidats FTS5 de chaque passe, filtrés par
`library_id`, dans les exports JSON et CSV. `selected` et `candidates` conservent
la sélection des quatre passes initiales : aucun résultat diagnostic ne remplace
implicitement la politique par défaut.

Sur une couverture arabe dorée sur fond bleu fournie pour le benchmark, une
passe couleur a reconnu le mot « الفارض », absent des quatre passes initiales.
C'est une observation sur une image, pas une validation du titre complet, du
matching sur le catalogue réel ni un gain général mesuré. Les images privées
ne sont pas ajoutées au dépôt. Comparer plusieurs couvertures avec leurs titres
vérifiés avant de modifier la sélection ou d'activer ces passes en production.

## Candidats combinés pour revue

`combined_candidates` expose au plus cinq livres issus de toutes les passes
activées, dédupliqués par livre. Chaque source distincte `(preprocessing, psm)`
apparaît avec son rang FTS5, son score BM25 et les `title_words` communs exacts
après normalisation. Une correspondance sur auteur/éditeur peut avoir une liste
de mots du titre vide. Le tri utilise le nombre de passes sources décroissant,
le meilleur rang puis l'identifiant du livre. Les passes sont corrélées : ce
nombre n'est ni une probabilité ni une validation du titre.

La sélection OCR initiale et `candidates` restent disponibles pour comparaison.
Cette union diagnostique ne rattache aucun livre, n'écrit pas dans SQLite et
porte `review_required: true`. Elle conserve le filtrage par bibliothèque et
l'exclusion des livres supprimés. Les quatre passes couleur nécessitent toujours
`--color-diagnostics`.

## Service FastAPI interne (US-1713)

Le service ne lit ni n’écrit le catalogue : aucun volume SQLite, aucun candidat,
aucun rattachement et aucun compte utilisateur Python. Le navigateur conserve
les routes Go et JWT existants. Le worker Go peut utiliser ce service via le flag US-1714, désactivé par défaut.

### Exécution CPU locale

```bash
services/ocr-experimental/.venv/bin/python -m pip install -r services/ocr-experimental/requirements-api.txt
cd services/ocr-experimental
.venv/bin/uvicorn api:app --host 127.0.0.1 --port 8091 --workers 1 --limit-concurrency 16 --no-access-log
```

Le service nécessite Tesseract 5 et `ara` déjà installés. Il ne télécharge rien
au démarrage ni pendant l’extraction. `GET /health/live` répond 200 si le
processus est vivant ; `GET /health/ready` vérifie réellement Tesseract et `ara`
avec un délai de 2 secondes et répond 503 si le runtime est indisponible.

### Contrat HTTP

`POST /v1/ocr` accepte exactement un fichier multipart `image`, avec MIME
`image/jpeg` ou `image/png` cohérent avec sa signature et son format décodé.
Un nom de fichier n’est jamais utilisé comme chemin. URL, chemins clients,
fichiers supplémentaires et champs texte ne sont pas acceptés.

```bash
curl --fail-with-body http://127.0.0.1:8091/v1/ocr \
  -H 'X-Request-Id: diagnostic-1713' \
  -F 'image=@/chemin/prive/couverture.png;type=image/png'
```

La réponse 200 comporte `schemaVersion: 1`, `engine`, `engineVersion` réelle,
`policyVersion: ocr-local-v1`, `language: ara`, `textRaw`, `textNormalized`,
`confidence` mesurée dans [0,1] ou null, `psm` et `preprocessing`. Le texte reste
Unicode logique ; la normalisation supprime diacritiques/tatweel et remplace la
ponctuation par des espaces, comme la CLI. Quatre passes, politique initiale
inchangée ; les variantes couleur restent un diagnostic CLI explicite.

Les erreurs ont l’enveloppe `{ "error": { "code", "message", "requestId" } }` :
`INVALID_IMAGE` 422, `IMAGE_TOO_LARGE` 413, `OCR_TIMEOUT` 504,
`OCR_UNAVAILABLE` 503, `OCR_BUSY` 503 avec `Retry-After: 1`. Un résultat TSV
invalide/trop long est une indisponibilité du moteur, sans détails internes.
L’identifiant de corrélation est repris s’il contient 1–128 caractères ASCII
lettres/chiffres/`._:-`, sinon un UUID est généré. Réponses OCR `no-store` ;
aucun texte/image/URL privée dans les logs, documentation interactive désactivée.

### Limites du service

| Variable du conteneur | Défaut | Borne |
|---|---:|---|
| `OCR_SERVICE_MAX_IMAGE_BYTES` | 10485760 | 1 à 10 Mio |
| `OCR_SERVICE_MAX_PIXELS` | 24000000 | 1 à 24 millions |
| `OCR_SERVICE_MAX_DIMENSION` | 10000 | 1 à 10000 par côté |
| `OCR_SERVICE_MAX_OUTPUT_BYTES` | 1048576 | 1 à 1 Mio par passe |
| `OCR_SERVICE_TIMEOUT_SECONDS` | 30 | >0 à 120, partagé entre toutes les passes |
| `OCR_SERVICE_UPLOAD_TIMEOUT_SECONDS` | 15 | >0 à 30, lecture/parsing multipart |
| `OCR_SERVICE_CONCURRENCY` | 1 | 1 à 4, mémoire/CPU à dimensionner si augmenté |

Le corps multipart entier est limité à la taille image + 64 Kio, y compris
sans `Content-Length`, avant parsing. La capacité est réservée avant lecture ;
aucune file d’attente OCR illimitée. Une déconnexion ne libère pas un traitement
encore actif : le timeout du cœur tue le sous-processus et ses temporaires sont
nettoyés avant de rendre le créneau. Les uploads multipart sont toujours fermés.
Le temps maximal normal comprend le budget upload puis le budget OCR ; le
client Go configure son propre délai (60 secondes par défaut) via US-1714.
Une configuration invalide empêche le démarrage.

### Conteneur privé

```bash
docker compose -f services/ocr-experimental/compose.ocr-experimental.yaml up -d --build
```

Pas de port publié par défaut : réseau Docker `internal`, sans egress, exposé
uniquement en 8091 aux services joints à ce réseau. L’intégration réseau avec
Go est documentée dans le [guide US-1714](../../docs/OCR_GO_ADAPTER_V1_7_1.md). Pour un diagnostic sur la machine uniquement :

```bash
docker compose -f services/ocr-experimental/compose.ocr-experimental.yaml \
  -f services/ocr-experimental/compose.ocr-diagnostic.yaml up -d --build
```

Le port est alors lié à `127.0.0.1:8091`. Image Python 3.12.12, dépendances
Python et paquets Tesseract épinglés, utilisateur 10001, système read-only,
`/tmp` tmpfs 256 Mio, capacités supprimées, `no-new-privileges`, mémoire 1 Gio,
2 CPU et 64 PID. Un worker Uvicorn, Tesseract borné à un thread OpenMP. La mémoire
doit être adaptée avant d’augmenter concurrence ou taille des images.
Aucun modèle, base, image ou export privé n’entre dans le contexte Docker.

### Vérification de livraison

```bash
sh services/ocr-experimental/check-container.sh
```

Le contrôle construit l’image, valide Compose et lance le runtime en réseau
`none`, sans volume métier et avec système read-only. Un smoke test HTTP utilise
une image synthétique en mémoire, vérifie probes, version réelle, contrat et
nettoyage des temporaires. La CI OCR exécute aussi tests unitaires/intégration,
Ruff et mypy strict. Les limites, refus, saturation, upload lent et déconnexion
sont testés. La qualité sur couvertures réelles reste à mesurer dans US-1716.
