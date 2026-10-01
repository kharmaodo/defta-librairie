# v1.7.1 — OCR local expérimental et comparaison des candidats

## Objectif et état

Cadrage du 30 septembre 2026, après fusion du PR #186. Le cœur CPU/CLI (US-1712, PR #190) est fusionné. Le service FastAPI/Docker (US-1713) est implémenté sur sa branche dédiée ; son intégration Go et son activation relèvent de US-1714. La qualité OCR de la v1.7.0 reste une limite connue. Aucun tag ou artefact v1.7.0 n'est publié par ce cadrage.

Le nouveau cœur Python doit être partagé par CLI et FastAPI, fonctionner sur CPU et rester local. Go conserve l'authentification, les autorisations, les états, la persistance, l'outbox JetStream et les décisions humaines.

## Constats de recette

- Import, revue et rejet fonctionnent après le correctif du panneau.
- Sur le lot de douze images, quatre jobs ont un candidat ; leur pertinence n'est pas démontrée.
- Les textes fournis sont fortement bruités. Le runner Tesseract actuel impose PSM 6.
- Le matching actuel tronque aux douze premiers tokens : un titre situé après le bruit peut être exclu.
- Le rollback d'import masque notamment l'erreur de quota en 503 ; conserver les erreurs métier pour produire 429.
- La colonne confidence existe mais n'est pas alimentée par le runner actuel.

## Découpage et branches

Chaque US possède sa branche et son PR. Mettre à jour BACKLOG.md avec preuves avant publication.

| US | Branche prévue | Livrable et critère vérifiable |
|---|---|---|
| US-1710 | feature/v1.7.1-us-1710-ocr-spec | Contrat, mapping SQLite, critères de comparaison et limites documentés. |
| US-1711 | feature/v1.7.1-us-1711-import-errors | Quota conservé à travers rollback, HTTP 429, validation image conservée, tests de compensation et message UI explicite. |
| US-1712 | feature/v1.7.1-us-1712-ocr-core-cli | Cœur OCR CPU, CLI read-only, prétraitement borné, comparaison PSM, confiance réelle, tests et export JSON/CSV. |
| US-1713 | feature/v1.7.1-us-1713-ocr-fastapi | API interne, validation des images, limites, délais, capacité bornée, Docker et probes. |
| US-1714 | feature/v1.7.1-us-1714-ocr-go-adapter | Feature flag off par défaut, adaptateur HTTP Go, métadonnées OCR persistées, tests off/on/timeout/réponse invalide et reprise idempotente. |
| US-1715 | feature/v1.7.1-us-1715-matching-quality | Tokens utiles de tout le texte, correction prudente par dictionnaire de bibliothèque, candidats BM25 bornés, tests d'isolation et benchmark annoté. |
| US-1716 | feature/v1.7.1-us-1716-local-acceptance | Recette locale, comparaison qualité/latence/mémoire, rollback et gate v1.7.1. Activation par défaut seulement après validation explicite. |

## Mapping et responsabilités SQLite

| Prototype fourni | DEFTA-LIBRAIRIE | Règle |
|---|---|---|
| livres.id | defta.id | Identifiant du catalogue existant. |
| livres.titre_complet | defta.title | Texte logique Unicode, aucune réorganisation visuelle bidi en stockage. |
| livres.auteur / editeur | defta.auteur / editeur | Métadonnées existantes. |
| livres_fts | defta_fts | Index existant ; aucun nouvel index concurrent. |
| collection | Pas d'équivalence validée | Ne pas substituer automatiquement tags, catégorie ou bibliothèque. |
| raw_ocr | cover_import_ocr_results.text_raw | Go persiste après succès. |
| normalized | cover_import_ocr_results.text_normalized | Politique de normalisation versionnée. |
| confidence | cover_import_ocr_results.confidence | Valeur OCR mesurée dans [0,1], NULL si indisponible. |
| résultats candidats | cover_import_candidate_matches | Maximum cinq, rangs stables, scoped library_id, persistance idempotente Go. |

Le service OCR REST ne monte pas SQLite et ne reçoit aucun accès en écriture au catalogue.
La CLI de comparaison ouvre une base existante avec mode=ro et query_only ; elle ne crée ni tables, ni triggers, ni livres. Une copie cohérente de la base sert au benchmark reproductible. Toute migration supplémentaire relève de Go, avec un nouveau numéro et tests de migration.

Le dictionnaire et toutes les recherches excluent les livres supprimés et filtrent library_id avant correction ou classement. Il est interdit d'utiliser les titres d'une autre bibliothèque pour corriger un texte.

## Flux intégré

1. Go autorise le job et ne lance OCR qu'après SAFE ou approbation root de la quarantaine.
2. Go lit les octets du stockage privé, applique les limites et appelle le moteur sélectionné.
3. FastAPI retourne uniquement l'extraction et ses métadonnées ; aucun rattachement ni écriture métier.
4. Go persiste le résultat et l'événement MATCHING dans la même transaction.
5. Le worker de matching Go recherche et persiste les candidats dans la bibliothèque autorisée.
6. Le propriétaire ou root conserve la décision humaine et l'audit existant.

