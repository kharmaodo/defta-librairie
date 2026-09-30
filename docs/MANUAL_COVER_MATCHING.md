# Rattachement manuel de couverture — v1.7.1

Une image SAFE à l'état REVIEW_REQUIRED peut être recherchée manuellement,
même sans candidat OCR. La recherche propose au plus trois livres actifs de la
bibliothèque du job, triés par BM25 puis identifiant. Les mots sont des termes
littéraux : les opérateurs FTS saisis ne sont pas exécutés.

## Parcours

- Rechercher par titre ou auteur ; trois suggestions maximum, auteur et aperçu
  authentifié de la couverture actuelle, avec indication de remplacement.
- Résoudre : confirmer le livre, puis réutiliser la promotion existante via
  book_covers PENDING et outbox. L'ancienne couverture reste active jusqu'au
  traitement réussi de la nouvelle. La décision humaine seule ne garantit pas
  le succès du traitement différé.
- Rejeter la solution : écarter durablement ce livre pour cette image, sans
  rejeter le job. Il est exclu des recherches ultérieures et candidats OCR.
- Rejeter le rattachement : confirmation séparée ; termine le job REJECTED.

L'ISBN-13 exact, avec ou sans tirets, est recherché parmi les ISBN déjà enregistrés
dans cover_import_ocr_results pour des jobs READY rattachés à un livre de cette
bibliothèque. Le catalogue defta ne possède pas de champ ISBN : aucun ISBN
absent de ces résultats OCR n'est inventé ou déduit du titre.

## Contrats

POST /api/manage/cover-imports/{id}/candidate-search : JSON libraryId (root),
query (2 à 200 caractères, 20 termes maximum) ; retourne results, maximum trois.
La recherche matérialise les suggestions, sans créer/modifier un livre.

POST /api/manage/cover-imports/{id}/candidate-dismiss : JSON libraryId (root),
bookId ; rejet idempotent, audité une seule fois.

POST /api/manage/cover-imports/{id}/decision conserve ACCEPT/REJECT. ACCEPT
exige un candidat OCR ou une suggestion manuelle enregistrée, non écartée,
appartenant à la bibliothèque et non supprimée.

Chaque décision finale et rejet de proposition est audité avec acteur, rôle,
bibliothèque, bookId, corrélation et politique NSFW ; l'audit d'acceptation indique
selectionMethod. Une répétition d'acceptation après décision finale renvoie 409
sans seconde copie, couverture ou outbox. Les recherches ne stockent pas le
texte saisi dans les logs ou dans les suggestions.

## Sécurité et rétention

Les propriétaires sont bornés à leur bibliothèque active. Le root doit choisir
la bibliothèque du job. REVIEW/UNSAFE ne permettent aucun rattachement : la
revue root de quarantaine reste un parcours distinct. Les aperçus passent par
l'API privée existante, avec JWT et autorisation, sans URL MinIO publique.

Migration 034 : cover_import_review_suggestions, clé job_id/book_id, origine
MANUAL/OCR, rejet et acteur. Les données sont supprimées avec les métadonnées
d'import selon la rétention existante. Aucun nouveau plan/quota ou moteur cloud.

## Recette

Tester absence de candidat, recherche 0/1/3 résultats, candidat étranger ou
supprimé, rejet persistant, annulation puis confirmation, couverture déjà
présente, reprise HTTP et absence de doublons d'audit/outbox. Vérifier la
conservation de l'ancienne couverture si le traitement différé échoue.
