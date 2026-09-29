# Recette et rétention v1.7.0

La v1.7.0 ajoute la rétention des imports de couvertures. Le worker de nettoyage réconcilie les échéances toutes
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
contre ces services. Une CI verte sur le commit candidat de `develop` est nécessaire avant le tag.

## Publication du candidat

Après fusion de la branche de clôture, noter le SHA exact de `develop` et vérifier
que la CI `Release v1.7.0 gate` est verte pour ce SHA. Faire une sauvegarde SQLite
et vérifier sa restauration sur une copie avant migration 032/033. Préparer la
persistance de SQLite, MinIO et JetStream, ainsi que Tesseract 5 et la langue
arabe dans le worker. Conserver les droits du worker sur SQLite et le bucket privé.
Si le backend tourne sur l'hôte et le worker sous Docker avec `../data:/app/data`,
définir `COVER_WORKER_UID` et `COVER_WORKER_GID` dans `.env` avec les résultats
de `id -u` et `id -g` du compte propriétaire de `data/defta.db`. Le dossier
`data/backups` doit être accessible en écriture à cette identité ; la sauvegarde
SQLite est obligatoire avant les migrations. Ne pas modifier les droits de la
base pour tous les utilisateurs.

Une fois le SHA et les résultats validés, créer le tag annoté sur ce SHA puis
déclencher le workflow des exécutables :

```sh
git switch develop
git pull --ff-only origin develop
git rev-parse HEAD
git tag -a v1.7.0 -m "Defta Librairie 1.7.0"
git push origin v1.7.0
gh workflow run release-binaries.yml --ref develop -f tag=v1.7.0
```

Vérifier les archives Windows/Linux AMD64, `SHA256SUMS`, `BUILD-INFO.txt`,
l'image du worker, le SBOM et la provenance selon `docs/RELEASE_ARTIFACTS.md`.
Contrôler un import SAFE, une revue, un rejet, un rattachement humain,
l'isolation inter-librairies et une reprise après incident. En cas de retour
arrière, arrêter les nouveaux imports et les workers, préserver base et objets,
puis restaurer une sauvegarde SQLite et des artefacts compatibles. Ne pas déplacer
un tag publié.

Les images MinIO publiques utilisées par Compose proviennent de Chainguard.
Les digests sont fixés dans le Compose ; `MINIO_SERVER_IMAGE` et
`MINIO_CLIENT_IMAGE` permettent une mise à jour contrôlée. Le gate d'intégration
utilise la configuration du déploiement et des volumes jetables.
