# Rattachement manuel de couverture — v1.7.1

## État US-1717

US-1717 clôturée le 1er octobre 2026 après revue finale et recette automatisée.
Implémentation fusionnée via PR #193 au commit `c11d0e3`, présente dans
`develop` de référence `9570e28`. Aucun défaut fonctionnel trouvé lors de la
revue ; les compléments portent sur les tests et la mise à jour des preuves.
La recherche manuelle du parcours d’import activé ne dépend pas du flag
`OCR_EXPERIMENTAL_ENABLED` ; la qualité OCR arabe reste dans son suivi distinct.

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


## Preuves de recette automatisée

- `TestManualCoverSearchScopeDismissAndPromotion` : trois résultats maximum,
  propriétaire/root bornés à la bibliothèque, refus de quarantaine, ISBN connu,
  rejet persistant et audité une seule fois, confirmation d’un remplacement,
  sélection MANUAL et acceptation répétée refusée sans seconde copie/outbox.
- `TestManualCoverSearchZeroOneAndDeletedSuggestions` : zéro et un résultat,
  opérateurs FTS littéraux, exclusion des livres étrangers/supprimés et refus
  d’une suggestion supprimée après recherche, sans copie ni décision finale.
- `TestManualCoverReplacementFailureKeepsExistingCover` : schéma issu des
  migrations réelles ; acceptation manuelle puis échec durable du traitement
  de la nouvelle couverture. L’ancienne couverture READY reste active avec
  son aperçu, la nouvelle FAILED reste inactive ; replay sans doublon.
- Tests HTTP existants : validation des entrées, session JWT et rôle existants.
  Le repository recontrôle aussi l’isolation dans la transaction de décision.
- Playwright `cover import UI searches three manual suggestions, dismisses one
  and confirms replacement` : zéro candidat OCR, trois suggestions, rejet,
  annulation de confirmation puis confirmation de remplacement.

[CI release #48](https://github.com/kharmaodo/defta-librairie/actions/runs/36885212283)
verte sur la branche de clôture US-1716, déjà fusionnée : **30 parcours navigateur**,
dont celui ci-dessus, et **125 tests frontend**. Revue finale sur `9570e28` :
Go/FTS5 complet, vet, race services/repositories/handlers, 125 tests frontend,
123 opérations OpenAPI et contrôle du patch réussis. Les deux nouveaux tests
sont inclus dans le gate du PR de finalisation.

Reproduire la recette métier ciblée :

```bash
go test -tags fts5 ./internal/services -run ManualCover -count=1 -v
go test -tags fts5 ./internal/handlers -run ManualCoverReview -count=1 -v
npm run test:browser -- --grep 'manual suggestions'
```

Ces preuves automatisées vérifient les parcours de rattachement manuel ; elles
ne revendiquent pas une validation de qualité de l’extraction OCR arabe sur
un corpus privé. La décision finale reste humaine et le succès différé d’une
nouvelle couverture demeure distinct de l’acceptation du rattachement.