ACK après persistance durable. Les doublons JetStream ne doivent pas multiplier les résultats ou candidats. Une reprise utilise la politique de retries existante et bornée ; ne pas supposer que le passage actuel à FAILED permet un retry automatique.

## Contrat interne US-1713

POST /v1/ocr, multipart avec champ image JPEG/PNG. Pas de chemin disque ni URL fournis par le client. En-tête X-Request-Id pour corrélation, sans contenu OCR dans les logs.

Réponse 200 :

```json
{
  "schemaVersion": 1,
  "engine": "tesseract-experimental",
  "engineVersion": "version-reelle-du-runtime",
  "policyVersion": "ocr-local-v1",
  "language": "ara",
  "textRaw": "نص",
  "textNormalized": "نص",
  "confidence": null,
  "psm": 11,
  "preprocessing": "original"
}
```

Le choix PSM et du prétraitement ci-dessus est illustratif, à mesurer sur les images. La confiance ne devient jamais 95 % parce qu'un livre a été trouvé. BM25 est un score de classement, pas une probabilité. Conserver les hypothèses de correction séparément du texte brut.

Erreurs internes stables : INVALID_IMAGE (422), IMAGE_TOO_LARGE (413), OCR_TIMEOUT (504), OCR_UNAVAILABLE (503), OCR_BUSY (503). Enveloppe JSON avec code, message public et requestId. Go valide type, version, longueur maximale des textes et bornes numériques ; réponse invalide traitée comme erreur du moteur.

Le contrat et les limites effectives sont documentés dans [le service OCR](../services/ocr-experimental/README.md#service-fastapi-interne-us-1713).

GET /health/live : processus vivant. GET /health/ready : Tesseract, langue ara et configuration valides. Aucun téléchargement de modèle au démarrage.

## Configuration proposée

```dotenv
OCR_EXPERIMENTAL_ENABLED=false
OCR_EXPERIMENTAL_ENDPOINT=http://ocr-experimental:8091
OCR_EXPERIMENTAL_TIMEOUT_SECONDS=30
```

Ces variables sont proposées, pas encore consommées par le code. Off : runner actuel sans appel réseau expérimental. On : endpoint requis, validé au démarrage, timeout global bornant toutes les passes. Aucun fallback silencieux.
Le service est accessible sur réseau Docker interne ; l'accès CLI/REST de diagnostic est lié à 127.0.0.1. Le navigateur continue de passer par Go et les rôles JWT existants. Ne pas créer de comptes ou sessions Python.

## Qualité et confidentialité

Comparer l'original à des transformations limitées : rotation faible, contraste et segmentation PSM 6/11, puis autres modes si leurs mesures le justifient. La suppression des reflets du prototype peut effacer du texte ; elle ne doit pas être imposée sans validation sur les images.

Conserver l'ordre Unicode logique arabe. Supprimer les diacritiques pour le matching selon une politique cohérente avec l'index ; toute normalisation des alefs ou autres lettres doit être appliquée symétriquement ou utiliser un score secondaire explicitement documenté.

Une requête FTS utilise des tokens cités et des paramètres SQL. FTS5/BM25 n'est pas une distance d'édition : un éventuel reranking fuzzy est séparé, explicable, borné et testé. Ne pas confondre absence de candidat et panne technique.

Les originaux restent privés dans MinIO. Aucun cloud ni GPU. Temporaires supprimés même en timeout. Limites d'octets, dimensions, pixels et concurrence avant décodage et OCR. Image container non-root, runtime sans egress, dépendances figées. Logs : identifiants, durée, politique, statut ; jamais image, texte OCR, secret ou URL signée.

## Preuves et gate

- Corpus local annoté : image, SHA-256, library_id, book_id attendu ou absence réelle du catalogue. Ne pas commiter de couvertures ou données privées sans autorisation.
- Comparaison baseline/expérimental sur le même corpus : exactitude du titre, CER/WER quand transcription disponible, recall@1/5, faux candidats, absence de candidat, latence p50/p95 et mémoire maximale.
- Résultats sans vérité terrain ne permettent pas de conclure à une amélioration. Deux premières couvertures avec titres exacts attendus restent à fournir.
- Tests : bruit avant titre au-delà de douze mots, Unicode arabe, opérateurs FTS, zéro candidat, deux bibliothèques avec titres proches, images invalides, timeout global, service indisponible, doublons et reprise.
- Go : go test -tags fts5 ./... ; go vet -tags fts5 ./... ; git diff --check.
- Python : tests unitaires/intégration, lint et typage ; Docker Compose et parcours navigateur off/on.
- Adoption : gain de qualité mesuré et absence de régression d'isolation/idempotence ; budgets chiffrés de qualité et de latence à fixer après baseline, avant activation par défaut.
