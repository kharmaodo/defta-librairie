# OCR expérimental v1.7.1 — cœur local et CLI (US-1712)

Diagnostic CPU local, partagé avec la future API FastAPI. Ce lot ne modifie pas
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

FastAPI/Docker : US-1713. Adaptateur Go et flag off/on : US-1714. Aucune API
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
