# Préparation de la version 1.4.0

Cette procédure prépare la release à partir de `develop`. Le tag et la release
GitHub ne sont créés qu’après validation du commit candidat exact.

## 1. Vérifier le candidat

```sh
git switch develop
git pull --ff-only
git status --short
./scripts/check-delivery.sh
git rev-parse --verify HEAD
```

Le statut doit être vide. Conserver le SHA validé ; refaire la recette si le
commit change. Le contrôle inclut la restauration SQLite, les tests Go avec
détecteur de courses, les tests frontend et la suite Playwright complète.
Le pipeline MinIO → outbox → JetStream → worker peut être vérifié séparément
avec `COVER_PIPELINE_INTEGRATION=1` sur l’infrastructure isolée documentée dans
`BOOK_COVERS_V1_4.md`. Ce test n’est pas requis lorsque cette infrastructure
n’est pas disponible.

## 2. Préparer l’exploitation

Avant toute migration d’une base existante, créer et vérifier une sauvegarde
SQLite selon `SQLITE_RESTORE.md`. Prévoir la persistance de la base, du bucket
MinIO et de JetStream, leurs sauvegardes et les droits du worker sur le même
fichier SQLite. Le bucket reste privé ; le navigateur lit les images par l’API.

Configurer `COVERS_ENABLED=true` seulement après avoir renseigné les variables
`MINIO_*` et `NATS_*` de `.env.example`. La clé JWT et les mots de passe
proviennent du gestionnaire de secrets. Si l’API et Compose ne partagent pas le
même réseau, définir des endpoints accessibles depuis chaque processus.
L’API démarre malgré l’arrêt de NATS ; les événements en attente restent dans
l’outbox. La perte de MinIO empêche un nouvel upload.

Sur Linux AMD64, démarrer le worker séparé avec le profil Compose `worker`.
Sur Windows AMD64, le binaire applicatif supervise le worker lorsque les
couvertures sont activées. Éviter deux workers sur la même identité de
consommateur durable sans plan de capacité et de reprise.

## 3. Construire le candidat

```sh
mkdir -p dist
CGO_ENABLED=1 go build -trimpath -tags fts5 \
  -o dist/defta-librairie-v1.4.0-linux-amd64 ./cmd
sha256sum dist/defta-librairie-v1.4.0-linux-amd64 \
  > dist/defta-librairie-v1.4.0-linux-amd64.sha256
```

Au lancement, définir `VERSION=1.4.0` et `BUILD_DATE` à la date réelle du
build. Ne jamais incorporer `.env`, une base SQLite, une sauvegarde ou les
objets MinIO dans l’archive.

## 4. Taguer et publier après validation

Vérifier que `HEAD` est toujours le SHA validé, puis :

```sh
git tag -a v1.4.0 -m "Defta Librairie 1.4.0"
git show --no-patch --decorate v1.4.0
git push origin v1.4.0
```

Le tag déclenche la construction et publication de l’image worker Linux AMD64
sur GHCR. Déclencher ensuite le workflow des exécutables depuis `develop` :

```sh
gh workflow run release-binaries.yml --ref develop -f tag=v1.4.0
gh run list --workflow release-binaries.yml --limit 3
```

Vérifier le succès des deux workflows, la provenance et le SBOM du worker,
la présence des archives et les sommes de contrôle selon
`RELEASE_ARTIFACTS.md`. Publier les notes depuis `CHANGELOG.md`.

## 5. Vérifier et revenir en arrière

Après déploiement, vérifier les sondes et une connexion, puis un upload JPEG/PNG
réel : `PENDING` → `READY`, lecture de la miniature, remplacement, relance
d’un échec autorisé et nettoyage différé. Observer la reprise après arrêt de
NATS et les journaux du worker avant d’ouvrir l’upload aux utilisateurs.

En cas de retour arrière, arrêter les nouveaux uploads et le worker, conserver
la base et le bucket pour diagnostic et restaurer ensemble un binaire et une
sauvegarde SQLite compatibles, en suivant `SQLITE_RESTORE.md`. Documenter le
SHA, l’heure, la sauvegarde et les objets MinIO concernés. Ne jamais déplacer
un tag publié.
