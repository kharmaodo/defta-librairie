# Recette et rétention v1.7.0

La branche `feature/v1.7.0-us-1707-retention-gate` ajoute la rétention des imports
de couvertures. Le worker de nettoyage existant réconcilie les échéances toutes
les cinq minutes, place les objets en file SQLite et réessaie une suppression
MinIO échouée. Une reprise ne crée pas de seconde tâche pour la même clé.

| Original | Point de départ | Durée |
|---|---|---:|
| Échec technique | `terminal_at` | 7 jours |
| Rejet NSFW (`REVIEW` ou `UNSAFE`) | `terminal_at` | 14 jours |
| Rejet après revue SAFE | `terminal_at` | 30 jours |
| Revue non terminée ou quarantaine | `expires_at` de l'import | 30 jours après dépôt |
| Accepté | `terminal_at` | 90 jours |

À l'expiration d'une revue ou d'un traitement en cours, le job passe à
`CANCELLED` avec `SOURCE_EXPIRED`. Après 24 mois depuis le dépôt, les résultats
OCR, les candidats, les décisions et les audits échus sont supprimés, mais
uniquement si la suppression de l'original a réussi et si aucune retenue légale
active ne concerne le job. La purge conserve une trace agrégée, sans image ni
texte OCR. Les sources des couvertures attachées aux livres suivent le cycle de
vie de `book_covers`, distinct de la source de l'import.

## Retenue légale

Seul `SUPER_ADMIN_ROOT` peut poser, prolonger ou lever une retenue sur un job :

- `POST /api/admin/cover-imports/{id}/legal-hold` avec un objet JSON
  `{"reason":"Litige en cours","expiresAt":"2027-01-01T00:00:00Z"}`.
- `DELETE /api/admin/cover-imports/{id}/legal-hold` pour la lever.

La date doit être future. Une pose après la prise en charge ou la suppression
de l'objet retourne `409`. Chaque changement est audité avec l'identité root.
Une retenue active bloque également une tâche déjà placée dans la file.
L'accès des propriétaires aux revues SAFE et la décision root sur la
quarantaine restent définis par les routes existantes.

## Gate

Dans un environnement où Go, Node.js, Chromium et Docker Compose sont
installés :

```sh
sh scripts/check-release-v1.7.0.sh
```

Le gate inclut les tests Go et de course, les tests frontend et navigateur,
le build React/Vite, le contrat OpenAPI et la validation de la configuration
Docker. Le workflow `Release v1.7.0 gate` lance ensuite MinIO et JetStream
avec des identifiants temporaires et exécute les tests d'intégration existants
contre ces services. Une CI verte est nécessaire avant la fusion dans `develop`.
