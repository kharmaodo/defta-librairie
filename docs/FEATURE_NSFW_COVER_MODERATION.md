# Fonctionnalité — modération NSFW des couvertures

## Objectif

Empêcher qu’une couverture non conforme soit visible dans le catalogue ou utilisée
pour créer un livre. La décision est rendue par un modèle local exécuté dans le
conteneur `nsfw-moderator` ; aucune image n’est envoyée à un service tiers.

Le contrat technique est décrit dans
[`NSFW_COVER_MODERATION.md`](NSFW_COVER_MODERATION.md). Cette page est la
référence d’avancement des user stories et de leur recette.

## User stories

| US | Résultat attendu | État | Preuve principale |
|---|---|---|---|
| Soumettre un nouveau livre avec couverture | Le livre reste absent tant que la décision n’est pas positive. | Réalisé | `POST /api/manage/book-submissions`, outbox SQLite et worker. |
| Accepter automatiquement une couverture sûre | Le livre est créé, puis la variante active est produite. | Réalisé | `APPROVED`, `created_book_id`, `book_covers.status=READY`. |
| Refuser une couverture non conforme | Aucun livre n’est créé et aucune couverture n’est publique. | Réalisé | `REJECTED`, `MODEL_UNSAFE`, `created_book_id IS NULL`. |
| Traiter un résultat ambigu | Seul le root décide manuellement ; la décision est auditée. | Réalisé | `POST /api/manage/book-submissions/{id}/decision`. |
| Reprendre une indisponibilité du modèle | Une soumission `FAILED` est relançable sans doublon. | Réalisé | `POST /api/manage/book-submissions/{id}/retry`. |
| Remplacer la couverture d’un livre existant | L’image est modérée avant stockage ; un refus conserve la couverture précédente. | Réalisé | `422 cover_moderation_rejected`. |
| Afficher une couverture sûre | Les routes publique et admin ne servent qu’une variante `READY` active ; sinon l’interface utilise l’image par défaut. | Réalisé | `GET /api/books/{id}/cover` et `GET /api/manage/books/{id}/cover`. |
| Purger la quarantaine | Les sources rejetées ou expirées sont supprimées avec reprise vérifiable. | À réaliser | Worker dédié, file de nettoyage et tests de reprise. |

## Politique appliquée

- `SAFE` : acceptation ;
- `UNSAFE` : refus ;
- `REVIEW` : revue humaine ;
- service indisponible, délai ou réponse invalide : refus de création et état
  `FAILED`.

La politique est **fail-closed** : en l’absence de décision positive, le livre
n’est jamais créé. Lors du remplacement d’une couverture existante, l’ancienne
couverture reste inchangée en cas de refus ou d’indisponibilité.

## Exploitation locale

Démarrer le modérateur depuis la racine du dépôt :

```bash
docker compose -f services/nsfw-moderator/compose.nsfw-moderator.yaml up -d --build
```

Quand l’application Go s’exécute sur l’hôte, elle doit joindre le port publié :

```bash
NSFW_MODERATION_ENDPOINT=http://127.0.0.1:8090 go run -tags fts5 ./cmd
```

Quand l’application s’exécute dans le même réseau Docker que le modérateur,
l’endpoint est `http://nsfw-moderator:8090`.

Le modèle et son manifeste restent hors du dépôt, par exemple sous
`/opt/models/nsfw`, et sont montés en lecture seule dans le conteneur.

## Critères de clôture

La fonctionnalité sera clôturée lorsque l’US de purge sera réalisée et que la
recette complète confirmera :

1. la suppression différée et rejouable d’une source rejetée ou expirée ;
2. l’absence de toute route HTTP vers la quarantaine ;
3. la non-régression des parcours approuvé, refusé, ambigu et indisponible ;
4. les contrôles `go test -tags fts5 ./...`,
   `./scripts/check-delivery.sh` et Playwright verts.
